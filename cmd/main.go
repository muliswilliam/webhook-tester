package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"
	"webhook-tester/cmd/server"
)

func main() {
	s := server.NewServer()
	s.MountHandlers()

	go func() {
		err := s.Srv.ListenAndServe()
		if err != nil {
			s.Logger.Fatal(err)
		}
	}()

	s.Logger.Printf("server listening on port 3000")

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
