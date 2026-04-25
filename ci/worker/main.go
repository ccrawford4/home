package main

import (
	"ci-worker/packages/api"
	"log"
	"os"
)

func main() {
	addr := os.Getenv("PORT")
	if addr == "" {
		addr = "8080"
	}

	server := api.NewServer()
	log.Fatal(server.Start(":" + addr))
}
