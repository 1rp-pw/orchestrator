package flow

import (
	"context"
	"fmt"
	"github.com/1rp-pw/orchestrator/internal/structs"
	ConfigBuilder "github.com/keloran/go-config"
)

type System struct {
	Config  *ConfigBuilder.Config
	Context context.Context
}

func NewSystem(cfg *ConfigBuilder.Config) *System {
	return &System{
		Config: cfg,
	}
}

func (s *System) SetContext(ctx context.Context) *System {
	s.Context = ctx
	return s
}

func (s *System) RunTestFlow(f structs.FlowRequest) (structs.FlowResponse, error) {
	fr := structs.FlowResponse{}

	flow := f.Flow
	data := f.Data

	fmt.Sprintf("%+v", flow)
	fmt.Sprintf("%+v", data)

	return fr, nil
}
