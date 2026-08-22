package main

import (
	"fmt"
	"github.com/creamcroissant/mgpanel/internal/support/hash"
)

func main() {
	hashed := "$2a$10$ytiVGYNGt1ARF9gWvKY.hOXZ4B5iYxCw8ZBCPv89AmOiPuAU0bGKi"
	password := "password123"

	hasher, err := hash.NewBcryptHasher(10)
	if err != nil {
		panic(err)
	}

	err = hasher.Compare(hashed, password)
	if err != nil {
		fmt.Printf("Mismatch: %v\n", err)
	} else {
		fmt.Println("Match!")
	}
}
