package main

import (
	"fmt"
	"log"
	"net/http"
	"webhook-tester/cmd/server"
)

func main() {
	srv := server.NewServer()
	srv.MountHandlers()

	fmt.Println("Server running on http://localhost:3000")
	err := http.ListenAndServe(":3000", srv.Router)
	if err != nil {
		log.Fatal("Failed to start server", err)
	}
}
