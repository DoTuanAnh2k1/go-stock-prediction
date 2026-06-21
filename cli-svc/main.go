// Command cli-svc is an interactive SSH server: users SSH in with their
// dashboard credentials and get a shell exposing four verbs (get/set/update/
// delete) over the platform API, with per-command RBAC and table output.
package main

import (
	"log"

	"go-stock-prediction/cli-svc/internal/server"
)

func main() {
	cfg := server.LoadConfig()
	srv := server.New(cfg)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("cli-svc server error: %v", err)
	}
}
