package openid

import (
	"encoding/json"
	"fmt"
	"strings"
)

type DiscoveryDocument struct {
	Issuer  string
	JWKSURI string
}

func ParseDiscoveryDocument(body []byte) (*DiscoveryDocument, error) {
	var values map[string]any
	if err := json.Unmarshal(body, &values); err != nil {
		return nil, fmt.Errorf("failed to decode discovery document: %w", err)
	}

	issuer, _ := values["issuer"].(string)
	jwksURI, _ := values["jwks_uri"].(string)
	return &DiscoveryDocument{
		Issuer:  issuer,
		JWKSURI: jwksURI,
	}, nil
}

func RewriteDiscoveryDocument(body []byte, issuerURL string) ([]byte, error) {
	var values map[string]any
	if err := json.Unmarshal(body, &values); err != nil {
		return nil, fmt.Errorf("failed to decode discovery document: %w", err)
	}

	issuerURL = strings.TrimRight(issuerURL, "/")
	if issuerURL != "" {
		values["issuer"] = issuerURL
		values["jwks_uri"] = issuerURL + JWKSPath
	}

	updatedBody, err := json.Marshal(values)
	if err != nil {
		return nil, fmt.Errorf("failed to encode discovery document: %w", err)
	}
	return updatedBody, nil
}
