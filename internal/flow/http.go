package flow

import (
	"encoding/json"
	"github.com/1rp-pw/orchestrator/internal/errors"
	"github.com/1rp-pw/orchestrator/internal/structs"
	"github.com/bugfixes/go-bugfixes/logs"
	"net/http"
	"time"

	"gopkg.in/yaml.v3"
)

func (s *System) TestFlow(w http.ResponseWriter, r *http.Request) {
	var t structs.FlowTestRequest
	defer func() {
		if err := r.Body.Close(); err != nil {
			_ = logs.Errorf("error closing body: %v", err)
		}
	}()

	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		errors.WriteHTTPError(w, errors.NewValidationError("body", "invalid JSON format"))
		return
	}

	if err := yaml.Unmarshal([]byte(t.FlowYAML), &t.Flow); err != nil {
		errors.WriteHTTPError(w, errors.NewValidationError("flow", "invalid YAML flow format"))
		return
	}

	s.SetContext(r.Context())
	fr, err := s.RunTestFlow(t)
	if err != nil {
		errors.WriteHTTPError(w, err)
		return
	}

	if err := json.NewEncoder(w).Encode(fr); err != nil {
		errors.WriteHTTPError(w, errors.NewInternalError("failed to encode response"))
	}
}

func (s *System) GetAllFlows(w http.ResponseWriter, r *http.Request) {
	s.SetContext(r.Context())

	f, err := s.AllFlows()
	if err != nil {
		errors.WriteHTTPError(w, err)
		return
	}

	if err := json.NewEncoder(w).Encode(f); err != nil {
		errors.WriteHTTPError(w, errors.NewInternalError("failed to encode response"))
	}
}

func (s *System) CreateFlow(w http.ResponseWriter, r *http.Request) {
	s.SetContext(r.Context())

	var f structs.FlowRequest
	if err := json.NewDecoder(r.Body).Decode(&f); err != nil {
		errors.WriteHTTPError(w, errors.NewValidationError("body", "invalid JSON format"))
		return
	}

	if err := yaml.Unmarshal([]byte(f.FlowYAML), &f.Flow); err != nil {
		errors.WriteHTTPError(w, errors.NewValidationError("flow", "invalid YAML flow format"))
	}

	sf := structs.StoredFlow{
		CreatedAt:  time.Now(),
		Version:    "draft",
		Name:       f.Name,
		Nodes:      f.Nodes,
		Edges:      f.Edges,
		FlatYAML:   f.FlowYAML,
		FlowConfig: f.Flow,
		Tests:      f.Tests,
		BaseID:     f.BaseID,
		FlowID:     f.ID,
	}
	rf, err := s.StoreInitialFlow(&sf)
	if err != nil {
		errors.WriteHTTPError(w, err)
		return
	}

	if err := json.NewEncoder(w).Encode(rf); err != nil {
		errors.WriteHTTPError(w, errors.NewInternalError("failed to encode response"))
	}
}

func (s *System) ListFlowVersions(w http.ResponseWriter, r *http.Request) {
	s.SetContext(r.Context())
	flowId := r.PathValue("flowId")

	f, err := s.GetFlowVersions(flowId)
	if err != nil {
		errors.WriteHTTPError(w, err)
		return
	}

	if err := json.NewEncoder(w).Encode(f); err != nil {
		errors.WriteHTTPError(w, errors.NewInternalError("failed to encode response"))
	}
}

func (s *System) RunFlow(w http.ResponseWriter, r *http.Request) {
	s.SetContext(r.Context())
	flowId := r.PathValue("flowId")

	f, err := s.GetStoredFlow(flowId)
	if err != nil {
		errors.WriteHTTPError(w, err)
		return
	}

	var flowRequest interface{}
	if err := json.NewDecoder(r.Body).Decode(&flowRequest); err != nil {
		errors.WriteHTTPError(w, errors.NewValidationError("body", "invalid JSON format"))
		return
	}

	flowResult, err := s.RunFlowInternal(*f, flowRequest)
	if err != nil {
		errors.WriteHTTPError(w, err)
	}
	if err := json.NewEncoder(w).Encode(flowResult); err != nil {
		errors.WriteHTTPError(w, errors.NewInternalError("failed to encode response"))
	}
}

func (s *System) GetFlow(w http.ResponseWriter, r *http.Request) {
	s.SetContext(r.Context())
	flowId := r.PathValue("flowId")

	f, err := s.GetFullFlow(flowId)
	if err != nil {
		errors.WriteHTTPError(w, err)
		return
	}

	if err := json.NewEncoder(w).Encode(f); err != nil {
		errors.WriteHTTPError(w, errors.NewInternalError("failed to encode response"))
	}
}

func (s *System) UpdateFlow(w http.ResponseWriter, r *http.Request) {
	s.SetContext(r.Context())

	var f structs.FlowRequest
	if err := json.NewDecoder(r.Body).Decode(&f); err != nil {
		errors.WriteHTTPError(w, errors.NewValidationError("body", "invalid JSON format"))
		return
	}

	if err := yaml.Unmarshal([]byte(f.FlowYAML), &f.Flow); err != nil {
		errors.WriteHTTPError(w, errors.NewValidationError("flow", "invalid YAML flow format"))
	}

	sf := structs.StoredFlow{
		UpdatedAt:  time.Now(),
		Version:    "draft",
		Name:       f.Name,
		Nodes:      f.Nodes,
		Edges:      f.Edges,
		FlatYAML:   f.FlowYAML,
		FlowConfig: f.Flow,
		Tests:      f.Tests,
		BaseID:     f.BaseID,
		FlowID:     f.ID,
	}
	rf, err := s.StoreFlow(&sf)
	if err != nil {
		errors.WriteHTTPError(w, err)
		return
	}

	if err := json.NewEncoder(w).Encode(rf); err != nil {
		errors.WriteHTTPError(w, errors.NewInternalError("failed to encode response"))
	}
}
