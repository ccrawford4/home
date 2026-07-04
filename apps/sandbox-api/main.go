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
"log"
"net/http"

"sandbox-api/api"
"sandbox-api/sandbox"
)

func main() {
mgr := sandbox.NewManager()
h := api.NewHandler(mgr)

mux := http.NewServeMux()
mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
fmt.Fprintln(w, "ok")
})
h.RegisterRoutes(mux)

log.Println("sandbox-api listening on :8080")
if err := http.ListenAndServe(":8080", mux); err != nil {
log.Fatal(err)
}
}
