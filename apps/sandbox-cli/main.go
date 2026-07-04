package sandboxcli

import "fmt"

// This package is responsible for the CLI for the sandbox. It is a simple CLI which allows you to connect to the API. Should be split up into these packages:

// cmd/ - the actual commands for the CLI
// api/ - the API client for the CLI

// example commands:

// sandbox create - POST /sandbox/create

// sandbox get <sandbox-id> - GET /sandbox/<sandbox-id>

// sandbox delete <sandbox-id> - DELETE /sandbox/<sandbox-id>

// sandbox list - GET /sandbox/list

// Uses state of the art go CLI libraries, like Cobra, Viper, and Bubble Tea/Lip Gloss
func main() int {
	fmt.Printf("Hello, World!\n")

	return 0
}
