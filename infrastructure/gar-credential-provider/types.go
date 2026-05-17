package main

type CredentialProviderRequest struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`

	Image                     string            `json:"image"`
	ServiceAccountToken       string            `json:"serviceAccountToken"`
	ServiceAccountAnnotations map[string]string `json:"serviceAccountAnnotations"`
}

type PluginCacheKeyType string

const (
	PluginCacheKeyTypeImage PluginCacheKeyType = "Image"
)

type AuthConfig struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type CredentialProviderResponse struct {
	APIVersion    string                `json:"apiVersion"`
	Kind          string                `json:"kind"`
	CacheKeyType  PluginCacheKeyType    `json:"cacheKeyType"`
	CacheDuration string                `json:"cacheDuration,omitempty"`
	Auth          map[string]AuthConfig `json:"auth"`
}

type generateAccessTokenRequest struct {
	Scope []string `json:"scope"`
}

type generateAccessTokenResponse struct {
	AccessToken string `json:"accessToken"`
	ExpireTime  string `json:"expireTime"`
}
