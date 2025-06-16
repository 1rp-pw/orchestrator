package policy

import (
	"encoding/json"
	"github.com/1rp-pw/orchestrator/internal/structs"
	"github.com/bugfixes/go-bugfixes/logs"
	"net/http"
	"time"
)

func (s *System) CreatePolicy(w http.ResponseWriter, r *http.Request) {
	var i structs.Policy
	if err := json.NewDecoder(r.Body).Decode(&i); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	i.CreatedAt = time.Now()
	i.Version = "draft"

	p, err := s.StoreInitialPolicy(&i)
	if err != nil {
		_ = logs.Errorf("failed to store initial structs: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		if err := json.NewEncoder(w).Encode(err); err != nil {
			_ = logs.Errorf("failed to encode structs: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		return
	}

	if err := json.NewEncoder(w).Encode(p); err != nil {
		_ = logs.Errorf("failed to encode structs: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (s *System) DeletePolicy(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
}

func (s *System) UpdatePolicy(w http.ResponseWriter, r *http.Request) {
	var i structs.Policy
	if err := json.NewDecoder(r.Body).Decode(&i); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if i.Status == "draft" {
		if err := s.UpdateDraft(i); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		return
	}

	if err := s.CreateVersion(i); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
	w.WriteHeader(http.StatusCreated)
}

func (s *System) GetPolicy(w http.ResponseWriter, r *http.Request) {
	policyId := r.PathValue("policyId")
	p, err := s.LoadPolicy(policyId)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if err := json.NewEncoder(w).Encode(p); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (s *System) CreateDraftFromVersion(w http.ResponseWriter, r *http.Request) {
	policyId := r.PathValue("policyId")
	p, err := s.DraftFromVersion(policyId)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if err := json.NewEncoder(w).Encode(p); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (s *System) GetPolicyVersion(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
}

func (s *System) ListPolicyVersions(w http.ResponseWriter, r *http.Request) {
	policyId := r.PathValue("policyId")

	p, err := s.GetPolicyVersions(policyId)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if err := json.NewEncoder(w).Encode(p); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (s *System) GetAllPolicies(w http.ResponseWriter, r *http.Request) {
	p, err := s.AllPolicies()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(p); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}
