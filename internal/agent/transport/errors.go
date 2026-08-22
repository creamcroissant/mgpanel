// Package transport provides gRPC transport utilities for the agent.
package transport

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ErrorCategory defines error classification for retry decisions.
type ErrorCategory int

const (
	// CategoryRetryable indicates a transient error that can be retried.
	CategoryRetryable ErrorCategory = iota
	// CategoryPermanent indicates a permanent error that should not be retried.
	CategoryPermanent
	// CategoryFatal indicates a fatal error requiring manual intervention.
	CategoryFatal
)

// String returns the string representation of the error category.
func (c ErrorCategory) String() string {
	switch c {
	case CategoryRetryable:
		return "retryable"
	case CategoryPermanent:
		return "permanent"
	case CategoryFatal:
		return "fatal"
	default:
		return "unknown"
	}
}

// ClassifyError categorizes a gRPC error based on its status code.
func ClassifyError(err error) ErrorCategory {
	if err == nil {
		return CategoryRetryable
	}

	st, ok := status.FromError(err)
	if !ok {
		// Non-gRPC error, assume retryable (e.g., network issues)
		return CategoryRetryable
	}

	switch st.Code() {
	// Retryable errors - transient failures
	case codes.Unavailable, // Service temporarily unavailable
		codes.DeadlineExceeded,  // Request timeout
		codes.ResourceExhausted, // Rate limited or quota exceeded
		codes.Aborted:           // Operation aborted, can retry
		return CategoryRetryable

	// Permanent errors - no point in retrying
	case codes.Unauthenticated,    // Authentication failed
		codes.PermissionDenied,    // Access denied
		codes.InvalidArgument,     // Bad request
		codes.NotFound,            // Resource not found
		codes.AlreadyExists,       // Resource already exists
		codes.FailedPrecondition,  // Precondition check failed
		codes.OutOfRange,          // Value out of range
		codes.Unimplemented:       // Method not implemented
		return CategoryPermanent

	// Fatal errors - require manual intervention
	case codes.DataLoss, // Unrecoverable data loss
		codes.Internal: // Internal server error
		return CategoryFatal

	default:
		// Unknown codes default to retryable
		return CategoryRetryable
	}
}

// IsRetryable returns true if the error is transient and can be retried.
func IsRetryable(err error) bool {
	return ClassifyError(err) == CategoryRetryable
}

// IsAuthError returns true if the error is an authentication failure.
func IsAuthError(err error) bool {
	st, ok := status.FromError(err)
	if !ok {
		return false
	}
	return st.Code() == codes.Unauthenticated
}

// IsPermissionError returns true if the error is a permission denial.
func IsPermissionError(err error) bool {
	st, ok := status.FromError(err)
	if !ok {
		return false
	}
	return st.Code() == codes.PermissionDenied
}

// GetGRPCCode extracts the gRPC status code from an error.
// Returns codes.Unknown if the error is not a gRPC error.
func GetGRPCCode(err error) codes.Code {
	if err == nil {
		return codes.OK
	}
	st, ok := status.FromError(err)
	if !ok {
		return codes.Unknown
	}
	return st.Code()
}
