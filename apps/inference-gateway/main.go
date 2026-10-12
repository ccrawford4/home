package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"inference-gateway/api"
	"inference-gateway/internal/auth"
	"inference-gateway/internal/config"
	"inference-gateway/internal/demand"
	"inference-gateway/internal/logger"
	"inference-gateway/internal/registry"
	"inference-gateway/internal/router"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log := logger.New()
	slog.SetDefault(log)

	cfgPath := env("CONFIG_PATH", "/etc/inference-gateway/config.yaml")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Error("failed to load config", "path", cfgPath, "error", err)
		os.Exit(1)
	}

	keys := auth.LoadDir(env("KEYS_DIR", "/etc/inference-gateway/keys"), cfg.Clients, log)
	if keys.Len() == 0 {
		log.Warn("no client keys loaded; every /api and /v1 request will be rejected")
	}
	adminKeyPath := env("ADMIN_KEY_FILE", "/etc/inference-gateway/admin/key")
	adminKey, err := auth.LoadAdminKey(adminKeyPath)
	if err != nil {
		log.Warn("admin key unavailable; /internal/backends is disabled", "path", adminKeyPath, "error", err)
	}

	reg := registry.New(cfg, log)
	rt := router.New(cfg, reg)

	// No overall timeout: responses stream for as long as generation runs,
	// and the client's context cancels the upstream request.
	upstream := &http.Client{Transport: &http.Transport{
		Proxy:                 nil,
		DialContext:           (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		MaxIdleConnsPerHost:   16,
		IdleConnTimeout:       90 * time.Second,
		ExpectContinueTimeout: time.Second,
	}}
	go reg.RunHealthChecks(ctx, upstream, 10*time.Second, 2*time.Second)

	catalog := make([]string, 0, len(cfg.Models))
	for _, m := range cfg.Models {
		catalog = append(catalog, m.Model)
	}

	dem := demand.New(cfg.DemandURL, cfg.DemandInterval, log)
	if dem == nil {
		log.Info("demandURL not set; GPU demand signalling disabled")
	}

	mux := http.NewServeMux()
	api.NewHandler(log, rt, reg, keys, adminKey, upstream, catalog, dem).RegisterRoutes(mux)

	srv := &http.Server{Addr: cfg.Listen, Handler: mux, ReadHeaderTimeout: 10 * time.Second}

	log.Info("starting inference-gateway",
		"config", cfgPath,
		"backends", len(cfg.Backends),
		"models", len(cfg.Models),
		"tiers", len(cfg.Tiers),
		"clients", keys.Len(),
	)
	go func() {
		log.Info("listening", "addr", cfg.Listen)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server error", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("shutdown error", "error", err)
	}
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
