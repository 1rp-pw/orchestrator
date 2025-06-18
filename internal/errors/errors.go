package errors

import (
	"errors"
	"fmt"
)

// Sentinel errors for common cases
var (
	// ErrMissingPolicyID is returned when a policy node doesn't have a policyId
	ErrMissingPolicyID = errors.New("policyId is required for policy nodes")

	// ErrInvalidFlow is returned when flow configuration is invalid
	ErrInvalidFlow   = errors.New("invalid flow configuration")
	ErrInvalidPolicy = errors.New("invalid policy configuration")

	// ErrPolicyNotFound is returned when a policy cannot be found
	ErrPolicyNotFound = errors.New("policy not found")

	// ErrFlowNotFound is returned when a flow cannot be found
	ErrFlowNotFound = errors.New("flow not found")
)

// ValidationError represents a validation error with field information
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("validation error on field '%s': %s", e.Field, e.Message)
	}
	return fmt.Sprintf("validation error: %s", e.Message)
}

// NewValidationError creates a new validation error
func NewValidationError(field, message string) error {
	return &ValidationError{
		Field:   field,
		Message: message,
	}
}

// FlowError represents errors specific to flow operations
type FlowError struct {
	FlowID  string
	NodeID  string
	Message string
	Err     error
}

func (e *FlowError) Error() string {
	if e.NodeID != "" {
		return fmt.Sprintf("flow error in flow %s at node %s: %s", e.FlowID, e.NodeID, e.Message)
	}
	if e.FlowID != "" {
		return fmt.Sprintf("flow error in flow %s: %s", e.FlowID, e.Message)
	}
	return fmt.Sprintf("flow error: %s", e.Message)
}

// Unwrap allows errors.Is and errors.As to work with wrapped errors
func (e *FlowError) Unwrap() error {
	return e.Err
}

// NewFlowError creates a new flow error
func NewFlowError(flowID, nodeID, message string) error {
	return &FlowError{
		FlowID:  flowID,
		NodeID:  nodeID,
		Message: message,
	}
}

// WrapFlowError wraps an existing error with flow context
func WrapFlowError(err error, flowID, nodeID string) error {
	return &FlowError{
		FlowID:  flowID,
		NodeID:  nodeID,
		Message: err.Error(),
		Err:     err,
	}
}

// PolicyError represents errors specific to policy operations
type PolicyError struct {
	PolicyID string
	Message  string
	Err      error
}

func (e *PolicyError) Error() string {
	if e.PolicyID != "" {
		return fmt.Sprintf("policy error for policy %s: %s", e.PolicyID, e.Message)
	}
	return fmt.Sprintf("policy error: %s", e.Message)
}

// Unwrap allows errors.Is and errors.As to work with wrapped errors
func (e *PolicyError) Unwrap() error {
	return e.Err
}

// NewPolicyError creates a new policy error
func NewPolicyError(policyID, message string) error {
	return &PolicyError{
		PolicyID: policyID,
		Message:  message,
	}
}

// WrapPolicyError wraps an existing error with policy context
func WrapPolicyError(err error, policyID string) error {
	return &PolicyError{
		PolicyID: policyID,
		Message:  err.Error(),
		Err:      err,
	}
}

// Helper functions to check error types

// IsMissingPolicyID checks if the error is due to missing policy ID
func IsMissingPolicyID(err error) bool {
	return errors.Is(err, ErrMissingPolicyID)
}

// IsValidationError checks if the error is a validation error
func IsValidationError(err error) bool {
	var validationErr *ValidationError
	return errors.As(err, &validationErr)
}

// IsFlowError checks if the error is a flow error
func IsFlowError(err error) bool {
	var flowErr *FlowError
	return errors.As(err, &flowErr)
}

// IsPolicyError checks if the error is a policy error
func IsPolicyError(err error) bool {
	var policyErr *PolicyError
	return errors.As(err, &policyErr)
}

// NewInternalError creates a new internal server error
func NewInternalError(message string) error {
	return fmt.Errorf("internal server error: %s", message)
}
