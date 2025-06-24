package policy

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/1rp-pw/orchestrator/internal/structs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	ConfigBuilder "github.com/keloran/go-config"
)

func setupTestDatabase(t *testing.T) (*postgres.PostgresContainer, *ConfigBuilder.Config) {
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
	)
	require.NoError(t, err)

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	cfg := ConfigBuilder.NewConfigNoVault()
	if err := cfg.Build(ConfigBuilder.Postgres); err != nil {
		require.NoError(t, err)
	}

	// Parse the connection string to set up the database configuration
	if err := cfg.Database.ParseConnectionString(connStr); err != nil {
		require.NoError(t, err)
	}
	cfg.Database.Details.ConnectionTimeout = 30 * time.Second

	// Wait a bit to ensure database is fully ready
	time.Sleep(2 * time.Second)

	client, err := cfg.Database.GetPGXPoolClient(ctx)
	require.NoError(t, err)
	defer client.Close()

	schemaSQL, err := os.ReadFile("../../sql/policy.sql")
	require.NoError(t, err)
	_, err = client.Exec(ctx, string(schemaSQL))
	if err != nil {
		t.Fatalf("Failed to execute policy schema SQL: %v", err)
	}

	return pgContainer, cfg
}

func TestSystem_StoreInitialPolicy(t *testing.T) {
	pgContainer, cfg := setupTestDatabase(t)
	defer func() {
		if pgContainer != nil {
			if err := pgContainer.Terminate(context.Background()); err != nil {
				t.Logf("failed to terminate container: %v", err)
			}
		}
	}()

	s := NewSystem(cfg)
	s.SetContext(context.Background())

	testPolicy := &structs.Policy{
		Name:      "Test Policy",
		DataModel: `{"field": "value"}`,
		Tests:     `{"test": "case"}`,
		Rule:      "Test rule for validation",
	}

	result, err := s.StoreInitialPolicy(testPolicy)
	assert.NoError(t, err)
	assert.NotEmpty(t, result.BaseID)
	assert.Equal(t, "draft", result.Version)
	assert.Equal(t, testPolicy.Name, result.Name)
	assert.Equal(t, testPolicy.DataModel, result.DataModel)
	assert.Equal(t, testPolicy.Tests, result.Tests)
	assert.Equal(t, testPolicy.Rule, result.Rule)
}

func TestSystem_UpdateDraft(t *testing.T) {
	pgContainer, cfg := setupTestDatabase(t)
	defer func() {
		if pgContainer != nil {
			if err := pgContainer.Terminate(context.Background()); err != nil {
				t.Logf("failed to terminate container: %v", err)
			}
		}
	}()

	s := NewSystem(cfg)
	s.SetContext(context.Background())

	initialPolicy := &structs.Policy{
		Name:      "Update Test Policy",
		DataModel: `{"initial": "data"}`,
		Tests:     `{"initial": "test"}`,
		Rule:      "Initial rule",
	}

	created, err := s.StoreInitialPolicy(initialPolicy)
	require.NoError(t, err)

	updatedPolicy := structs.Policy{
		BaseID:      created.BaseID,
		DataModel:   `{"updated": "data"}`,
		Tests:       `{"updated": "test"}`,
		Rule:        "Updated rule",
		Description: "Updated description",
	}

	err = s.UpdateDraft(updatedPolicy)
	assert.NoError(t, err)

	// Get the actual draft policy ID
	ctx := context.Background()
	client, err := cfg.Database.GetPGXPoolClient(ctx)
	require.NoError(t, err)
	defer client.Close()

	var draftPolicyId string
	err = client.QueryRow(ctx, `SELECT policy_id FROM policies WHERE base_policy_id = $1 AND status = 'draft'`, created.BaseID).Scan(&draftPolicyId)
	require.NoError(t, err)

	loaded, err := s.LoadPolicy(draftPolicyId)
	require.NoError(t, err)
	assert.Equal(t, updatedPolicy.DataModel, loaded.DataModel)
	assert.Equal(t, updatedPolicy.Tests, loaded.Tests)
	assert.Equal(t, updatedPolicy.Rule, loaded.Rule)
}

func TestSystem_CreateVersion(t *testing.T) {
	pgContainer, cfg := setupTestDatabase(t)
	defer func() {
		if pgContainer != nil {
			if err := pgContainer.Terminate(context.Background()); err != nil {
				t.Logf("failed to terminate container: %v", err)
			}
		}
	}()

	s := NewSystem(cfg)
	s.SetContext(context.Background())

	initialPolicy := &structs.Policy{
		Name:      "Version Test Policy",
		DataModel: `{"version": "test"}`,
		Tests:     `{"version": "test"}`,
		Rule:      "Version test rule",
	}

	created, err := s.StoreInitialPolicy(initialPolicy)
	require.NoError(t, err)

	versionPolicy := structs.Policy{
		BaseID:      created.BaseID,
		Version:     "1.0",
		Description: "First version release",
	}

	err = s.CreateVersion(versionPolicy)
	assert.NoError(t, err)

	versions, err := s.GetPolicyVersions(created.BaseID)
	require.NoError(t, err)
	assert.Len(t, versions, 1)
	assert.Equal(t, "v1.0", versions[0].Version)
	assert.Equal(t, "First version release", versions[0].Description)
}

func TestSystem_LoadPolicy(t *testing.T) {
	pgContainer, cfg := setupTestDatabase(t)
	defer func() {
		if pgContainer != nil {
			if err := pgContainer.Terminate(context.Background()); err != nil {
				t.Logf("failed to terminate container: %v", err)
			}
		}
	}()

	s := NewSystem(cfg)
	s.SetContext(context.Background())

	testPolicy := &structs.Policy{
		Name:      "Load Test Policy",
		DataModel: `{"load": "test"}`,
		Tests:     `{"load": "test"}`,
		Rule:      "Load test rule",
	}

	created, err := s.StoreInitialPolicy(testPolicy)
	require.NoError(t, err)

	ctx := context.Background()
	client, err := cfg.Database.GetPGXPoolClient(ctx)
	require.NoError(t, err)
	defer client.Close()

	var policyId string
	err = client.QueryRow(ctx, `SELECT policy_id FROM policies WHERE base_policy_id = $1 AND status = 'draft'`, created.BaseID).Scan(&policyId)
	require.NoError(t, err)

	loaded, err := s.LoadPolicy(policyId)
	assert.NoError(t, err)
	assert.Equal(t, testPolicy.Name, loaded.Name)
	assert.Equal(t, testPolicy.DataModel, loaded.DataModel)
	assert.Equal(t, testPolicy.Tests, loaded.Tests)
	assert.Equal(t, testPolicy.Rule, loaded.Rule)
	assert.Equal(t, "draft", loaded.Status)
}

func TestSystem_DraftFromVersion(t *testing.T) {
	pgContainer, cfg := setupTestDatabase(t)
	defer func() {
		if pgContainer != nil {
			if err := pgContainer.Terminate(context.Background()); err != nil {
				t.Logf("failed to terminate container: %v", err)
			}
		}
	}()

	s := NewSystem(cfg)
	s.SetContext(context.Background())

	initialPolicy := &structs.Policy{
		Name:      "Draft From Version Test",
		DataModel: `{"draft": "from version"}`,
		Tests:     `{"draft": "test"}`,
		Rule:      "Draft from version rule",
	}

	created, err := s.StoreInitialPolicy(initialPolicy)
	require.NoError(t, err)

	versionPolicy := structs.Policy{
		BaseID:      created.BaseID,
		Version:     "1.0",
		Description: "Version to create draft from",
	}

	err = s.CreateVersion(versionPolicy)
	require.NoError(t, err)

	ctx := context.Background()
	client, err := cfg.Database.GetPGXPoolClient(ctx)
	require.NoError(t, err)
	defer client.Close()

	var versionPolicyId string
	err = client.QueryRow(ctx, `SELECT policy_id FROM policies WHERE base_policy_id = $1 AND version = 'v1.0'`, created.BaseID).Scan(&versionPolicyId)
	require.NoError(t, err)

	newDraft, err := s.DraftFromVersion(versionPolicyId)
	assert.NoError(t, err)
	assert.NotEqual(t, versionPolicyId, newDraft.PolicyID)
	assert.Equal(t, created.BaseID, newDraft.BaseID)
	assert.Equal(t, initialPolicy.Name, newDraft.Name)
	assert.Equal(t, initialPolicy.DataModel, newDraft.DataModel)
	assert.Equal(t, "draft", newDraft.Status)
	assert.True(t, newDraft.IsDraft)
}

func TestSystem_AllPolicies(t *testing.T) {
	pgContainer, cfg := setupTestDatabase(t)
	defer func() {
		if pgContainer != nil {
			if err := pgContainer.Terminate(context.Background()); err != nil {
				t.Logf("failed to terminate container: %v", err)
			}
		}
	}()

	s := NewSystem(cfg)
	s.SetContext(context.Background())

	for i := 0; i < 3; i++ {
		policy := &structs.Policy{
			Name:      "Test Policy " + string(rune('A'+i)),
			DataModel: `{"test": "data"}`,
			Tests:     `{"test": "case"}`,
			Rule:      "Test rule",
		}
		_, err := s.StoreInitialPolicy(policy)
		require.NoError(t, err)
	}

	policies, err := s.AllPolicies()
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(policies), 3)

	for _, p := range policies {
		assert.NotEmpty(t, p.BaseID)
		assert.NotEmpty(t, p.Name)
		assert.True(t, p.HasDraft)
	}
}

func TestSystem_GetPolicyVersions(t *testing.T) {
	pgContainer, cfg := setupTestDatabase(t)
	defer func() {
		if pgContainer != nil {
			if err := pgContainer.Terminate(context.Background()); err != nil {
				t.Logf("failed to terminate container: %v", err)
			}
		}
	}()

	s := NewSystem(cfg)
	s.SetContext(context.Background())

	initialPolicy := &structs.Policy{
		Name:      "Multi-Version Policy",
		DataModel: `{"version": "test"}`,
		Tests:     `{"version": "test"}`,
		Rule:      "Version test rule",
	}

	created, err := s.StoreInitialPolicy(initialPolicy)
	require.NoError(t, err)

	for i := 1; i <= 3; i++ {
		versionPolicy := structs.Policy{
			BaseID:      created.BaseID,
			Version:     string(rune('0' + i)) + ".0",
			Description: "Version " + string(rune('0'+i)),
		}
		err = s.CreateVersion(versionPolicy)
		require.NoError(t, err)

		if i < 3 {
			ctx := context.Background()
			client, err := cfg.Database.GetPGXPoolClient(ctx)
			require.NoError(t, err)

			var baseId sql.NullString
			err = client.QueryRow(ctx, `SELECT base_policy_id FROM policies WHERE base_policy_id = $1 LIMIT 1`, created.BaseID).Scan(&baseId)
			require.NoError(t, err)

			_, err = client.Exec(ctx, `SELECT create_draft_from_version($1, $2)`, created.BaseID, "v"+string(rune('0'+i))+".0")
			require.NoError(t, err)
			client.Close()
		}
	}

	versions, err := s.GetPolicyVersions(created.BaseID)
	assert.NoError(t, err)
	assert.Len(t, versions, 3)

	for i, v := range versions {
		if i < 3 {
			assert.Equal(t, "v"+string(rune('0'+i+1))+".0", v.Version)
			assert.False(t, v.IsDraft)
		}
	}
}

func TestSystem_CompleteWorkflow(t *testing.T) {
	pgContainer, cfg := setupTestDatabase(t)
	defer func() {
		if pgContainer != nil {
			if err := pgContainer.Terminate(context.Background()); err != nil {
				t.Logf("failed to terminate container: %v", err)
			}
		}
	}()

	s := NewSystem(cfg)
	s.SetContext(context.Background())

	policy := &structs.Policy{
		Name:      "Complete Workflow Policy",
		DataModel: `{"workflow": "test"}`,
		Tests:     `{"workflow": "test"}`,
		Rule:      "Workflow test rule",
	}
	created, err := s.StoreInitialPolicy(policy)
	require.NoError(t, err)

	updatedPolicy := structs.Policy{
		BaseID:      created.BaseID,
		DataModel:   `{"workflow": "updated"}`,
		Tests:       `{"workflow": "updated"}`,
		Rule:        "Updated workflow rule",
		Description: "Draft updates",
	}
	err = s.UpdateDraft(updatedPolicy)
	require.NoError(t, err)

	versionPolicy := structs.Policy{
		BaseID:      created.BaseID,
		Version:     "1.0",
		Description: "Initial release",
	}
	err = s.CreateVersion(versionPolicy)
	require.NoError(t, err)

	ctx := context.Background()
	client, err := cfg.Database.GetPGXPoolClient(ctx)
	require.NoError(t, err)

	var draftCount int
	err = client.QueryRow(ctx, `SELECT COUNT(*) FROM policies WHERE base_policy_id = $1 AND status = 'draft'`, created.BaseID).Scan(&draftCount)
	require.NoError(t, err)
	assert.Equal(t, 0, draftCount)

	var versionPolicyId string
	err = client.QueryRow(ctx, `SELECT policy_id FROM policies WHERE base_policy_id = $1 AND version = 'v1.0'`, created.BaseID).Scan(&versionPolicyId)
	require.NoError(t, err)
	client.Close()

	newDraft, err := s.DraftFromVersion(versionPolicyId)
	require.NoError(t, err)
	assert.True(t, newDraft.IsDraft)

	secondUpdate := structs.Policy{
		BaseID:      created.BaseID,
		DataModel:   `{"workflow": "v2"}`,
		Tests:       `{"workflow": "v2"}`,
		Rule:        "V2 workflow rule",
		Description: "V2 draft",
	}
	err = s.UpdateDraft(secondUpdate)
	require.NoError(t, err)

	secondVersion := structs.Policy{
		BaseID:      created.BaseID,
		Version:     "2.0",
		Description: "Second release with improvements",
	}
	err = s.CreateVersion(secondVersion)
	require.NoError(t, err)

	versions, err := s.GetPolicyVersions(created.BaseID)
	require.NoError(t, err)
	assert.Len(t, versions, 2)
}