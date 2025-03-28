package main

import (
	"crypto/rand"
	"fmt"
	"io"
	"log"
)

type Pair[T, U any] struct {
	first  T
	second U
}

func MakePair[T, U any](first T, second U) Pair[T, U] {
	return Pair[T, U]{first: first, second: second}
}

func CloseReadCloserWithLog(c io.ReadCloser) {
	if err := c.Close(); err != nil {
		log.Printf("failed to close resource: %v", err)
	}
}

type Number interface {
	int | int8 | int16 | int32 | int64 | uint | uint8 | uint16 | uint32 | uint64
}

func ceilDiv[T Number](a, b T) T {
	if b == 0 {
		panic("division by zero")
	}
	if a%b == 0 {
		return a / b
	}
	return (a / b) + 1
}

func assertAndReturnPerfectDivision[T Number](a, b T) T {
	if b == 0 {
		log.Fatalln("division by zero")
	}
	if a%b != 0 {
		log.Fatalln("the division is not perfect and leaves a remainder")
	}
	return a / b
}

// generateLocalPeerId generates a Peer ID for the client.
func generateLocalPeerId() ([20]byte, error) {
	var localPeerId [20]byte

	prefix := "-PTC001-"
	copy(localPeerId[:], prefix)

	randomBytes := make([]byte, 13)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return localPeerId, fmt.Errorf("failed to generate random bytes: %v", err)
	}
	copy(localPeerId[7:], randomBytes)

	return localPeerId, nil
}
