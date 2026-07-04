package sandboxapi

import "fmt"

// Lightweight sandbox API for executing untrusted code in secure environments.

// APIS:

// sandbox create - POST /sandbox/create

// sandbox get <sandbox-id> - GET /sandbox/<sandbox-id>

// sandbox delete <sandbox-id> - DELETE /sandbox/<sandbox-id>

// sandbox list - GET /sandbox/list

// Main packages:

// /cmd -> entrypoint
// /api -> API endpoints
// /sandbox -> sandbox management

// V0 -> Take an API call -> trigger a K8s Job -> return the result

func main() int {
	fmt.Printf("Hello, World!\n")
	return 0
}
