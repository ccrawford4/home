package kubernetes

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"
)

const serviceAccountCAFile = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"

type RawClientOptions struct {
	APIServerURL string
	HTTPClient   *http.Client
}

type RawClient struct {
	apiServerURL string
	httpClient   *http.Client
	tokenSource  *BearerTokenSource
}

func NewRawClient(options RawClientOptions) (*RawClient, error) {
	httpClient := options.HTTPClient
	if httpClient == nil {
		var err error
		httpClient, err = newHTTPClient()
		if err != nil {
			return nil, err
		}
	}

	return &RawClient{
		apiServerURL: options.APIServerURL,
		httpClient:   httpClient,
		tokenSource: &BearerTokenSource{
			tokenFile: serviceAccountTokenFile,
		},
	}, nil
}

func (c *RawClient) GetRaw(ctx context.Context, rawPath string) ([]byte, error) {
	apiURL, err := url.Parse(c.apiServerURL)
	if err != nil {
		return nil, fmt.Errorf("invalid Kubernetes API URL %q: %w", c.apiServerURL, err)
	}
	apiURL.Path = path.Join(apiURL.Path, rawPath)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL.String(), nil)
	if err != nil {
		return nil, err
	}

	token, err := c.tokenSource.Token()
	if err != nil {
		return nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Kubernetes API returned %s for %s: %s", resp.Status, rawPath, strings.TrimSpace(string(body)))
	}

	return body, nil
}

func newHTTPClient() (*http.Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	ca, err := os.ReadFile(serviceAccountCAFile)
	if err == nil {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(ca) {
			return nil, fmt.Errorf("failed to parse Kubernetes CA file %s", serviceAccountCAFile)
		}
		transport.TLSClientConfig.RootCAs = pool
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to read Kubernetes CA file %s: %w", serviceAccountCAFile, err)
	}

	return &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}, nil
}
