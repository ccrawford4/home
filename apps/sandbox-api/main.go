// Package main is the entrypoint for the sandbox API.
//
// Lightweight sandbox API for executing untrusted code in secure environments.
//
// APIs:
//   - POST   /sandbox/create         - create a new sandbox
//   - GET    /sandbox/:id            - get sandbox by ID
//   - DELETE /sandbox/:id            - delete sandbox by ID
//   - GET    /sandbox/list           - list all sandboxes
//
// Package structure:
//   - /api      -> HTTP handlers
//   - /sandbox  -> sandbox management
//
// V0: Take an API call -> trigger a Kubernetes Job -> return the result
package main

import (
	"fmt"
	log "log/slog"
	"net/http"

	"sandbox-api/api"
	"sandbox-api/k8s"
	"sandbox-api/sandbox"
)

func main() {
	mgr, err := sandbox.NewManager(&sandbox.ManagerConfig{
		KubeConfig: &k8s.KubeClientConfig{
			InCluster: false,
		},
	})
	if err != nil {
		log.Error("Failed to create sandbox manager", err)
	}

	h := api.NewHandler(mgr)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok")
	})
	h.RegisterRoutes(mux)

	log.Info("sandbox-api listening on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Error("{}", err)
	}
}
