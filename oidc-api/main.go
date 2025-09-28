package main

import (
	"fmt"

	"github.com/gin-gonic/gin"
)

func main() {
	fmt.Printf("Starting OIDC Connect API Server...\n")

	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "ok",
		})
	})

	r.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "Welcome to the OIDC Connect API Server",
		})
	})

	r.GET("/.well-known/openid-configuration", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"issuer":                                "https://kubernetes.default.svc.cluster.local",
			"jwks_uri":                              "https://192.168.1.42:6443/openid/v1/jwks",
			"response_types_supported":              []string{"id_token"},
			"subject_types_supported":               []string{"public"},
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})

	r.Run(":8080")
}
