package policy

import (
	"context"
	"fmt"
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

func (s *System) UpdateDraft(p policy.Policy) error {
	client, err := s.Config.Database.GetPGXPoolClient(s.Context)
	if err != nil {
		return logs.Errorf("failed to connect to database: %v", err)
	}
	defer client.Close()
	if _, err := client.Exec(s.Context, `
		UPDATE public.policy_versions SET 
			data_model = $1, 
			tests = $2, 
			policy_text = $3, 
			updated_at = now()
		WHERE id = $4`, p.DataModel, p.Tests, p.Rule, p.ID); err != nil {
		return logs.Errorf("failed to update draft: %v", err)
	}

	return nil
}

func (s *System) CreateVersion(p policy.Policy) error {
	client, err := s.Config.Database.GetPGXPoolClient(s.Context)
	if err != nil {
		return logs.Errorf("failed to connect to database: %v", err)
	}
	defer client.Close()

	if _, err := client.Exec(s.Context, `SELECT publish_draft($1, $2)`, p.ID, fmt.Sprintf("v%s", p.Version)); err != nil {
		return logs.Errorf("failed to create version: %v", err)
	}

	return nil
}

func (s *System) LoadPolicy(policyId string) (policy.Policy, error) {
	p := policy.Policy{}

	client, err := s.Config.Database.GetPGXPoolClient(s.Context)
	if err != nil {
		return p, logs.Errorf("failed to connect to database: %v", err)
	}
	defer client.Close()
	if err := client.QueryRow(s.Context, "SELECT policy_name, version, data_model, tests, policy_text FROM public.policy_versions WHERE id = $1", policyId).Scan(&p.Name, &p.Version, &p.DataModel, &p.Tests, &p.Rule); err != nil {
		return p, logs.Errorf("failed to load policy: %v", err)
	}
	p.ID = policyId

	return p, nil
}

func (s *System) AllPolicies() ([]policy.Policy, error) {
	var pp []policy.Policy

	client, err := s.Config.Database.GetPGXPoolClient(s.Context)
	if err != nil {
		return pp, logs.Errorf("failed to connect to database: %v", err)
	}
	defer client.Close()
	rows, err := client.Query(s.Context, "SELECT id, policy_name, version, created_at, updated_at FROM public.policy_versions GROUP BY id")
	if err != nil {
		return pp, logs.Errorf("failed to load policies: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		p := policy.Policy{}
		if err := rows.Scan(
			&p.ID,
			&p.Name,
			&p.Version,
			&p.CreatedAt,
			&p.UpdatedAt,
		); err != nil {
			return pp, logs.Errorf("failed to load policies: %v", err)
		}
		pp = append(pp, p)
	}

	return pp, nil
}

func (s *System) GetPolicyVersions(policyId string) ([]policy.Policy, error) {
	var pp []policy.Policy

	client, err := s.Config.Database.GetPGXPoolClient(s.Context)
	if err != nil {
		return pp, logs.Errorf("failed to connect to database: %v", err)
	}
	defer client.Close()
	rows, err := client.Query(s.Context, "SELECT id, policy_name, data_model, tests, policy_text, version, created_at, updated_at, is_immutable FROM public.policy_versions WHERE id = $1", policyId)
	if err != nil {
		return pp, logs.Errorf("failed to load policies: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		draft := false

		p := policy.Policy{}
		if err := rows.Scan(
			&p.ID,
			&p.Name,
			&p.DataModel,
			&p.Tests,
			&p.Rule,
			&p.Version,
			&p.CreatedAt,
			&p.UpdatedAt,
			&draft,
		); err != nil {
			return pp, logs.Errorf("failed to load policies: %v", err)
		}
		p.IsDraft = draft

		pp = append(pp, p)
	}

	return pp, nil
}
