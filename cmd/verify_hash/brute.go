//go:build brute
// +build brute

package main

import (
	"fmt"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	hash := "$2a$10$ytiVGYNGt1ARF9gWvKY.hOXZ4B5iYxCw8ZBCPv89AmOiPuAU0bGKi"
	candidates := []string{
		"password123",
		"password123\n",
		"password123\r\n",
		"password123 ",
		" password123",
		"admin@example.com", // 可能是把邮箱做了哈希？
		"",
	}

	for _, p := range candidates {
		err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(p))
		if err == nil {
			fmt.Printf("MATCH FOUND: %q\n", p)
			return
		}
	}
	fmt.Println("No match found")
}
