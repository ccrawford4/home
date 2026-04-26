package openid

import (
	"encoding/json"
	"fmt"
)

type DiscoveryDocument struct {
	Issuer string
}

func ParseDiscoveryDocument(body []byte) (*DiscoveryDocument, error) {
	var values map[string]any
	if err := json.Unmarshal(body, &values); err != nil {
		return nil, fmt.Errorf("failed to decode discovery document: %w", err)
	}

	issuer, _ := values["issuer"].(string)
	return &DiscoveryDocument{
		Issuer: issuer,
	}, nil
}
