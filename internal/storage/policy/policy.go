package policy

import (
	"context"
	"github.com/1rp-pw/orchestrator/internal/policy"
	"github.com/bugfixes/go-bugfixes/logs"
	ConfigBuilder "github.com/keloran/go-config"
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

func (s *System) StoreInitialPolicy(p *policy.Policy) (*policy.Policy, error) {
	client, err := s.Config.Database.GetPGXPoolClient(s.Context)
	if err != nil {
		return nil, logs.Errorf("failed to connect to database: %v", err)
	}
	defer client.Close()

	if err := client.QueryRow(s.Context, `
		INSERT INTO 
		    public.policy_versions (
				policy_name, 
				version, 
				data_model, 
				tests, 
				policy_text,
		        user_id
		    ) VALUES ($1, $2, $3, $4, $5, 'f08c6d21-f0d2-42fa-ad4d-f3b45dc81998') RETURNING id`, p.Name, p.Version, p.DataModel, p.Tests, p.Rule).Scan(&p.ID); err != nil {
		return nil, logs.Errorf("failed to store initial policy: %v", err)
	}

	return p, nil
}

func (s *System) LoadPolicy(policyId string) (interface{}, error) {
	return nil, nil
}
