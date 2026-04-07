package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/hilman2/ELNSSM/internal/model"
	bolt "go.etcd.io/bbolt"
)

var (
	bucketServices     = []byte("services")
	bucketServiceState = []byte("service_state")
	bucketEvents       = []byte("events")
	bucketHealth       = []byte("health_history")
	bucketResources    = []byte("resource_samples")
	bucketMeta         = []byte("meta")
)

// BoltStore implements Store using bbolt as the backend.
type BoltStore struct {
	db *bolt.DB
}

// NewBoltStore opens or creates a bbolt database at the given path.
func NewBoltStore(path string) (*BoltStore, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("creating database directory: %w", err)
	}

	db, err := bolt.Open(path, 0600, &bolt.Options{Timeout: 5 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// Create all buckets
	err = db.Update(func(tx *bolt.Tx) error {
		for _, bucket := range [][]byte{bucketServices, bucketServiceState, bucketEvents, bucketHealth, bucketResources, bucketMeta} {
			if _, err := tx.CreateBucketIfNotExists(bucket); err != nil {
				return fmt.Errorf("creating bucket %s: %w", bucket, err)
			}
		}
		return nil
	})
	if err != nil {
		db.Close()
		return nil, err
	}

	return &BoltStore{db: db}, nil
}

func (s *BoltStore) ListServices(_ context.Context) ([]*model.Service, error) {
	var services []*model.Service
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketServices)
		return b.ForEach(func(k, v []byte) error {
			var svc model.Service
			if err := json.Unmarshal(v, &svc); err != nil {
				return fmt.Errorf("unmarshaling service %s: %w", k, err)
			}
			services = append(services, &svc)
			return nil
		})
	})
	return services, err
}

func (s *BoltStore) GetService(_ context.Context, id string) (*model.Service, error) {
	var svc model.Service
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketServices)
		v := b.Get([]byte(id))
		if v == nil {
			return fmt.Errorf("service %q not found", id)
		}
		return json.Unmarshal(v, &svc)
	})
	if err != nil {
		return nil, err
	}
	return &svc, nil
}

func (s *BoltStore) SaveService(_ context.Context, svc *model.Service) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketServices)
		data, err := json.Marshal(svc)
		if err != nil {
			return fmt.Errorf("marshaling service: %w", err)
		}
		return b.Put([]byte(svc.ID), data)
	})
}

func (s *BoltStore) DeleteService(_ context.Context, id string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketServices)
		return b.Delete([]byte(id))
	})
}

func (s *BoltStore) GetServiceState(_ context.Context, id string) (*ServiceRuntimeState, error) {
	var state ServiceRuntimeState
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketServiceState)
		v := b.Get([]byte(id))
		if v == nil {
			return nil // no state yet, return zero value
		}
		return json.Unmarshal(v, &state)
	})
	if err != nil {
		return nil, err
	}
	return &state, nil
}

func (s *BoltStore) SaveServiceState(_ context.Context, id string, state *ServiceRuntimeState) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketServiceState)
		data, err := json.Marshal(state)
		if err != nil {
			return fmt.Errorf("marshaling service state: %w", err)
		}
		return b.Put([]byte(id), data)
	})
}

func (s *BoltStore) AppendEvent(_ context.Context, event model.Event) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketEvents)
		// Key format: timestamp-serviceID-eventID for chronological ordering
		key := fmt.Sprintf("%s-%s-%s", event.Timestamp.Format(time.RFC3339Nano), event.ServiceID, event.ID)
		data, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("marshaling event: %w", err)
		}
		return b.Put([]byte(key), data)
	})
}

func (s *BoltStore) ListEvents(_ context.Context, filter EventFilter) ([]model.Event, error) {
	var events []model.Event
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketEvents)
		c := b.Cursor()

		// Iterate in reverse order (newest first)
		for k, v := c.Last(); k != nil; k, v = c.Prev() {
			var event model.Event
			if err := json.Unmarshal(v, &event); err != nil {
				continue
			}

			// Apply filters
			if filter.ServiceID != nil && event.ServiceID != *filter.ServiceID {
				continue
			}
			if filter.Type != nil && event.Type != *filter.Type {
				continue
			}
			if filter.Since != nil && event.Timestamp.Before(*filter.Since) {
				break // events are chronologically ordered, we can stop
			}

			events = append(events, event)
			if filter.Limit > 0 && len(events) >= filter.Limit {
				break
			}
		}
		return nil
	})
	return events, err
}

func (s *BoltStore) AppendHealthResult(_ context.Context, serviceID string, result model.HealthCheckResult) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketHealth)

		// Use a sub-bucket per service for efficient per-service queries
		sb, err := b.CreateBucketIfNotExists([]byte(serviceID))
		if err != nil {
			return fmt.Errorf("creating health sub-bucket: %w", err)
		}

		key := result.Timestamp.Format(time.RFC3339Nano)
		data, err := json.Marshal(result)
		if err != nil {
			return fmt.Errorf("marshaling health result: %w", err)
		}

		if err := sb.Put([]byte(key), data); err != nil {
			return err
		}

		// Prune old entries (keep last 1000)
		return pruneOldEntries(sb, 1000)
	})
}

func (s *BoltStore) GetHealthHistory(_ context.Context, serviceID string, limit int) ([]model.HealthCheckResult, error) {
	var results []model.HealthCheckResult
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketHealth)
		sb := b.Bucket([]byte(serviceID))
		if sb == nil {
			return nil
		}

		c := sb.Cursor()
		for k, v := c.Last(); k != nil; k, v = c.Prev() {
			var result model.HealthCheckResult
			if err := json.Unmarshal(v, &result); err != nil {
				continue
			}
			results = append(results, result)
			if limit > 0 && len(results) >= limit {
				break
			}
		}
		return nil
	})

	// Reverse to get chronological order
	sort.Slice(results, func(i, j int) bool {
		return results[i].Timestamp.Before(results[j].Timestamp)
	})

	return results, err
}

func (s *BoltStore) AppendResourceSample(_ context.Context, serviceID string, sample model.ResourceSample) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketResources)
		sb, err := b.CreateBucketIfNotExists([]byte(serviceID))
		if err != nil {
			return fmt.Errorf("creating resource sub-bucket: %w", err)
		}

		key := sample.Timestamp.Format(time.RFC3339Nano)
		data, err := json.Marshal(sample)
		if err != nil {
			return fmt.Errorf("marshaling resource sample: %w", err)
		}

		if err := sb.Put([]byte(key), data); err != nil {
			return err
		}

		// Keep at most 100k samples per service (~35 days at 30s intervals)
		return pruneOldEntries(sb, 100000)
	})
}

func (s *BoltStore) GetResourceHistory(_ context.Context, serviceID string, from, to time.Time, maxPoints int) ([]model.ResourceSample, error) {
	var samples []model.ResourceSample
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketResources)
		sb := b.Bucket([]byte(serviceID))
		if sb == nil {
			return nil
		}

		fromKey := []byte(from.Format(time.RFC3339Nano))
		toKey := []byte(to.Format(time.RFC3339Nano))
		c := sb.Cursor()

		// Collect all samples in range
		for k, v := c.Seek(fromKey); k != nil; k, v = c.Next() {
			if string(k) > string(toKey) {
				break
			}
			var sample model.ResourceSample
			if err := json.Unmarshal(v, &sample); err != nil {
				continue
			}
			samples = append(samples, sample)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Downsample if too many points
	if maxPoints > 0 && len(samples) > maxPoints {
		samples = downsample(samples, maxPoints)
	}

	return samples, nil
}

func (s *BoltStore) PruneResourceHistory(_ context.Context, olderThan time.Time) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketResources)
		return b.ForEach(func(k, v []byte) error {
			if v != nil {
				return nil // not a sub-bucket
			}
			sb := b.Bucket(k)
			if sb == nil {
				return nil
			}

			cutoff := []byte(olderThan.Format(time.RFC3339Nano))
			c := sb.Cursor()
			for ck, _ := c.First(); ck != nil; ck, _ = c.Next() {
				if string(ck) >= string(cutoff) {
					break
				}
				if err := sb.Delete(ck); err != nil {
					return err
				}
			}
			return nil
		})
	})
}

// downsample reduces the number of samples by averaging groups.
func downsample(samples []model.ResourceSample, maxPoints int) []model.ResourceSample {
	if len(samples) <= maxPoints {
		return samples
	}

	step := float64(len(samples)) / float64(maxPoints)
	result := make([]model.ResourceSample, 0, maxPoints)

	for i := 0; i < maxPoints; i++ {
		start := int(float64(i) * step)
		end := int(float64(i+1) * step)
		if end > len(samples) {
			end = len(samples)
		}

		var cpuSum float64
		var memSum int64
		count := 0
		for j := start; j < end; j++ {
			cpuSum += samples[j].CPUPercent
			memSum += samples[j].MemoryBytes
			count++
		}

		if count > 0 {
			result = append(result, model.ResourceSample{
				Timestamp:   samples[(start+end-1)/2].Timestamp,
				CPUPercent:  cpuSum / float64(count),
				MemoryBytes: memSum / int64(count),
			})
		}
	}

	return result
}

func (s *BoltStore) Close() error {
	return s.db.Close()
}

// pruneOldEntries removes the oldest entries from a bucket, keeping only the most recent 'keep' entries.
func pruneOldEntries(b *bolt.Bucket, keep int) error {
	count := 0
	c := b.Cursor()
	for k, _ := c.Last(); k != nil; k, _ = c.Prev() {
		count++
	}

	if count <= keep {
		return nil
	}

	toDelete := count - keep
	deleted := 0
	for k, _ := c.First(); k != nil && deleted < toDelete; k, _ = c.Next() {
		if err := b.Delete(k); err != nil {
			return err
		}
		deleted++
	}
	return nil
}
