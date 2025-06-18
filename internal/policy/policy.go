package policy

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/1rp-pw/orchestrator/internal/structs"
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

func (s *System) StoreInitialPolicy(p *structs.Policy) (*structs.Policy, error) {
	client, err := s.Config.Database.GetPGXPoolClient(s.Context)
	if err != nil {
		return nil, logs.Errorf("failed to connect to database: %v", err)
	}
	defer client.Close()

	if err := client.QueryRow(s.Context, `SELECT create_policy ($1, $2, $3, $4)`, p.Name, p.DataModel, p.Tests, p.Rule).Scan(&p.BaseID); err != nil {
		return nil, logs.Errorf("failed to store initial structs: %v", err)
	}
	p.Version = "draft"

	return p, nil
}

func (s *System) UpdateDraft(p structs.Policy) error {
	client, err := s.Config.Database.GetPGXPoolClient(s.Context)
	if err != nil {
		return logs.Errorf("failed to connect to database: %v", err)
	}
	defer client.Close()
	if _, err := client.Exec(s.Context, `SELECT update_draft($1, $2, $3, $4, $5)`, p.BaseID, p.DataModel, p.Tests, p.Rule, p.Description); err != nil {
		return logs.Errorf("failed to update draft: %v", err)
	}

	return nil
}

func (s *System) CreateVersion(p structs.Policy) error {
	client, err := s.Config.Database.GetPGXPoolClient(s.Context)
	if err != nil {
		return logs.Errorf("failed to connect to database: %v", err)
	}
	defer client.Close()

	if _, err := client.Exec(s.Context, `SELECT publish_draft_as_version($1, $2, $3)`, p.BaseID, fmt.Sprintf("v%s", p.Version), p.Description); err != nil {
		return logs.Errorf("failed to create version: %v", err)
	}

	return nil
}

func (s *System) LoadPolicy(policyId string) (structs.Policy, error) {
	type dataStruct struct {
		Name      sql.NullString
		BaseID    sql.NullString
		Version   sql.NullString
		DataModel sql.NullString
		Tests     sql.NullString
		Rule      sql.NullString
		Status    sql.NullString
	}
	d := dataStruct{}

	client, err := s.Config.Database.GetPGXPoolClient(s.Context)
	if err != nil {
		return structs.Policy{}, logs.Errorf("failed to connect to database: %v", err)
	}
	defer client.Close()
	if err := client.QueryRow(s.Context, `
		SELECT 
		    name, 
		    base_policy_id, 
		    version, 
		    data_model, 
		    tests, 
		    rule, 
		    status 
		FROM public.policies 
		WHERE policy_id = $1`, policyId,
	).Scan(
		&d.Name,
		&d.BaseID,
		&d.Version,
		&d.DataModel,
		&d.Tests,
		&d.Rule,
		&d.Status,
	); err != nil {
		return structs.Policy{}, logs.Errorf("failed to load structs: %v", err)
	}
	p := structs.Policy{
		PolicyID:  policyId,
		BaseID:    d.BaseID.String,
		Name:      d.Name.String,
		Version:   d.Version.String,
		DataModel: d.DataModel.String,
		Tests:     d.Tests.String,
		Rule:      d.Rule.String,
		Status:    d.Status.String,
	}

	return p, nil
}

func (s *System) DraftFromVersion(policyId string) (structs.Policy, error) {
	p := structs.Policy{}

	client, err := s.Config.Database.GetPGXPoolClient(s.Context)
	if err != nil {
		return p, logs.Errorf("failed to connect to database: %v", err)
	}
	defer client.Close()

	var basePolicyId sql.NullString
	var version sql.NullString

	if err := client.QueryRow(s.Context, `SELECT base_policy_id, version FROM policies WHERE policy_id = $1`, policyId).Scan(&basePolicyId, &version); err != nil {
		return p, logs.Errorf("failed to load structs: %v", err)
	}

	var newPolicyId sql.NullString

	if basePolicyId.Valid {
		if err := client.QueryRow(s.Context, `SELECT create_draft_from_version($1, $2)`, basePolicyId.String, version.String).Scan(&newPolicyId); err != nil {
			return p, logs.Errorf("failed to load structs: %v", err)
		}
	}

	return s.LoadPolicy(newPolicyId.String)
}

func (s *System) AllPolicies() ([]structs.Policy, error) {
	var pp []structs.Policy

	client, err := s.Config.Database.GetPGXPoolClient(s.Context)
	if err != nil {
		return pp, logs.Errorf("failed to connect to database: %v", err)
	}
	defer client.Close()
	rows, err := client.Query(s.Context, `
		SELECT 
		    base_policy_id, 
		    current_name, 
		    version_count, 
		    draft_id, 
		    first_created_date, 
		    latest_activity_date, 
		    latest_version_date, 
		    has_draft 
		FROM public.policy_summary`)
	if err != nil {
		return pp, logs.Errorf("failed to load policies: %v", err)
	}
	defer rows.Close()

	type dataStruct struct {
		ID              sql.NullString
		Name            sql.NullString
		Versions        sql.NullInt32
		DraftID         sql.NullString
		CreatedAt       sql.NullTime
		UpdatedAt       sql.NullTime
		LastPublishedAt sql.NullTime
		HasDraft        bool
	}

	for rows.Next() {
		d := dataStruct{}
		if err := rows.Scan(
			&d.ID,
			&d.Name,
			&d.Versions,
			&d.DraftID,
			&d.CreatedAt,
			&d.UpdatedAt,
			&d.LastPublishedAt,
			&d.HasDraft,
		); err != nil {
			return pp, logs.Errorf("failed to load policies: %v", err)
		}

		p := structs.Policy{
			BaseID:    d.ID.String,
			Name:      d.Name.String,
			HasDraft:  d.HasDraft,
			UpdatedAt: d.UpdatedAt.Time,
			CreatedAt: d.CreatedAt.Time,
		}
		if d.LastPublishedAt.Valid {
			p.LastPublishedAt = d.LastPublishedAt.Time
		}
		pp = append(pp, p)
	}

	return pp, nil
}

func (s *System) GetPolicyVersions(basePolicyId string) ([]structs.Policy, error) {
	var pp []structs.Policy

	client, err := s.Config.Database.GetPGXPoolClient(s.Context)
	if err != nil {
		return pp, logs.Errorf("failed to connect to database: %v", err)
	}
	defer client.Close()
	rows, err := client.Query(s.Context, `
		SELECT 
		    policy_id, 
		    name, 
		    version, 
		    rule, 
		    data_model, 
		    description, 
		    status, 
		    created_at, 
		    updated_at 
		FROM policies 
		WHERE base_policy_id = $1
		ORDER BY 
		    CASE WHEN status = 'draft' THEN 0 ELSE 1 END,
		    CASE WHEN version IS NULL THEN '' ELSE version END`, basePolicyId)
	if err != nil {
		return pp, logs.Errorf("failed to load policies: %v", err)
	}
	defer rows.Close()

	type dataStruct struct {
		ID          sql.NullString
		Name        sql.NullString
		Version     sql.NullString
		Rule        sql.NullString
		DataModel   sql.NullString
		Description sql.NullString
		Status      sql.NullString
		CreatedAt   sql.NullTime
		UpdatedAt   sql.NullTime
	}

	for rows.Next() {
		d := dataStruct{}
		if err := rows.Scan(
			&d.ID,
			&d.Name,
			&d.Version,
			&d.Rule,
			&d.DataModel,
			&d.Description,
			&d.Status,
			&d.CreatedAt,
			&d.UpdatedAt,
		); err != nil {
			return pp, logs.Errorf("failed to load policies: %v", err)
		}

		p := structs.Policy{
			BaseID:      basePolicyId,
			PolicyID:    d.ID.String,
			Name:        d.Name.String,
			Description: d.Description.String,
			DataModel:   d.DataModel.String,
			CreatedAt:   d.CreatedAt.Time,
			UpdatedAt:   d.UpdatedAt.Time,
			Status:      d.Status.String,
			Rule:        d.Rule.String,
		}

		if d.Status.String == "draft" {
			p.IsDraft = true
		}

		if d.Version.String != "" {
			p.Version = d.Version.String
		}

		pp = append(pp, p)
	}

	return pp, nil
}
