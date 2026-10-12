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
	"sandbox-api/gpupool"
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

	// KUBERNETES_SERVICE_HOST is set in every pod, so its presence means
	// we are running in the cluster.
	inCluster := os.Getenv("KUBERNETES_SERVICE_HOST") != ""

	mgr, err := sandbox.NewManager(&sandbox.ManagerConfig{
		Logger: log,
		KubeConfig: &k8s.KubeClientConfig{
			InCluster: inCluster,
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

	gpuPool, err := startGPUPool(ctx, log, inCluster)
	if err != nil {
		log.Error("failed to start gpu pool scaler", "error", err)
		os.Exit(1)
	}
	// A nil *Controller inside the interface would not compare equal to nil.
	var pool api.GPUPool
	if gpuPool != nil {
		pool = gpuPool
	}
	h.RegisterGPUPoolRoutes(mux, pool)

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

// startGPUPool starts the GPU pool scaler when GPU_POOL_ENABLED is set. It
// returns nil when the scaler is disabled.
func startGPUPool(ctx context.Context, log *slog.Logger, inCluster bool) (*gpupool.Controller, error) {
	cfg, err := gpupool.ConfigFromEnv()
	if err != nil {
		return nil, err
	}
	if !cfg.Enabled {
		log.Info("gpu pool scaler disabled")
		return nil, nil
	}

	clientset, err := k8s.NewClientset(inCluster)
	if err != nil {
		return nil, fmt.Errorf("creating kubernetes client: %w", err)
	}

	var mig gpupool.MIG
	if cfg.DryRun {
		mig = gpupool.NewDryRunMIG(log)
	} else if mig, err = gpupool.NewGCEMIG(ctx, cfg); err != nil {
		return nil, err
	}

	c := gpupool.NewController(cfg, mig, gpupool.NewKubeCluster(clientset, cfg, log), log)
	go c.Run(ctx)
	return c, nil
}
