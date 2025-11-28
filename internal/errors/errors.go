// Package errors provides common error handling utilities for the Doku CLI.
// It includes error types, error formatting helpers, and common error messages.
package errors

import (
	"errors"
	"fmt"
	"strings"

	"github.com/fatih/color"
)

// Common error types that can be used throughout the application
var (
	// ErrNotFound indicates a resource was not found
	ErrNotFound = errors.New("not found")

	// ErrAlreadyExists indicates a resource already exists
	ErrAlreadyExists = errors.New("already exists")

	// ErrInvalidConfig indicates invalid configuration
	ErrInvalidConfig = errors.New("invalid configuration")

	// ErrDockerUnavailable indicates Docker is not available
	ErrDockerUnavailable = errors.New("docker is not available")

	// ErrNotInitialized indicates Doku has not been initialized
	ErrNotInitialized = errors.New("doku is not initialized")

	// ErrPermissionDenied indicates insufficient permissions
	ErrPermissionDenied = errors.New("permission denied")

	// ErrTimeout indicates an operation timed out
	ErrTimeout = errors.New("operation timed out")

	// ErrUserCancelled indicates the user cancelled an operation
	ErrUserCancelled = errors.New("operation cancelled by user")
)

// ServiceError represents an error related to a specific service
type ServiceError struct {
	Service string
	Err     error
}

func (e *ServiceError) Error() string {
	return fmt.Sprintf("service '%s': %v", e.Service, e.Err)
}

func (e *ServiceError) Unwrap() error {
	return e.Err
}

// NewServiceError creates a new ServiceError
func NewServiceError(service string, err error) *ServiceError {
	return &ServiceError{
		Service: service,
		Err:     err,
	}
}

// ContainerError represents an error related to a Docker container
type ContainerError struct {
	ContainerName string
	Action        string
	Err           error
}

func (e *ContainerError) Error() string {
	return fmt.Sprintf("container '%s' %s: %v", e.ContainerName, e.Action, e.Err)
}

func (e *ContainerError) Unwrap() error {
	return e.Err
}

// NewContainerError creates a new ContainerError
func NewContainerError(containerName, action string, err error) *ContainerError {
	return &ContainerError{
		ContainerName: containerName,
		Action:        action,
		Err:           err,
	}
}

// ConfigError represents a configuration error
type ConfigError struct {
	Field   string
	Message string
	Err     error
}

func (e *ConfigError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("config error in '%s': %s: %v", e.Field, e.Message, e.Err)
	}
	return fmt.Sprintf("config error in '%s': %s", e.Field, e.Message)
}

func (e *ConfigError) Unwrap() error {
	return e.Err
}

// NewConfigError creates a new ConfigError
func NewConfigError(field, message string, err error) *ConfigError {
	return &ConfigError{
		Field:   field,
		Message: message,
		Err:     err,
	}
}

// ValidationError represents a validation error with multiple issues
type ValidationError struct {
	Errors []string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation failed: %s", strings.Join(e.Errors, "; "))
}

// Add adds a validation error message
func (e *ValidationError) Add(msg string) {
	e.Errors = append(e.Errors, msg)
}

// HasErrors returns true if there are validation errors
func (e *ValidationError) HasErrors() bool {
	return len(e.Errors) > 0
}

// NewValidationError creates a new ValidationError
func NewValidationError() *ValidationError {
	return &ValidationError{
		Errors: make([]string, 0),
	}
}

// Formatting helpers

// PrintError prints an error message in red
func PrintError(format string, args ...interface{}) {
	color.Red("✗ "+format, args...)
}

// PrintErrorf prints a formatted error message in red
func PrintErrorf(format string, args ...interface{}) {
	color.Red("✗ "+format, args...)
}

// PrintWarning prints a warning message in yellow
func PrintWarning(format string, args ...interface{}) {
	color.Yellow("⚠️  "+format, args...)
}

// PrintWarningf prints a formatted warning message in yellow
func PrintWarningf(format string, args ...interface{}) {
	color.Yellow("⚠️  "+format, args...)
}

// WarnOnError prints a warning if err is not nil
func WarnOnError(err error, format string, args ...interface{}) {
	if err != nil {
		msg := fmt.Sprintf(format, args...)
		PrintWarning("%s: %v", msg, err)
	}
}

// LogOnError logs an error if err is not nil (non-fatal)
func LogOnError(err error, format string, args ...interface{}) {
	if err != nil {
		msg := fmt.Sprintf(format, args...)
		fmt.Printf("Warning: %s: %v\n", msg, err)
	}
}

// Wrap wraps an error with additional context
func Wrap(err error, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	msg := fmt.Sprintf(format, args...)
	return fmt.Errorf("%s: %w", msg, err)
}

// WrapService wraps an error with service context
func WrapService(service string, err error) error {
	if err == nil {
		return nil
	}
	return NewServiceError(service, err)
}

// WrapContainer wraps an error with container context
func WrapContainer(containerName, action string, err error) error {
	if err == nil {
		return nil
	}
	return NewContainerError(containerName, action, err)
}

// Is checks if an error is of a specific type
func Is(err, target error) bool {
	return errors.Is(err, target)
}

// As attempts to cast an error to a specific type
func As(err error, target interface{}) bool {
	return errors.As(err, target)
}

// IsNotFound checks if an error indicates something was not found
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, ErrNotFound) ||
		strings.Contains(strings.ToLower(err.Error()), "not found") ||
		strings.Contains(strings.ToLower(err.Error()), "no such")
}

// IsAlreadyExists checks if an error indicates something already exists
func IsAlreadyExists(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, ErrAlreadyExists) ||
		strings.Contains(strings.ToLower(err.Error()), "already exists") ||
		strings.Contains(strings.ToLower(err.Error()), "duplicate")
}

// IsPermissionDenied checks if an error is a permission error
func IsPermissionDenied(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, ErrPermissionDenied) ||
		strings.Contains(strings.ToLower(err.Error()), "permission denied") ||
		strings.Contains(strings.ToLower(err.Error()), "access denied")
}

// Must panics if err is not nil. Use sparingly and only for programmer errors.
func Must(err error) {
	if err != nil {
		panic(err)
	}
}

// MustValue returns the value or panics if err is not nil.
func MustValue[T any](value T, err error) T {
	if err != nil {
		panic(err)
	}
	return value
}
