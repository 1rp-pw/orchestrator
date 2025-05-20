package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/bugfixes/go-bugfixes/logs"
	ConfigBuilder "github.com/keloran/go-config"
	"io"
	"net/http"
)

type Policy struct {
	Rule string      `json:"rule"`
	Data interface{} `json:"data"`
}

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
	defer r.Body.Close()

	// Optionally, you can validate the JSON here if needed
	var policy Policy
	if err := json.Unmarshal(bodyBytes, &policy); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	req, err := http.NewRequestWithContext(s.Context, "POST", fmt.Sprintf("%s", s.Config.ProjectProperties["engine_address"]), bytes.NewBuffer(bodyBytes))
	if err != nil {
		_ = logs.Errorf("Error building http request: %s", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if req == nil {
		_ = logs.Error("request is nil")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Policy Orchestrator")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
	}
	if resp == nil {
		_ = logs.Error("response is nil")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	defer resp.Body.Close()
	w.WriteHeader(resp.StatusCode)
	w.Header().Set("Content-Type", "application/json")
	if _, err := io.Copy(w, resp.Body); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = logs.Errorf("Error copying response body: %s", err)
	}
}

func (s *System) RunPolicy(w http.ResponseWriter, r *http.Request) {
	s.Context = r.Context()
	policyID := r.PathValue("policyID")
	logs.Infof("policy: %s", policyID)

	w.WriteHeader(http.StatusNotImplemented)
}
