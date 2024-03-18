package concurrentlimit

import (
	"errors"
	"time"
)

// ConcurrentLimit limits functions to a specific concurrency
// This is goroutine safe, and concurrency is bounded by the size of c
type ConcurrentLimit struct {
	c chan bool
}

// Run f with the specified concurrency limit
func (c *ConcurrentLimit) Run(f func() error) error {
	// we push to start, and pop to finish
	c.c <- true
	err := f()
	<-c.c
	return err
}

// RunWithTimeout runs f with the specified concurrency limit and times out waiting duration for an available slot
func (c *ConcurrentLimit) RunWithTimeout(f func() error, d time.Duration) error {
	// we push to start, and pop to finish
	select {
	case c.c <- true:
		err := f()
		<-c.c
		return err
	case <-time.After(d):
		return errors.New("timed out")
	}
}

// NewConcurrentLimit constructs a ConcurrentLimit with a limit of n
func NewConcurrentLimit(n int) *ConcurrentLimit {
	return &ConcurrentLimit{make(chan bool, n)}
}
