package api

import (
	"ci-worker/packages/terraform"
	"net/http"
)

func (s *Server) handleTerraformPlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tfClient := &terraform.TerraformClient{}
	tfClient.Plan(w, r)
}
