// Package resilience wraps external-service calls with the shared circuit
// breaker so a failing dependency (Vault, ArgoCD, Git, Kubernetes, ...) trips
// fast instead of cascading.
package resilience

import "github.com/kriipke/platformctl/internal/circuitbreaker"

// New builds a circuit breaker for a named external service, using the default
// per-service tuning when one exists.
func New(service string) *circuitbreaker.CircuitBreaker {
	cfg, ok := circuitbreaker.DefaultServiceConfigs()[service]
	if !ok {
		cfg = circuitbreaker.Config{}
	}
	cfg.Name = service
	return circuitbreaker.New(cfg)
}

// Run executes fn through the circuit breaker while preserving fn's typed
// result. When the breaker is open (or fn fails) the typed zero value and the
// error are returned.
func Run[T any](cb *circuitbreaker.CircuitBreaker, fn func() (T, error)) (T, error) {
	res, err := cb.Execute(func() (interface{}, error) {
		return fn()
	})
	if err != nil {
		var zero T
		return zero, err
	}
	v, _ := res.(T)
	return v, nil
}
