package gpupool

import (
	"testing"
	"time"
)

func TestConfigFromEnvDefaults(t *testing.T) {
	t.Setenv("GPU_POOL_ENABLED", "true")
	t.Setenv("GPU_POOL_PROJECT", "home-473419")
	t.Setenv("GPU_POOL_ZONE", "us-central1-a")
	t.Setenv("GPU_POOL_MIG", "gpu-pool")

	cfg, err := ConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MaxSize != 1 || cfg.IdleTimeout != 10*time.Minute || cfg.DryRun {
		t.Fatalf("cfg = %+v, want max 1, idle 10m, not dry run", cfg)
	}
	if cfg.NodeLabelKey != "accelerator" || cfg.NodeLabelValue != "nvidia-l4" {
		t.Fatalf("node selector = %s=%s", cfg.NodeLabelKey, cfg.NodeLabelValue)
	}
}

func TestConfigFromEnvRequiresMIGUnlessDryRun(t *testing.T) {
	t.Setenv("GPU_POOL_ENABLED", "true")
	if _, err := ConfigFromEnv(); err == nil {
		t.Fatal("want an error without GPU_POOL_PROJECT/ZONE/MIG")
	}

	t.Setenv("GPU_POOL_DRY_RUN", "true")
	if _, err := ConfigFromEnv(); err != nil {
		t.Fatalf("dry run without a MIG: %v", err)
	}
}

func TestConfigFromEnvDisabledSkipsValidation(t *testing.T) {
	cfg, err := ConfigFromEnv()
	if err != nil || cfg.Enabled {
		t.Fatalf("cfg = %+v, err = %v, want disabled with no error", cfg, err)
	}
}
