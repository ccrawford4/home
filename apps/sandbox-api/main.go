package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"sandbox-api/api"
	"sandbox-api/internal/logger"
	"sandbox-api/internal/tracing"
	"sandbox-api/k8s"
	"sandbox-api/sandbox"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log := logger.New()
	slog.SetDefault(log)

	log.Info("starting sandbox-api")

	provider, err := tracing.InitTracer(ctx, "sandbox-api")
	if err != nil {
		log.Error("failed to initialize tracer", "error", err)
		os.Exit(1)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := provider.Shutdown(shutdownCtx); err != nil {
			log.Error("failed to shutdown tracer", "error", err)
		}
	}()

	mgr, err := sandbox.NewManager(&sandbox.ManagerConfig{
		Logger: log,
		KubeConfig: &k8s.KubeClientConfig{
			InCluster: false,
		},
	})

	if err != nil {
		log.Error("failed to create sandbox manager", "error", err)
	}

	h := api.NewHandler(log, mgr)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok")
	})
	h.RegisterRoutes(mux)

	srv := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	log.Info("sandbox-api listening", "addr", srv.Addr)

	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		log.Info("shutting down sandbox-api")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Error("server shutdown failed", "error", err)
			os.Exit(1)
		}
	case err := <-errCh:
		log.Error("server error", "error", err)
		os.Exit(1)
	}
}
