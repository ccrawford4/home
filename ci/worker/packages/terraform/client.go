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

func (tfClient *TerraformClient) Plan() {
	cmd := "terraform plan -out=tfplan"
	output, err := shell.ExecuteCommand(cmd)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write([]byte(output))
}
