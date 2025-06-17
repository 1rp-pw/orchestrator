package flow

import (
	"encoding/json"
	"github.com/1rp-pw/orchestrator/internal/structs"
	"github.com/bugfixes/go-bugfixes/logs"
	"net/http"

	"gopkg.in/yaml.v3"
)

func (s *System) TestFlow(w http.ResponseWriter, r *http.Request) {
	var t structs.FlowRequest
	defer func() {
		if err := r.Body.Close(); err != nil {
			_ = logs.Errorf("error closing body: %v", err)
		}
	}()

	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := yaml.Unmarshal([]byte(t.JSONFlow), &t.Flow); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	s.SetContext(r.Context())
	fr, err := s.RunTestFlow(t)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = logs.Errorf("error running test flow: %v", err)
		return
	}

	if err := json.NewEncoder(w).Encode(fr); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}
