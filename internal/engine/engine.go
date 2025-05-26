package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	policymodel "github.com/1rp-pw/orchestrator/internal/policy"
	"github.com/bugfixes/go-bugfixes/logs"
	ConfigBuilder "github.com/keloran/go-config"
	"net/http"
)

type engineResponse struct {
	Result bool        `json:"result"`
	Trace  interface{} `json:"trace"`
	Text   []string    `json:"text"`
	Data   interface{} `json:"data"`
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

func (s *System) runPolicy(policy policymodel.Policy) (*engineResponse, error) {
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

	er := engineResponse{}
	//var er interface{}
	if err := json.NewDecoder(resp.Body).Decode(&er); err != nil {
		_ = logs.Errorf("error decoding response: %v", err)
	}

	return &er, nil
}
