package worker

import (
	"context"
	"log"
	"time"
)

type Worker struct {
	Label    string
	Action   func()        // func to get work done
	Interval time.Duration // time interval to run the task
	Period   time.Duration // actual waiting time after a task
	Stopped  bool          // state of the worker
	ctx      context.Context
	cancel   context.CancelFunc
}

func NewWorker(label string, interval time.Duration, action func()) *Worker {
	ctx, cancel := context.WithCancel(context.Background())
	w := &Worker{
		Label:    label,
		Interval: interval,
		Action:   action,
		ctx:      ctx,
		cancel:   cancel,
	}

	return w
}

func (w *Worker) Run() {
	log.Printf("Starting worker with label %s", w.Label)
	ticker := time.NewTicker(w.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			log.Printf("Worker shutting down %s", w.Label)
			return

		case <-ticker.C:
			go func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("Worker %s panicked: %v", w.Label, r)
					}
				}()

				w.Action()
				log.Printf("%s: worker finished", w.Label)
			}()
		}
	}
}

func (w *Worker) Shutdown() {
	w.cancel()
}
