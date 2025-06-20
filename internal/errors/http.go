package errors

import (
	"encoding/json"
	"errors"
	"net/http"
)

// HTTPError represents the structure of error responses sent to clients
type HTTPError struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

// WriteHTTPError writes an error response to the HTTP response writer
func WriteHTTPError(w http.ResponseWriter, err error) {
	statusCode := http.StatusInternalServerError
	httpErr := HTTPError{
		Code:    "INTERNAL_ERROR",
		Message: "An internal error occurred",
	}

	// Check for validation errors
	var validationErr *ValidationError
	if errors.As(err, &validationErr) {
		statusCode = http.StatusBadRequest
		httpErr.Code = "VALIDATION_ERROR"
		httpErr.Message = validationErr.Error()
		if validationErr.Field != "" {
			httpErr.Details = map[string]string{"field": validationErr.Field}
		}
	}

	// Check for flow errors
	var flowErr *FlowError
	if errors.As(err, &flowErr) {
		// Determine status code based on wrapped error
		if errors.Is(flowErr.Err, ErrMissingPolicyID) {
			statusCode = http.StatusBadRequest
			httpErr.Code = "MISSING_POLICY_ID"
		} else if errors.Is(flowErr.Err, ErrFlowNotFound) {
			statusCode = http.StatusNotFound
			httpErr.Code = "FLOW_NOT_FOUND"
		} else {
			statusCode = http.StatusBadRequest
			httpErr.Code = "FLOW_ERROR"
		}
		httpErr.Message = flowErr.Error()
		details := make(map[string]string)
		if flowErr.FlowID != "" {
			details["flowId"] = flowErr.FlowID
		}
		if flowErr.NodeID != "" {
			details["nodeId"] = flowErr.NodeID
		}
		if len(details) > 0 {
			httpErr.Details = details
		}
	}

	// Check for policy errors
	var policyErr *PolicyError
	if errors.As(err, &policyErr) {
		if errors.Is(policyErr.Err, ErrPolicyNotFound) {
			statusCode = http.StatusNotFound
			httpErr.Code = "POLICY_NOT_FOUND"
		} else {
			statusCode = http.StatusBadRequest
			httpErr.Code = "POLICY_ERROR"
		}
		httpErr.Message = policyErr.Error()
		if policyErr.PolicyID != "" {
			httpErr.Details = map[string]string{"policyId": policyErr.PolicyID}
		}
	}

	// Check for sentinel errors
	switch {
	case errors.Is(err, ErrMissingPolicyID):
		statusCode = http.StatusBadRequest
		httpErr.Code = "MISSING_POLICY_ID"
		httpErr.Message = err.Error()
	case errors.Is(err, ErrInvalidFlow):
		statusCode = http.StatusBadRequest
		httpErr.Code = "INVALID_FLOW"
		httpErr.Message = err.Error()
	case errors.Is(err, ErrPolicyNotFound):
		statusCode = http.StatusNotFound
		httpErr.Code = "POLICY_NOT_FOUND"
		httpErr.Message = err.Error()
	case errors.Is(err, ErrFlowNotFound):
		statusCode = http.StatusNotFound
		httpErr.Code = "FLOW_NOT_FOUND"
		httpErr.Message = err.Error()
	}

	// Write the response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": httpErr,
	})
}

// WriteHTTPSuccess writes a success response to the HTTP response writer
func WriteHTTPSuccess(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if data != nil {
		json.NewEncoder(w).Encode(data)
	}
}

