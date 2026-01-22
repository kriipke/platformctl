package auth

import "errors"

var (
	// ErrInvalidCredentials indicates invalid username/password
	ErrInvalidCredentials = errors.New("invalid credentials")

	// ErrNoCustomerInContext indicates no customer found in request context
	ErrNoCustomerInContext = errors.New("no customer found in context")

	// ErrUnauthorized indicates the customer is not authorized for the operation
	ErrUnauthorized = errors.New("unauthorized")
)