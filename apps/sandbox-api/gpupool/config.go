// Package gpupool scales the GCP managed instance group that backs the GPU
// nodes of the cluster. It raises the group's target size when a pod that
// requests a GPU is pending or when a client reports demand, and lowers it
// back to zero once the GPU nodes have been idle for a while.
package gpupool

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// GPUResource is the extended resource name GPU workloads request.
const GPUResource = "nvidia.com/gpu"

// Config controls the GPU pool scaler.
type Config struct {
	// Enabled turns the scaler on. When false, no controller runs and the
	// demand endpoint answers 503.
	Enabled bool
	// DryRun logs every resize, cordon, eviction and node deletion instead of
	// doing it, and tracks the group's size in memory instead of calling GCE.
	DryRun bool

	Project string
	Zone    string
	MIG     string

	// NodeLabelKey and NodeLabelValue select the GPU nodes.
	NodeLabelKey   string
	NodeLabelValue string

	MaxSize      int
	IdleTimeout  time.Duration
	SyncInterval time.Duration
}

// ConfigFromEnv reads the scaler's configuration from GPU_POOL_* variables.
func ConfigFromEnv() (Config, error) {
	cfg := Config{
		Project:        os.Getenv("GPU_POOL_PROJECT"),
		Zone:           os.Getenv("GPU_POOL_ZONE"),
		MIG:            os.Getenv("GPU_POOL_MIG"),
		NodeLabelKey:   "accelerator",
		NodeLabelValue: "nvidia-l4",
		MaxSize:        1,
		IdleTimeout:    10 * time.Minute,
		SyncInterval:   30 * time.Second,
	}

	var err error
	if cfg.Enabled, err = envBool("GPU_POOL_ENABLED", false); err != nil {
		return cfg, err
	}
	if cfg.DryRun, err = envBool("GPU_POOL_DRY_RUN", false); err != nil {
		return cfg, err
	}
	if v := os.Getenv("GPU_POOL_NODE_SELECTOR"); v != "" {
		key, value, ok := strings.Cut(v, "=")
		if !ok || key == "" {
			return cfg, fmt.Errorf("GPU_POOL_NODE_SELECTOR must be key=value, got %q", v)
		}
		cfg.NodeLabelKey, cfg.NodeLabelValue = key, value
	}
	if v := os.Getenv("GPU_POOL_MAX_SIZE"); v != "" {
		if cfg.MaxSize, err = strconv.Atoi(v); err != nil {
			return cfg, fmt.Errorf("GPU_POOL_MAX_SIZE: %w", err)
		}
	}
	if cfg.IdleTimeout, err = envDuration("GPU_POOL_IDLE_TIMEOUT", cfg.IdleTimeout); err != nil {
		return cfg, err
	}
	if cfg.SyncInterval, err = envDuration("GPU_POOL_SYNC_INTERVAL", cfg.SyncInterval); err != nil {
		return cfg, err
	}

	if cfg.Enabled {
		return cfg, cfg.Validate()
	}
	return cfg, nil
}

// Validate reports a missing or out-of-range setting.
func (c Config) Validate() error {
	if !c.DryRun && (c.Project == "" || c.Zone == "" || c.MIG == "") {
		return fmt.Errorf("GPU_POOL_PROJECT, GPU_POOL_ZONE and GPU_POOL_MIG are required")
	}
	if c.MaxSize < 1 {
		return fmt.Errorf("GPU_POOL_MAX_SIZE must be at least 1, got %d", c.MaxSize)
	}
	if c.IdleTimeout <= 0 || c.SyncInterval <= 0 {
		return fmt.Errorf("GPU_POOL_IDLE_TIMEOUT and GPU_POOL_SYNC_INTERVAL must be positive")
	}
	return nil
}

func envBool(key string, def bool) (bool, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def, fmt.Errorf("%s: %w", key, err)
	}
	return b, nil
}

func envDuration(key string, def time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def, fmt.Errorf("%s: %w", key, err)
	}
	return d, nil
}
