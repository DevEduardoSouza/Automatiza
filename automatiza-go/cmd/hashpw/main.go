// Package main provides a CLI tool to generate Argon2id password hashes.
// Usage: go run ./cmd/hashpw/main.go "YourPassword123"
package main

import (
	"fmt"
	"os"

	"automatiza-go/internal/auth"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run ./cmd/hashpw/main.go <password>")
		fmt.Println("")
		fmt.Println("Example:")
		fmt.Println("  go run ./cmd/hashpw/main.go \"Test123456\"")
		os.Exit(1)
	}

	password := os.Args[1]

	// Validate password complexity
	if err := auth.ValidatePasswordComplexity(password); err != nil {
		fmt.Fprintf(os.Stderr, "Password validation failed: %v\n", err)
		fmt.Println("")
		fmt.Println("Password requirements:")
		fmt.Println("  - At least 8 characters")
		fmt.Println("  - At most 128 characters")
		fmt.Println("  - At least one lowercase letter")
		fmt.Println("  - At least one uppercase letter")
		fmt.Println("  - At least one digit")
		os.Exit(1)
	}

	// Generate hash
	hash, err := auth.HashPassword(password)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to hash password: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Password Hash (Argon2id):")
	fmt.Println("")
	fmt.Println(hash)
	fmt.Println("")
	fmt.Println("Copy this hash to scripts/db/seed.sql")
}
