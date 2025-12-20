package circuitbreaker

import (
	"errors"
	"sync"
	"time"
)

type State int

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

type Config struct {
	FailureThreshold int
	SuccessThreshold int
	OpenTimeout      time.Duration
}

type Breaker struct {
	mu             sync.Mutex
	state          State
	failureCount   int
	successCount   int
	reopenDeadline time.Time
	cfg            Config
}

var ErrOpen = errors.New("circuit open")

func New(cfg Config) *Breaker {
	if cfg.FailureThreshold <= 0 {
		cfg.FailureThreshold = 3
	}
	if cfg.SuccessThreshold <= 0 {
		cfg.SuccessThreshold = 1
	}
	if cfg.OpenTimeout <= 0 {
		cfg.OpenTimeout = 5 * time.Second
	}

	return &Breaker{cfg: cfg}
}

func (cb *Breaker) Allow() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()
	if cb.state == StateOpen {
		if now.Before(cb.reopenDeadline) {
			return ErrOpen
		}
		cb.transitionTo(StateHalfOpen)
	}

	return nil
}

func (cb *Breaker) MarkSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == StateHalfOpen {
		cb.successCount++
		if cb.successCount >= cb.cfg.SuccessThreshold {
			cb.reset()
		}
		return
	}

	cb.reset()
}

func (cb *Breaker) MarkFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failureCount++
	if cb.failureCount >= cb.cfg.FailureThreshold {
		cb.trip()
	}
}

func (cb *Breaker) transitionTo(state State) {
	cb.state = state
	cb.failureCount = 0
	cb.successCount = 0
	if state == StateOpen {
		cb.reopenDeadline = time.Now().Add(cb.cfg.OpenTimeout)
	}
}

func (cb *Breaker) reset() {
	cb.transitionTo(StateClosed)
}

func (cb *Breaker) trip() {
	cb.transitionTo(StateOpen)
}
