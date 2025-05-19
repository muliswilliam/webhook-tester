package main

import (
	"context"
	"github.com/robfig/cron"
	"gorm.io/gorm"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
	"webhook-tester/cmd/server"
	"webhook-tester/internal/metrics"
)

func scheduleCleanup(db *gorm.DB, c *cron.Cron) {
	// clean every day
	err := c.AddFunc("0 0 * * *", func() {
		//sqlstore.CleanPublicWebhooks(db, 48*time.Hour) // 48 hours old
	})
	if err != nil {
		log.Fatalf("error scheduling cleanup: %s", err)
	}
}

// @title Webhook Tester API
// @version 1.0
// @description REST API to interact with webhooks and webhook requests

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
	metrics.Register()

	go func() {
		err := s.Srv.ListenAndServe()
		if err != nil {
			s.Logger.Fatal(err)
		}
	}()

	s.Logger.Printf("server listening on port 3000")

	// cron setup
	c := cron.New()
	scheduleCleanup(s.DB, c)
	c.Start()
	defer c.Stop()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	signal.Notify(quit, syscall.SIGTERM)

	shut := <-quit
	s.Logger.Printf("shutting down by signal: %s", shut.String())
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := s.Srv.Shutdown(ctx); err != nil {
		s.Logger.Printf("graceful shutdown failed: %s", err)
	}

	s.Logger.Printf("server stopped")
}
