package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestGenerateCredentialProviderResponseExchangesServiceAccountTokenWithSTS(t *testing.T) {
	t.Setenv("STS_SUBJECT_TOKEN", "")

	oldHTTPClient := stsHTTPClient
	t.Cleanup(func() {
		stsHTTPClient = oldHTTPClient
	})

	stsHTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodPost {
			t.Errorf("method = %s, want %s", request.Method, http.MethodPost)
		}

		if got := request.Header.Get("User-Agent"); got != "yaak" {
			t.Errorf("User-Agent = %q, want %q", got, "yaak")
		}

		if got := request.Header.Get("Accept"); got != "application/json" {
			t.Errorf("Accept = %q, want %q", got, "application/json")
		}

		if got := request.Header.Get("Content-Type"); got != "application/x-www-form-urlencoded" {
			t.Errorf("Content-Type = %q, want %q", got, "application/x-www-form-urlencoded")
		}

		if err := request.ParseForm(); err != nil {
			t.Errorf("ParseForm() error = %v", err)
		}

		wantForm := map[string]string{
			"subject_token":        "service-account-jwt",
			"subject_token_type":   subjectTokenTypeJWT,
			"grant_type":           tokenExchangeGrantType,
			"scope":                "https://www.googleapis.com/auth/cloud-platform",
			"requested_token_type": requestedAccessTokenTyp,
			"audience":             "//iam.googleapis.com/projects/example/locations/global/workloadIdentityPools/pool/providers/provider",
		}

		for key, want := range wantForm {
			if got := request.Form.Get(key); got != want {
				t.Errorf("%s = %q, want %q", key, got, want)
			}
		}

		responseBody := strings.Builder{}
		if err := json.NewEncoder(&responseBody).Encode(stsTokenResponse{
			AccessToken: "access-token",
			ExpiresIn:   3600,
		}); err != nil {
			t.Fatalf("Encode() error = %v", err)
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(responseBody.String())),
			Request:    request,
		}, nil
	})}

	cfg := &config{
		imagePrefix: "us-central1-docker.pkg.dev/example-project/internal/",
		registry:    "us-central1-docker.pkg.dev",
		stsTokenURL: "https://sts.example.test/v1/token",
		stsAudience: "//iam.googleapis.com/projects/example/locations/global/workloadIdentityPools/pool/providers/provider",
		stsScope:    defaultSTSScope,
		userAgent:   defaultUserAgent,
	}

	credentialResponse, err := generateCredentialProviderResponse(&CredentialProviderRequest{
		APIVersion:          "credentialprovider.kubelet.k8s.io/v1",
		Image:               "us-central1-docker.pkg.dev/example-project/internal/image:tag",
		ServiceAccountToken: "service-account-jwt",
	}, cfg)
	if err != nil {
		t.Fatalf("generateCredentialProviderResponse() error = %v", err)
	}

	if credentialResponse.APIVersion != "credentialprovider.kubelet.k8s.io/v1" {
		t.Fatalf("APIVersion = %q", credentialResponse.APIVersion)
	}

	if credentialResponse.Kind != "CredentialProviderResponse" {
		t.Fatalf("Kind = %q", credentialResponse.Kind)
	}

	if credentialResponse.CacheKeyType != PluginCacheKeyTypeImage {
		t.Fatalf("CacheKeyType = %q", credentialResponse.CacheKeyType)
	}

	if credentialResponse.CacheDuration != "59m0s" {
		t.Fatalf("CacheDuration = %q, want %q", credentialResponse.CacheDuration, "59m0s")
	}

	auth := credentialResponse.Auth["us-central1-docker.pkg.dev"]
	if auth.Username != dockerAccessTokenUsername {
		t.Fatalf("Username = %q, want %q", auth.Username, dockerAccessTokenUsername)
	}

	if auth.Password != "access-token" {
		t.Fatalf("Password = %q, want %q", auth.Password, "access-token")
	}
}
