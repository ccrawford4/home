package kubernetes

import (
	"fmt"
	"os"
	"strings"
)

const serviceAccountTokenFile = "/var/run/secrets/kubernetes.io/serviceaccount/token"

type BearerTokenSource struct {
	tokenFile string
}

func (s *BearerTokenSource) Token() (string, error) {
	if s.tokenFile == "" {
		return "", nil
	}

	token, err := os.ReadFile(s.tokenFile)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("failed to read Kubernetes token file %s: %w", s.tokenFile, err)
	}
	return strings.TrimSpace(string(token)), nil
}
