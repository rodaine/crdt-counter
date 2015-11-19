package main

import (
	"math/rand"
	"time"
)

func simulateLatency(max time.Duration) <-chan time.Time {
	return time.After(time.Duration(rand.Int63n(int64(max))))
}
