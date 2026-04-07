package guardian

import (
	"context"
	"log/slog"

	"github.com/hilman2/ELNSSM/internal/config"
	"golang.org/x/sys/windows/svc"
)

const serviceName = "ELNSSM"

// handler implements the Windows service handler interface.
type handler struct {
	cfg *config.Config
}

// Execute is called by the Windows Service Control Manager.
func (h *handler) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown

	changes <- svc.Status{State: svc.StartPending}

	g, err := New(h.cfg)
	if err != nil {
		slog.Error("Failed to create Guardian", "error", err)
		changes <- svc.Status{State: svc.StopPending}
		return true, 1
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Run Guardian in background
	errCh := make(chan error, 1)
	go func() {
		errCh <- g.Run(ctx)
	}()

	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
	slog.Info("ELNSSM Guardian service started")

	for {
		select {
		case cr := <-r:
			switch cr.Cmd {
			case svc.Interrogate:
				changes <- cr.CurrentStatus
			case svc.Stop, svc.Shutdown:
				slog.Info("Received stop command from SCM")
				changes <- svc.Status{State: svc.StopPending}
				cancel()
				// Wait for Guardian to finish
				<-errCh
				changes <- svc.Status{State: svc.Stopped}
				return false, 0
			}
		case err := <-errCh:
			cancel()
			if err != nil {
				slog.Error("Guardian exited with error", "error", err)
				return true, 1
			}
			return false, 0
		}
	}
}

// RunAsService starts the Guardian as a Windows service.
func RunAsService(cfg *config.Config) error {
	return svc.Run(serviceName, &handler{cfg: cfg})
}
