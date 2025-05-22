package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	policymodel "github.com/1rp-pw/orchestrator/internal/policy"
	"github.com/1rp-pw/orchestrator/internal/storage"
	"github.com/bugfixes/go-bugfixes/logs"
	ConfigBuilder "github.com/keloran/go-config"
	"io"
	"net/http"
)

type System struct {
	Config  *ConfigBuilder.Config
	Context context.Context
}

func NewSystem(cfg *ConfigBuilder.Config) *System {
	return &System{
		Config:  cfg,
		Context: context.Background(),
	}
}

func (s *System) SetContext(ctx context.Context) *System {
	s.Context = ctx
	return s
}

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

	policyResult, err := s.RunPolicyInEngine(policy)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if policyResult == nil {
		_ = logs.Error("response is nil")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(policyResult); err != nil {
		_ = logs.Errorf("failed to write response: %v", err)
	}
}

func (s *System) RunPolicy(w http.ResponseWriter, r *http.Request) {
	s.Context = r.Context()
	policyId := r.PathValue("policyId")

	// get the policy from storage
	st := storage.NewSystem(s.Config).SetContext(s.Context)
	p, err := st.GetLatestPolicyFromStorage(policyId)
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

	policyResult, err := s.RunPolicyInEngine(storedPolicy)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if policyResult == nil {
		_ = logs.Error("response is nil")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(policyResult); err != nil {
		_ = logs.Errorf("failed to write response: %v", err)
	}
}

func (s *System) RunPolicyInEngine(policy policymodel.Policy) ([]byte, error) {
	data, err := json.Marshal(policy)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(s.Context, "POST", fmt.Sprintf("%s", s.Config.ProjectProperties["engine_address"]), bytes.NewBuffer(data))
	if err != nil {
		_ = logs.Errorf("Error building http request: %s", err)
		return nil, nil
	}
	if req == nil {
		_ = logs.Error("request is nil")
		return nil, nil
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Policy Orchestrator")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, nil
	}
	if resp == nil {
		_ = logs.Error("response is nil")
		return nil, nil
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			_ = logs.Errorf("error closing body: %v", err)
		}
	}()

	data, err = io.ReadAll(resp.Body)
	if err != nil {
		_ = logs.Errorf("Error reading response body: %s", err)
		return nil, nil
	}

	return data, nil
}
