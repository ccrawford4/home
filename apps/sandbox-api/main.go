package main

import (
	"log"
	"net/http"

	"sandbox-api/internal/api"
)

func main() {
	srv := api.NewServer()
	log.Println("sandbox API listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", srv.Routes()))
}
