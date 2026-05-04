package config

import (
	"errors"
	"os"
	"strings"
)

const (
	defaultPort = "8080"
)

type Config struct {
	Port            string
	APIServerURL    string
	PublicIssuerURL string
}

func Load() (Config, error) {
	apiServerURL, err := kubernetesAPIServerURL()
	if err != nil {
		return Config{}, err
	}

	return Config{
		Port:            env("PORT", defaultPort),
		APIServerURL:    apiServerURL,
		PublicIssuerURL: strings.TrimRight(os.Getenv("PUBLIC_ISSUER_URL"), "/"),
	}, nil
}

func kubernetesAPIServerURL() (string, error) {
	if apiServerURL := strings.TrimRight(os.Getenv("KUBERNETES_API_URL"), "/"); apiServerURL != "" {
		return apiServerURL, nil
	}
	return "", errors.New("KUBERNETES_API_URL environment variable is required")
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
