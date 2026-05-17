package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	defaultSTSTokenURL = "https://sts.googleapis.com/v1/token"
	defaultSTSScope    = "https://www.googleapis.com/auth/cloud-platform"
	defaultUserAgent   = "yaak"

	subjectTokenTypeJWT     = "urn:ietf:params:oauth:token-type:jwt"
	tokenExchangeGrantType  = "urn:ietf:params:oauth:grant-type:token-exchange"
	requestedAccessTokenTyp = "urn:ietf:params:oauth:token-type:access_token"

	dockerAccessTokenUsername = "oauth2accesstoken"
)

var (
	stsHTTPClient = &http.Client{Timeout: 10 * time.Second}
	logger        = slog.New(slog.NewJSONHandler(os.Stderr, nil))
)

type config struct {
	imagePrefix           string
	registry              string
	stsTokenURL           string
	stsAudience           string
	stsScope              string
	userAgent             string
	serviceAccountEmail   string
	iamCredentialsBaseURL string
}

type stsTokenResponse struct {
	AccessToken     string `json:"access_token"`
	IssuedTokenType string `json:"issued_token_type"`
	TokenType       string `json:"token_type"`
	ExpiresIn       int    `json:"expires_in"`
	Scope           string `json:"scope"`

	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

func parseCredentialProviderRequest(stdin io.Reader) (*CredentialProviderRequest, error) {
	var request CredentialProviderRequest

	if err := json.NewDecoder(stdin).Decode(&request); err != nil {
		logger.ErrorContext(context.Background(), "failed to decode credential provider request", "error", err)
		return nil, fmt.Errorf("failed to decode credential provider request: %w", err)
	}

	logger.DebugContext(context.Background(), "parsed credential provider request", "image", request.Image, "apiVersion", request.APIVersion)
	return &request, nil

}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}

	return fallback
}

func requiredEnv(name string) (string, error) {
	value := os.Getenv(name)
	if value == "" {
		return "", fmt.Errorf("%s must be set", name)
	}

	return value, nil
}

func registryFromImagePrefix(imagePrefix string) string {
	registry, _, _ := strings.Cut(imagePrefix, "/")
	return registry
}

func loadConfig() (*config, error) {
	logger.DebugContext(context.Background(), "loading required environment variables")

	imagePrefix, err := requiredEnv("GAR_IMAGE_PREFIX")
	if err != nil {
		logger.ErrorContext(context.Background(), "missing required config", "env_var", "GAR_IMAGE_PREFIX", "error", err)
		return nil, err
	}
	logger.DebugContext(context.Background(), "loaded GAR_IMAGE_PREFIX", "value", imagePrefix)

	stsAudience, err := requiredEnv("STS_AUDIENCE")
	if err != nil {
		logger.ErrorContext(context.Background(), "missing required config", "env_var", "STS_AUDIENCE", "error", err)
		return nil, err
	}
	logger.DebugContext(context.Background(), "loaded STS_AUDIENCE")

	serviceAccountEmail, err := requiredEnv("SERVICE_ACCOUNT_EMAIL")
	if err != nil {
		logger.ErrorContext(context.Background(), "missing required config", "env_var", "SERVICE_ACCOUNT_EMAIL", "error", err)
		return nil, err
	}
	logger.DebugContext(context.Background(), "loaded SERVICE_ACCOUNT_EMAIL")

	logger.DebugContext(context.Background(), "loading optional environment variables")
	registry := envOrDefault("GAR_REGISTRY_HOST", registryFromImagePrefix(imagePrefix))
	if registry == "" {
		err := fmt.Errorf("GAR_REGISTRY_HOST must be set when GAR_IMAGE_PREFIX does not include a registry host")
		logger.ErrorContext(context.Background(), "invalid config", "error", err)
		return nil, err
	}
	logger.DebugContext(context.Background(), "resolved registry", "registry", registry)

	cfg := &config{
		imagePrefix:           imagePrefix,
		registry:              registry,
		stsTokenURL:           envOrDefault("STS_TOKEN_URL", defaultSTSTokenURL),
		stsAudience:           stsAudience,
		stsScope:              envOrDefault("STS_SCOPE", defaultSTSScope),
		userAgent:             envOrDefault("STS_USER_AGENT", defaultUserAgent),
		serviceAccountEmail:   serviceAccountEmail,
		iamCredentialsBaseURL: envOrDefault("IAM_CREDENTIALS_BASE_URL", "https://iamcredentials.googleapis.com/v1"),
	}

	logger.InfoContext(context.Background(), "config loaded", "registry", cfg.registry, "stsTokenURL", cfg.stsTokenURL, "serviceAccountEmail", cfg.serviceAccountEmail)
	return cfg, nil
}

func subjectTokenForRequest(request *CredentialProviderRequest) (string, error) {
	if subjectToken := os.Getenv("STS_SUBJECT_TOKEN"); subjectToken != "" {
		logger.DebugContext(context.Background(), "using subject token from environment")
		return subjectToken, nil
	}

	if request.ServiceAccountToken == "" {
		err := fmt.Errorf("serviceAccountToken is empty and STS_SUBJECT_TOKEN is not set")
		logger.ErrorContext(context.Background(), "no subject token available", "error", err)
		return "", err
	}

	logger.DebugContext(context.Background(), "using subject token from credential provider request")
	return request.ServiceAccountToken, nil
}

func requestSTSToken(cfg *config, subjectToken string) (*stsTokenResponse, error) {
	logger.DebugContext(context.Background(), "preparing STS token request", "url", cfg.stsTokenURL, "audience", cfg.stsAudience)

	form := url.Values{}
	form.Set("subject_token", subjectToken)
	form.Set("subject_token_type", subjectTokenTypeJWT)
	form.Set("grant_type", tokenExchangeGrantType)
	form.Set("scope", cfg.stsScope)
	form.Set("requested_token_type", requestedAccessTokenTyp)
	form.Set("audience", cfg.stsAudience)

	httpRequest, err := http.NewRequest(http.MethodPost, cfg.stsTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		logger.ErrorContext(context.Background(), "failed to create STS token request", "url", cfg.stsTokenURL, "error", err)
		return nil, fmt.Errorf("failed to create STS token request: %w", err)
	}

	httpRequest.Header.Set("User-Agent", cfg.userAgent)
	httpRequest.Header.Set("Accept", "application/json")
	httpRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	logger.InfoContext(context.Background(), "sending request to STS token API", "url", cfg.stsTokenURL)
	httpResponse, err := stsHTTPClient.Do(httpRequest)
	if err != nil {
		logger.ErrorContext(context.Background(), "failed to call STS token API", "url", cfg.stsTokenURL, "error", err)
		return nil, fmt.Errorf("failed to call STS token API: %w", err)
	}
	defer httpResponse.Body.Close()

	var tokenResponse stsTokenResponse
	if err := json.NewDecoder(httpResponse.Body).Decode(&tokenResponse); err != nil {
		logger.ErrorContext(context.Background(), "failed to decode STS token response", "status", httpResponse.Status, "error", err)
		return nil, fmt.Errorf("failed to decode STS token response: %w", err)
	}

	logger.InfoContext(context.Background(), "received response from STS token API", "status", httpResponse.Status, "statusCode", httpResponse.StatusCode)

	if httpResponse.StatusCode < 200 || httpResponse.StatusCode >= 300 {
		if tokenResponse.Error != "" {
			logger.ErrorContext(context.Background(), "STS token API error response", "status", httpResponse.Status, "stsError", tokenResponse.Error, "description", tokenResponse.ErrorDescription)
			return nil, fmt.Errorf("STS token API returned %s: %s: %s", httpResponse.Status, tokenResponse.Error, tokenResponse.ErrorDescription)
		}

		logger.ErrorContext(context.Background(), "STS token API non-2xx response", "status", httpResponse.Status)
		return nil, fmt.Errorf("STS token API returned %s", httpResponse.Status)
	}

	if tokenResponse.AccessToken == "" {
		err := fmt.Errorf("STS token API response did not include access_token")
		logger.ErrorContext(context.Background(), "missing access token in response", "error", err)
		return nil, err
	}

	logger.InfoContext(context.Background(), "successfully obtained STS token", "expiresIn", tokenResponse.ExpiresIn)
	return &tokenResponse, nil
}

func impersonateServiceAccount(cfg *config, stsToken string) (*generateAccessTokenResponse, error) {
	logger.DebugContext(context.Background(), "preparing service account impersonation request", "serviceAccount", cfg.serviceAccountEmail)

	requestBody := &generateAccessTokenRequest{
		Scope: []string{defaultSTSScope},
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		logger.ErrorContext(context.Background(), "failed to marshal impersonation request", "error", err)
		return nil, fmt.Errorf("failed to marshal impersonation request: %w", err)
	}

	iamURL := fmt.Sprintf("%s/projects/-/serviceAccounts/%s:generateAccessToken", cfg.iamCredentialsBaseURL, cfg.serviceAccountEmail)
	httpRequest, err := http.NewRequest(http.MethodPost, iamURL, strings.NewReader(string(bodyBytes)))
	if err != nil {
		logger.ErrorContext(context.Background(), "failed to create impersonation request", "url", iamURL, "error", err)
		return nil, fmt.Errorf("failed to create impersonation request: %w", err)
	}

	httpRequest.Header.Set("Authorization", fmt.Sprintf("Bearer %s", stsToken))
	httpRequest.Header.Set("User-Agent", cfg.userAgent)
	httpRequest.Header.Set("Accept", "application/json")
	httpRequest.Header.Set("Content-Type", "application/json")

	logger.InfoContext(context.Background(), "sending request to IAM Credentials API", "url", iamURL)
	httpResponse, err := stsHTTPClient.Do(httpRequest)
	if err != nil {
		logger.ErrorContext(context.Background(), "failed to call IAM Credentials API", "url", iamURL, "error", err)
		return nil, fmt.Errorf("failed to call IAM Credentials API: %w", err)
	}
	defer httpResponse.Body.Close()

	var iamResponse generateAccessTokenResponse
	if err := json.NewDecoder(httpResponse.Body).Decode(&iamResponse); err != nil {
		logger.ErrorContext(context.Background(), "failed to decode IAM Credentials response", "status", httpResponse.Status, "error", err)
		return nil, fmt.Errorf("failed to decode IAM Credentials response: %w", err)
	}

	logger.InfoContext(context.Background(), "received response from IAM Credentials API", "status", httpResponse.Status, "statusCode", httpResponse.StatusCode)

	if httpResponse.StatusCode < 200 || httpResponse.StatusCode >= 300 {
		logger.ErrorContext(context.Background(), "IAM Credentials API non-2xx response", "status", httpResponse.Status)
		return nil, fmt.Errorf("IAM Credentials API returned %s", httpResponse.Status)
	}

	if iamResponse.AccessToken == "" {
		err := fmt.Errorf("IAM Credentials API response did not include accessToken")
		logger.ErrorContext(context.Background(), "missing access token in response", "error", err)
		return nil, err
	}

	logger.InfoContext(context.Background(), "successfully obtained impersonated access token", "expireTime", iamResponse.ExpireTime)
	return &iamResponse, nil
}

func cacheDuration(tokenResponse *stsTokenResponse) string {
	if tokenResponse.ExpiresIn <= 60 {
		return ""
	}

	return (time.Duration(tokenResponse.ExpiresIn-60) * time.Second).String()
}

func generateCredentialProviderResponse(request *CredentialProviderRequest, cfg *config) (*CredentialProviderResponse, error) {
	logger.DebugContext(context.Background(), "validating image prefix", "image", request.Image, "expectedPrefix", cfg.imagePrefix)
	if !strings.HasPrefix(request.Image, cfg.imagePrefix) {
		logger.ErrorContext(context.Background(), "unsupported image", "image", request.Image, "expectedPrefix", cfg.imagePrefix)
		return nil, fmt.Errorf("unsupported image: %s", request.Image)
	}

	logger.InfoContext(context.Background(), "resolving subject token")
	subjectToken, err := subjectTokenForRequest(request)
	if err != nil {
		return nil, err
	}

	logger.InfoContext(context.Background(), "requesting STS token")
	stsTokenResponse, err := requestSTSToken(cfg, subjectToken)
	if err != nil {
		return nil, err
	}

	logger.InfoContext(context.Background(), "impersonating service account")
	iamTokenResponse, err := impersonateServiceAccount(cfg, stsTokenResponse.AccessToken)
	if err != nil {
		return nil, err
	}

	apiVersion := request.APIVersion
	if apiVersion == "" {
		apiVersion = "credentialprovider.kubelet.k8s.io/v1"
		logger.DebugContext(context.Background(), "using default API version")
	}

	response := &CredentialProviderResponse{
		APIVersion:    apiVersion,
		Kind:          "CredentialProviderResponse",
		CacheKeyType:  PluginCacheKeyTypeImage,
		CacheDuration: cacheDuration(stsTokenResponse),
		Auth: map[string]AuthConfig{
			cfg.registry: {
				Username: dockerAccessTokenUsername,
				Password: iamTokenResponse.AccessToken,
			},
		},
	}

	logger.InfoContext(context.Background(), "generated credential provider response", "image", request.Image, "registry", cfg.registry, "cacheDuration", response.CacheDuration)
	return response, nil
}

func main() {
	ctx := context.Background()

	logger.InfoContext(ctx, "credential provider starting")

	logger.InfoContext(ctx, "parsing credential provider request from stdin")
	request, err := parseCredentialProviderRequest(os.Stdin)
	if err != nil {
		logger.ErrorContext(ctx, "failed to parse credential provider request", "error", err)
		os.Exit(1)
	}

	logger.InfoContext(ctx, "loading configuration from environment", "image", request.Image)
	cfg, err := loadConfig()
	if err != nil {
		logger.ErrorContext(ctx, "failed to load config", "error", err)
		os.Exit(1)
	}

	logger.InfoContext(ctx, "generating credential provider response", "image", request.Image, "registry", cfg.registry)
	response, err := generateCredentialProviderResponse(request, cfg)
	if err != nil {
		logger.ErrorContext(ctx, "failed to generate credential provider response", "error", err)
		os.Exit(1)
	}

	logger.InfoContext(ctx, "encoding credential provider response to stdout")
	if err := json.NewEncoder(os.Stdout).Encode(response); err != nil {
		logger.ErrorContext(ctx, "failed to encode credential provider response", "error", err)
		os.Exit(1)
	}

	logger.InfoContext(ctx, "credential provider request completed successfully")
}
