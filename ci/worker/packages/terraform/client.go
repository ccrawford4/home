package terraform

import (
	"net/http"

	"ci-worker/packages/shell"
)

type TerraformClient struct {
}

type TerraformRoute string

const (
	PlanRoute TerraformRoute = "/terraform/plan"
)

func (tfClient *TerraformClient) Plan(w http.ResponseWriter, r *http.Request) {
	output, err := shell.ExecuteCommand("terraform", "plan", "-out=tfplan")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write([]byte(output))
}
