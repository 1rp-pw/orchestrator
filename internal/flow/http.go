package flow

import (
	"encoding/json"
	"github.com/1rp-pw/orchestrator/internal/errors"
	"github.com/1rp-pw/orchestrator/internal/structs"
	"github.com/bugfixes/go-bugfixes/logs"
	"net/http"

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

	if err := yaml.Unmarshal([]byte(t.JSONFlow), &t.Flow); err != nil {
		errors.WriteHTTPError(w, errors.NewValidationError("jsonFlow", "invalid YAML flow format"))
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

	if err := yaml.Unmarshal([]byte(f.JSONFlow), &f.Flow); err != nil {
		errors.WriteHTTPError(w, errors.NewValidationError("jsonFlow", "invalid YAML flow format"))
	}

	logs.Debugf("flow: %+v", f)
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
