package circuitbreaker

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrCircuitOpen     = errors.New("circuit breaker is open")
	ErrTooManyRequests = errors.New("too many requests")
	ErrTimeout         = errors.New("timeout")
)

// State represents the state of the circuit breaker
type State int

const (
	StateClosed State = iota
	StateHalfOpen
	StateOpen
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateHalfOpen:
		return "half-open"
	case StateOpen:
		return "open"
	default:
		return fmt.Sprintf("unknown state: %d", s)
	}
}

// Config holds the configuration for a circuit breaker
type Config struct {
	Name             string                                  // Name of the circuit breaker for monitoring
	MaxRequests      uint32                                  // Maximum requests allowed in half-open state
	Interval         time.Duration                           // Statistical window for failure counting
	Timeout          time.Duration                           // Time to wait before transitioning from open to half-open
	FailureThreshold uint32                                  // Number of failures to trigger open state
	SuccessThreshold uint32                                  // Number of successes to trigger closed state from half-open
	ShouldTrip       func(counts Counts) bool                // Custom logic to determine if breaker should trip
	OnStateChange    func(name string, from State, to State) // Callback for state changes
	IsSuccessful     func(err error) bool                    // Determines if an error should count as failure
}

// Counts holds the statistics for the circuit breaker
type Counts struct {
	Requests             uint32 // Total requests in current window
	TotalSuccesses       uint32 // Total successful requests
	TotalFailures        uint32 // Total failed requests
	ConsecutiveSuccesses uint32 // Consecutive successes (used in half-open state)
	ConsecutiveFailures  uint32 // Consecutive failures
}

func (c Counts) FailureRate() float64 {
	if c.Requests == 0 {
		return 0.0
	}
	return float64(c.TotalFailures) / float64(c.Requests)
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	name             string
	maxRequests      uint32
	successThreshold uint32
	interval         time.Duration
	timeout          time.Duration
	shouldTrip       func(counts Counts) bool
	onStateChange    func(name string, from State, to State)
	isSuccessful     func(err error) bool

	mutex      sync.RWMutex
	state      State
	generation uint64
	counts     Counts
	expiry     time.Time
}

// New creates a new CircuitBreaker with the given config
func New(config Config) *CircuitBreaker {
	cb := &CircuitBreaker{
		name:             config.Name,
		maxRequests:      config.MaxRequests,
		successThreshold: config.SuccessThreshold,
		interval:         config.Interval,
		timeout:          config.Timeout,
		shouldTrip:       config.ShouldTrip,
		onStateChange:    config.OnStateChange,
		isSuccessful:     config.IsSuccessful,
	}

	if cb.name == "" {
		cb.name = "CircuitBreaker"
	}

	if cb.maxRequests == 0 {
		cb.maxRequests = 1
	}

	if cb.successThreshold == 0 {
		cb.successThreshold = 1
	}

	if cb.interval <= 0 {
		cb.interval = time.Minute
	}

	if cb.timeout <= 0 {
		cb.timeout = 60 * time.Second
	}

	if cb.shouldTrip == nil {
		cb.shouldTrip = func(counts Counts) bool {
			return counts.ConsecutiveFailures > 5
		}
	}

	if cb.isSuccessful == nil {
		cb.isSuccessful = func(err error) bool {
			return err == nil
		}
	}

	cb.toNewGeneration(time.Now())

	return cb
}

// Execute runs the given request if the circuit breaker allows it
func (cb *CircuitBreaker) Execute(req func() (interface{}, error)) (interface{}, error) {
	generation, err := cb.beforeRequest()
	if err != nil {
		return nil, err
	}

	defer func() {
		e := recover()
		if e != nil {
			cb.afterRequest(generation, false)
			panic(e)
		}
	}()

	result, err := req()
	cb.afterRequest(generation, cb.isSuccessful(err))
	return result, err
}

// ExecuteContext runs the given request with context support
func (cb *CircuitBreaker) ExecuteContext(ctx context.Context, req func(context.Context) (interface{}, error)) (interface{}, error) {
	generation, err := cb.beforeRequest()
	if err != nil {
		return nil, err
	}

	defer func() {
		e := recover()
		if e != nil {
			cb.afterRequest(generation, false)
			panic(e)
		}
	}()

	result, err := req(ctx)
	cb.afterRequest(generation, cb.isSuccessful(err))
	return result, err
}

// State returns the current state of the circuit breaker
func (cb *CircuitBreaker) State() State {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()

	now := time.Now()
	state, _ := cb.currentState(now)
	return state
}

// Counts returns the current counts
func (cb *CircuitBreaker) Counts() Counts {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()

	return cb.counts
}

// Name returns the name of the circuit breaker
func (cb *CircuitBreaker) Name() string {
	return cb.name
}

func (cb *CircuitBreaker) beforeRequest() (uint64, error) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	now := time.Now()
	state, generation := cb.currentState(now)

	if state == StateOpen {
		return generation, ErrCircuitOpen
	} else if state == StateHalfOpen && cb.counts.Requests >= cb.maxRequests {
		return generation, ErrTooManyRequests
	}

	cb.counts.Requests++
	return generation, nil
}

func (cb *CircuitBreaker) afterRequest(before uint64, success bool) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	now := time.Now()
	state, generation := cb.currentState(now)
	if generation != before {
		return
	}

	if success {
		cb.onSuccess(state, now)
	} else {
		cb.onFailure(state, now)
	}
}

func (cb *CircuitBreaker) onSuccess(state State, now time.Time) {
	cb.counts.TotalSuccesses++
	cb.counts.ConsecutiveFailures = 0
	cb.counts.ConsecutiveSuccesses++

	if state == StateHalfOpen && cb.counts.ConsecutiveSuccesses >= cb.successThreshold {
		cb.setState(StateClosed, now)
	}
}

func (cb *CircuitBreaker) onFailure(state State, now time.Time) {
	cb.counts.TotalFailures++
	cb.counts.ConsecutiveSuccesses = 0
	cb.counts.ConsecutiveFailures++

	if cb.shouldTrip(cb.counts) {
		cb.setState(StateOpen, now)
	}
}

func (cb *CircuitBreaker) currentState(now time.Time) (State, uint64) {
	switch cb.state {
	case StateClosed:
		if !cb.expiry.IsZero() && cb.expiry.Before(now) {
			cb.toNewGeneration(now)
		}
	case StateOpen:
		if cb.expiry.Before(now) {
			cb.setState(StateHalfOpen, now)
		}
	}
	return cb.state, cb.generation
}

func (cb *CircuitBreaker) setState(state State, now time.Time) {
	if cb.state == state {
		return
	}

	prev := cb.state
	cb.state = state

	cb.toNewGeneration(now)

	if cb.onStateChange != nil {
		cb.onStateChange(cb.name, prev, state)
	}
}

func (cb *CircuitBreaker) toNewGeneration(now time.Time) {
	cb.generation++
	cb.counts = Counts{}

	var zero time.Time
	switch cb.state {
	case StateClosed:
		if cb.interval == 0 {
			cb.expiry = zero
		} else {
			cb.expiry = now.Add(cb.interval)
		}
	case StateOpen:
		cb.expiry = now.Add(cb.timeout)
	default: // StateHalfOpen
		cb.expiry = zero
	}
}
