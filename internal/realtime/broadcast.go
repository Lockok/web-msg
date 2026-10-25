package realtime

import (
	"context"
	"sync"

	"github.com/Lockok/web-msg/internal/domain"
	"github.com/coder/websocket"
	"github.com/google/uuid"
)

const broadcastWorkers = 4

type BroadcastReport struct {
	Connections int
	Written     int
	Failed      int
}

func (h *Hub) BroadcastMessageCreated(ctx context.Context, userIDs []uuid.UUID, message domain.Message) BroadcastReport {
	targets := make(map[*websocket.Conn]struct{})

	for _, userID := range userIDs {
		for _, conn := range h.Connections(userID) {
			targets[conn] = struct{}{}
		}
	}

	report := BroadcastReport{
		Connections: len(targets),
	}

	if len(targets) == 0 {
		return report
	}

	jobs := make(chan *websocket.Conn, len(targets))

	for conn := range targets {
		jobs <- conn
	}
	close(jobs)

	workers := broadcastWorkers
	if len(targets) < workers {
		workers = len(targets)
	}

	var wg sync.WaitGroup
	var reportMu sync.Mutex

	for i := 0; i < workers; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for conn := range jobs {
				err := WriteMessageCreated(ctx, conn, message)

				reportMu.Lock()
				if err != nil {
					report.Failed++
				} else {
					report.Written++
				}
				reportMu.Unlock()
			}
		}()
	}

	wg.Wait()

	return report

}
