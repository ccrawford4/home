// Package main is the entrypoint for the sandbox CLI.
//
// This package is responsible for the CLI for the sandbox. It is a simple CLI
// which allows you to connect to the API.
//
// Package structure:
//   - /cmd -> CLI commands (uses Cobra)
//   - /api -> API client
//
// Example commands:
//
//	sandbox create
//	sandbox get <sandbox-id>
//	sandbox delete <sandbox-id>
//	sandbox list
package main

import (
	"fmt"
	"os"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	fmt.Println("sandbox-cli")
	return nil
}
