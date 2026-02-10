// Package main provides a CLI tool to generate JWT tokens for PostgREST.
// Usage: go run ./cmd/gentokens/main.go
package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// PostgRESTClaims represents the JWT claims for PostgREST
type PostgRESTClaims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

func main() {
	// Get or generate JWT secret
	secret := os.Getenv("POSTGREST_JWT_SECRET")
	if secret == "" {
		// Generate a secure random secret
		secretBytes := make([]byte, 32)
		if _, err := rand.Read(secretBytes); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to generate secret: %v\n", err)
			os.Exit(1)
		}
		secret = base64.RawURLEncoding.EncodeToString(secretBytes)
	}

	// Token expiration (10 years for development)
	expiration := time.Now().Add(10 * 365 * 24 * time.Hour)

	// Generate anon token
	anonToken, err := generateToken(secret, "anon", expiration)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate anon token: %v\n", err)
		os.Exit(1)
	}

	// Generate service_role token
	serviceRoleToken, err := generateToken(secret, "service_role", expiration)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate service_role token: %v\n", err)
		os.Exit(1)
	}

	// Output
	separator := strings.Repeat("=", 80)
	fmt.Println(separator)
	fmt.Println("PostgREST JWT Tokens")
	fmt.Println(separator)
	fmt.Println("")
	fmt.Println("Add these to your .env file:")
	fmt.Println("")
	fmt.Printf("POSTGREST_JWT_SECRET=%s\n", secret)
	fmt.Println("")
	fmt.Printf("POSTGREST_ANON_KEY=%s\n", anonToken)
	fmt.Println("")
	fmt.Printf("POSTGREST_SERVICE_ROLE_KEY=%s\n", serviceRoleToken)
	fmt.Println("")
	fmt.Println(separator)
	fmt.Println("")
	fmt.Println("Token details:")
	fmt.Printf("  Expiration: %s\n", expiration.Format(time.RFC3339))
	fmt.Printf("  Algorithm: HS256\n")
	fmt.Println("")

	// Decode and show token content for verification
	fmt.Println("Anon token payload:")
	showTokenPayload(anonToken)
	fmt.Println("")
	fmt.Println("Service role token payload:")
	showTokenPayload(serviceRoleToken)
}

func generateToken(secret, role string, expiration time.Time) (string, error) {
	claims := PostgRESTClaims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiration),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "automatiza",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func showTokenPayload(tokenString string) {
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		fmt.Printf("  Error parsing token: %v\n", err)
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		fmt.Println("  Invalid claims")
		return
	}

	data, _ := json.MarshalIndent(claims, "  ", "  ")
	fmt.Printf("  %s\n", data)
}
