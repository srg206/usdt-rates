package closer

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type Closer struct {
	mu       sync.Mutex
	once     sync.Once
	done     chan struct{}
	closers  []func() error
	timeout  time.Duration
	shutdown bool
}

func New(timeout time.Duration) *Closer {
	return &Closer{
		done:    make(chan struct{}),
		closers: make([]func() error, 0),
		timeout: timeout,
	}
}

func (c *Closer) Add(fn func() error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closers = append(c.closers, fn)
}

func (c *Closer) Close() error {
	var err error
	c.once.Do(func() {
		c.mu.Lock()
		c.shutdown = true
		closers := make([]func() error, len(c.closers))
		copy(closers, c.closers)
		c.mu.Unlock()

		for i := len(closers) - 1; i >= 0; i-- {
			if closeErr := closers[i](); closeErr != nil {
				if err == nil {
					err = closeErr
				} else {
					log.Printf("error closing resource: %v", closeErr)
				}
			}
		}
		close(c.done)
	})
	return err
}

func (c *Closer) Wait() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	sig := <-sigChan
	log.Printf("received signal: %v, starting graceful shutdown...", sig)

	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	closeDone := make(chan error, 1)
	go func() {
		closeDone <- c.Close()
	}()

	select {
	case err := <-closeDone:
		if err != nil {
			log.Printf("error during shutdown: %v", err)
		} else {
			log.Println("graceful shutdown completed successfully")
		}
	case <-ctx.Done():
		log.Printf("shutdown timeout exceeded (%v), forcing exit", c.timeout)
	}
}

func (c *Closer) IsShutdown() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.shutdown
}
