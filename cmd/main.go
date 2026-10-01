package main

import (
	"context"
	"errors"
	"github.com/robfig/cron"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"webhook-tester/cmd/server"
	"webhook-tester/internal/service"
)

// guestWorkspaceTTL is how long a guest's public webhook lives, matching the
// "expires in two days" promise in the UI and the guest cookie's lifetime.
const guestWorkspaceTTL = 48 * time.Hour

func scheduleCleanup(webhookSvc *service.WebhookService, logger *log.Logger, c *cron.Cron) {
	err := c.AddFunc("@hourly", func() {
		if err := webhookSvc.CleanPublicWebhooks(guestWorkspaceTTL); err != nil {
			logger.Printf("error cleaning expired guest webhooks: %s", err)
		}
	})
	if err != nil {
		log.Fatalf("error scheduling cleanup: %s", err)
	}
}

// @title Webhook Tester API
// @version 1.0
// @description Manage your Webhook Tester endpoints programmatically. Authenticate every request with the X-API-Key header; your key is under API access in your [workspace](/).

// @contact.name William Muli
// @contact.url
// @contact.email william@srninety.one
// @BasePath    /api
// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name X-API-Key
func main() {
	s := server.NewServer()
	s.MountHandlers()

	for _, srv := range []*http.Server{s.Srv, s.MetricsSrv} {
		go func() {
			if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				s.Logger.Fatal(err)
			}
		}()
	}

	s.Logger.Printf("server listening on %s, metrics on %s", s.Srv.Addr, s.MetricsSrv.Addr)

	// cron setup
	c := cron.New()
	scheduleCleanup(s.WebhookSvc, s.Logger, c)
	c.Start()
	defer c.Stop()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	signal.Notify(quit, syscall.SIGTERM)

	shut := <-quit
	s.Logger.Printf("shutting down by signal: %s", shut.String())
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, srv := range []*http.Server{s.Srv, s.MetricsSrv} {
		if err := srv.Shutdown(ctx); err != nil {
			s.Logger.Printf("graceful shutdown of %s failed: %s", srv.Addr, err)
		}
	}
	// Let in-flight forwards record their deliveries. Captures still being
	// handled, if the server's shutdown timed out, record refused ones.
	if err := s.Forwarder.Shutdown(ctx); err != nil {
		s.Logger.Printf("in-flight forwards didn't finish: %s", err)
	}

	s.Logger.Printf("server stopped")
}
