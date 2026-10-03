package main

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"virgo-bench/api"
	"virgo-bench/internal/bench"
	"virgo-bench/internal/logger"
	"virgo-bench/internal/ollama"
	"virgo-bench/internal/store"
	"virgo-bench/web"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log := logger.New()
	slog.SetDefault(log)

	virgoURL := env("VIRGO_URL", "http://virgo.virgo.svc.cluster.local:8000")
	log.Info("starting virgo-bench", "virgo_url", virgoURL)

	var st store.Store
	if host := os.Getenv("MYSQL_HOST"); host != "" {
		db, err := store.OpenMySQL(ctx, store.MySQLConfig{
			Host:     host,
			Port:     env("MYSQL_PORT", "3306"),
			User:     os.Getenv("MYSQL_USERNAME"),
			Password: os.Getenv("MYSQL_PASSWORD"),
			Database: env("MYSQL_DATABASE", "virgobench"),
		}, log)
		if err != nil {
			log.Error("failed to open mysql", "error", err)
			os.Exit(1)
		}
		st = db
	} else {
		log.Warn("MYSQL_HOST not set; using in-memory store (results are lost on restart)")
		st = store.NewMemory()
	}
	defer st.Close()

	// Cold starts take ~10s and long prompts are slow on the NPU, so allow
	// plenty of time per request.
	client := ollama.New(virgoURL, 10*time.Minute)
	runner := bench.NewRunner(st, client, log)
	runner.Start(ctx)

	ui, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		log.Error("failed to load UI", "error", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	api.NewHandler(log, st, runner, client, ui).RegisterRoutes(mux)

	addr := ":" + env("PORT", "8080")
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}

	go func() {
		log.Info("listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server error", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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
