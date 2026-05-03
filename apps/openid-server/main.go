package main

import (
	"log"
	"net/http"

	"openid-proxy/internal/config"
	"openid-proxy/internal/kubernetes"
	"openid-proxy/internal/openid"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	kubeClient, err := kubernetes.NewRawClient(kubernetes.RawClientOptions{
		APIServerURL: cfg.APIServerURL,
	})
	if err != nil {
		log.Fatal(err)
	}

	server := openid.NewServer(kubeClient, openid.ServerOptions{
		PublicIssuerURL: cfg.PublicIssuerURL,
		JWKSJSON:        []byte(cfg.JWKSJSON),
	})

	log.Printf("serving Kubernetes OpenID proxy on :%s", cfg.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, server.Routes()))
}
