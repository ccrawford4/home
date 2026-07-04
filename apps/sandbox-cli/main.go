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
//sandbox create
//sandbox get <sandbox-id>
//sandbox delete <sandbox-id>
//sandbox list
package main

import "sandbox-cli/cmd"

func main() {
cmd.Execute()
}
