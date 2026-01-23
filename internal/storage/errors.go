package storage

import "errors"

var (
	// ErrNotFound indicates that the requested resource was not found
	ErrNotFound = errors.New("resource not found")
	
	// ErrConflict indicates that the resource already exists
	ErrConflict = errors.New("resource already exists")
	
	// ErrInvalidInput indicates invalid input data
	ErrInvalidInput = errors.New("invalid input data")
)