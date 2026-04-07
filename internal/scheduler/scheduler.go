package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/hilman2/ELNSSM/internal/model"
	"github.com/robfig/cron/v3"
)

// ServiceController is the interface the scheduler uses to control services.
// This avoids a circular dependency with the manager package.
type ServiceController interface {
	Start(ctx context.Context, id string) error
	Stop(ctx context.Context, id string) error
	Restart(ctx context.Context, id string) error
}

// Scheduler manages cron-based service schedules.
type Scheduler struct {
	cron       *cron.Cron
	controller ServiceController
	ctx        context.Context
	entryIDs   map[string][]cron.EntryID // serviceID -> list of cron entry IDs
	mu         sync.RWMutex
}

// New creates a new scheduler.
func New(controller ServiceController, ctx context.Context) *Scheduler {
	return &Scheduler{
		cron:       cron.New(cron.WithParser(cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor))),
		controller: controller,
		ctx:        ctx,
		entryIDs:   make(map[string][]cron.EntryID),
	}
}

// LoadAll registers cron jobs for all services.
func (s *Scheduler) LoadAll(services []*model.Service) {
	for _, svc := range services {
		s.updateServiceJobs(svc)
	}
}

// UpdateService re-registers cron jobs for a single service.
func (s *Scheduler) UpdateService(svc *model.Service) {
	s.RemoveService(svc.ID)
	s.updateServiceJobs(svc)
}

// RemoveService removes all cron jobs for a service.
func (s *Scheduler) RemoveService(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entries, ok := s.entryIDs[id]; ok {
		for _, entryID := range entries {
			s.cron.Remove(entryID)
		}
		delete(s.entryIDs, id)
	}
}

// Start begins executing scheduled jobs.
func (s *Scheduler) Start() {
	s.cron.Start()
	slog.Info("Scheduler started")
}

// Stop gracefully stops the scheduler.
func (s *Scheduler) Stop() {
	stopCtx := s.cron.Stop()
	<-stopCtx.Done()
	slog.Info("Scheduler stopped")
}

// ListEntries returns info about all scheduled entries.
func (s *Scheduler) ListEntries() []ScheduleInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []ScheduleInfo
	for serviceID, entryIDs := range s.entryIDs {
		for _, eid := range entryIDs {
			entry := s.cron.Entry(eid)
			result = append(result, ScheduleInfo{
				ServiceID: serviceID,
				Next:      entry.Next,
				Prev:      entry.Prev,
			})
		}
	}
	return result
}

// ScheduleInfo holds information about a scheduled entry.
type ScheduleInfo struct {
	ServiceID string    `json:"service_id"`
	Next      interface{} `json:"next"`
	Prev      interface{} `json:"prev"`
}

func (s *Scheduler) updateServiceJobs(svc *model.Service) {
	if len(svc.Schedules) == 0 {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, schedule := range svc.Schedules {
		serviceID := svc.ID
		action := schedule.Action
		name := schedule.Name

		entryID, err := s.cron.AddFunc(schedule.Cron, func() {
			slog.Info("Schedule triggered", "service", serviceID, "action", action, "name", name)
			var actionErr error
			switch action {
			case model.ScheduleStart:
				actionErr = s.controller.Start(s.ctx, serviceID)
			case model.ScheduleStop:
				actionErr = s.controller.Stop(s.ctx, serviceID)
			case model.ScheduleRestart:
				actionErr = s.controller.Restart(s.ctx, serviceID)
			default:
				slog.Error("Unknown schedule action", "action", action)
				return
			}
			if actionErr != nil {
				slog.Error("Schedule action failed", "service", serviceID, "action", action, "error", actionErr)
			}
		})

		if err != nil {
			slog.Error("Failed to register schedule", "service", svc.ID, "cron", schedule.Cron, "error", err)
			continue
		}

		s.entryIDs[svc.ID] = append(s.entryIDs[svc.ID], entryID)
		slog.Info("Schedule registered", "service", svc.ID, "cron", schedule.Cron, "action", schedule.Action,
			"name", fmt.Sprintf("%s", schedule.Name))
	}
}
