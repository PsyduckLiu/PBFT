package main

import (
	"crypto/sha256"
	"fmt"
)

func main() {
	// method 1
	sum := sha256.Sum256([]byte("hello world\n"))
	fmt.Printf("%x\n", sum)

	// method 2
	h := sha256.New()
	h.Write([]byte("hello world\n"))
	fmt.Printf("%x\n", h.Sum(nil))
}
