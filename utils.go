package main

import (
	"fmt"
	"io"
	"log"
	mathRand "math/rand"
	"net"
	"time"
)

func CloseReadCloserWithLog(c io.ReadCloser) {
	if err := c.Close(); err != nil {
		log.Printf("failed to close resource: %v", err)
	}
}

func CloseConnectionWithLog(c net.Conn) {
	if err := c.Close(); err != nil {
		log.Printf("failed to close connection: %v", err)
	}
}

func selectSubset(n, k int) ([]int, error) {
	if n < k {
		return nil, fmt.Errorf("total elements must be at least %d", k)
	}

	elements := make([]int, n)
	for i := 0; i < n; i++ {
		elements[i] = i
	}

	// Shuffle the slice
	r := mathRand.New(mathRand.NewSource(time.Now().UnixNano())) // Seed the random number generator
	r.Shuffle(n, func(i, j int) {
		elements[i], elements[j] = elements[j], elements[i]
	})

	return elements[:k], nil
}
