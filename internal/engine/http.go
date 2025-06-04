package engine

import (
	"encoding/json"
	policymodel "github.com/1rp-pw/orchestrator/internal/policy"
	"github.com/1rp-pw/orchestrator/internal/storage/policy"
	"github.com/bugfixes/go-bugfixes/logs"
	"io"
	"net/http"
)

func (s *System) Run(w http.ResponseWriter, r *http.Request) {
	s.Context = r.Context()

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	defer func() {
		if err := r.Body.Close(); err != nil {
			_ = logs.Errorf("error closing body: %v", err)
		}
	}()

	var policy policymodel.Policy
	if err := json.Unmarshal(bodyBytes, &policy); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	pr, err := s.runPolicy(policy)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(pr); err != nil {
		w.WriteHeader(http.StatusBadRequest)
	}
}

func (s *System) RunPolicy(w http.ResponseWriter, r *http.Request) {
	s.Context = r.Context()
	policyId := r.PathValue("policyId")

	// get the policy from storage
	st := policy.NewSystem(s.Config).SetContext(s.Context)
	p, err := st.LoadPolicy(policyId)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	storedPolicy := policymodel.Policy{}
	if err := json.Unmarshal(p.([]byte), &storedPolicy); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	var policyData policymodel.Policy
	if err := json.Unmarshal(bodyBytes, &policyData); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	storedPolicy.Data = policyData.Data

	pr, err := s.runPolicy(storedPolicy)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(pr); err != nil {
		w.WriteHeader(http.StatusBadRequest)
	}
}
