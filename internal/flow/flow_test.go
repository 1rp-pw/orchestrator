package flow

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
	"gopkg.in/yaml.v3"
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

	policySQL, err := os.ReadFile("../../sql/policy.sql")
	require.NoError(t, err)
	_, err = client.Exec(ctx, string(policySQL))
	if err != nil {
		t.Fatalf("Failed to execute policy schema SQL: %v", err)
	}

	flowSQL, err := os.ReadFile("../../sql/flow.sql")
	require.NoError(t, err)
	_, err = client.Exec(ctx, string(flowSQL))
	if err != nil {
		t.Fatalf("Failed to execute flow schema SQL: %v", err)
	}

	return pgContainer, cfg
}

func TestSystem_StoreInitialFlow(t *testing.T) {
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

	testFlow := &structs.StoredFlow{
		Name: "Test Flow",
		Nodes: []interface{}{
			map[string]interface{}{"id": "start-1", "type": "start"},
		},
		Edges: []interface{}{
			map[string]interface{}{"id": "edge-1", "source": "start-1", "target": "end-1"},
		},
		Tests: []interface{}{
			map[string]interface{}{"id": "test-1", "name": "Test 1"},
		},
		FlatYAML: `flow:\n  start:\n    - id: start-1\n      type: start`,
	}

	result, err := s.StoreInitialFlow(testFlow)
	assert.NoError(t, err)
	assert.NotEmpty(t, result.FlowID)
	assert.NotEmpty(t, result.BaseID)
	assert.Equal(t, "draft", result.Version)
	assert.Equal(t, testFlow.Name, result.Name)
	assert.Equal(t, testFlow.Nodes, result.Nodes)
	assert.Equal(t, testFlow.Edges, result.Edges)
	assert.Equal(t, testFlow.Tests, result.Tests)
	assert.Equal(t, testFlow.FlatYAML, result.FlatYAML)
}

func TestSystem_StoreFlow(t *testing.T) {
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

	initialFlow := &structs.StoredFlow{
		Name: "Update Test Flow",
		Nodes: []interface{}{
			map[string]interface{}{"id": "start-1", "type": "start"},
		},
		Edges: []interface{}{
			map[string]interface{}{"id": "edge-1"},
		},
		Tests: []interface{}{
			map[string]interface{}{"id": "test-1"},
		},
		FlatYAML: `flow: initial`,
	}

	created, err := s.StoreInitialFlow(initialFlow)
	require.NoError(t, err)

	updatedFlow := &structs.StoredFlow{
		BaseID: created.BaseID,
		Nodes: []interface{}{
			map[string]interface{}{"id": "start-1", "type": "start"},
			map[string]interface{}{"id": "policy-1", "type": "policy"},
		},
		Edges: []interface{}{
			map[string]interface{}{"id": "edge-1"},
			map[string]interface{}{"id": "edge-2"},
		},
		Tests: []interface{}{
			map[string]interface{}{"id": "test-1"},
			map[string]interface{}{"id": "test-2"},
		},
		FlatYAML:    `flow: updated`,
		Description: "Updated flow description",
	}

	result, err := s.StoreFlow(updatedFlow)
	assert.NoError(t, err)
	assert.Equal(t, created.BaseID, result.BaseID)

	loaded, err := s.GetFullFlow(created.FlowID)
	require.NoError(t, err)
	assert.Equal(t, updatedFlow.Nodes, loaded.Nodes)
	assert.Equal(t, updatedFlow.Edges, loaded.Edges)
	assert.Equal(t, updatedFlow.Tests, loaded.Tests)
	assert.Equal(t, updatedFlow.FlatYAML, loaded.FlatYAML)
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

	initialFlow := &structs.StoredFlow{
		Name: "Version Test Flow",
		Nodes: []interface{}{
			map[string]interface{}{"id": "start-1"},
		},
		Edges: []interface{}{
			map[string]interface{}{"id": "edge-1"},
		},
		Tests: []interface{}{
			map[string]interface{}{"id": "test-1"},
		},
		FlatYAML: `flow: version test`,
	}

	created, err := s.StoreInitialFlow(initialFlow)
	require.NoError(t, err)

	versionFlow := &structs.StoredFlow{
		BaseID:      created.BaseID,
		Version:     "v1.0",
		Description: "First version release",
	}

	result, err := s.CreateVersion(versionFlow)
	assert.NoError(t, err)
	assert.NotEmpty(t, result.FlowID)
	assert.Equal(t, versionFlow.Version, result.Version)

	versions, err := s.GetFlowVersions(created.BaseID)
	require.NoError(t, err)
	assert.Len(t, versions, 1)
	assert.Equal(t, "v1.0", versions[0].Version)
	assert.Equal(t, "First version release", versions[0].Description)
}

func TestSystem_GetStoredFlow(t *testing.T) {
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

	flowConfig := structs.FlowConfig{
		Flow: structs.Flow{
			Start: []structs.FlowNode{
				{
					ID:       "start-1",
					Type:     "start",
					PolicyID: "test-policy-id",
					OnTrue: []structs.FlowNode{
						{
							ID:          "return-1",
							Type:        "return",
							ReturnValue: true,
						},
					},
					OnFalse: []structs.FlowNode{
						{
							ID:          "return-2",
							Type:        "return",
							ReturnValue: false,
						},
					},
				},
			},
		},
	}

	flowYAML, err := yaml.Marshal(flowConfig)
	require.NoError(t, err)

	testFlow := &structs.StoredFlow{
		Name:     "Get Flow Test",
		Nodes:    `[{"id": "start-1"}]`,
		Edges:    `[{"id": "edge-1"}]`,
		Tests:    `[{"id": "test-1"}]`,
		FlatYAML: string(flowYAML),
	}

	created, err := s.StoreInitialFlow(testFlow)
	require.NoError(t, err)

	loaded, err := s.GetStoredFlow(created.FlowID)
	assert.NoError(t, err)
	assert.NotNil(t, loaded)
	assert.Len(t, loaded.Flow.Start, 1)
	assert.Equal(t, "start-1", loaded.Flow.Start[0].ID)
	assert.Equal(t, "test-policy-id", loaded.Flow.Start[0].PolicyID)
}

func TestSystem_GetFullFlow(t *testing.T) {
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

	testFlow := &structs.StoredFlow{
		Name: "Full Flow Test",
		Nodes: []interface{}{
			map[string]interface{}{"id": "node-1"},
		},
		Edges: []interface{}{
			map[string]interface{}{"id": "edge-1"},
		},
		Tests: []interface{}{
			map[string]interface{}{"id": "test-1"},
		},
		FlatYAML:    `flow: full test`,
		Description: "Full flow description",
	}

	created, err := s.StoreInitialFlow(testFlow)
	require.NoError(t, err)

	loaded, err := s.GetFullFlow(created.FlowID)
	assert.NoError(t, err)
	assert.Equal(t, created.FlowID, loaded.FlowID)
	assert.Equal(t, created.BaseID, loaded.BaseID)
	assert.Equal(t, testFlow.Name, loaded.Name)
	assert.Equal(t, testFlow.Nodes, loaded.Nodes)
	assert.Equal(t, testFlow.Edges, loaded.Edges)
	assert.Equal(t, testFlow.Tests, loaded.Tests)
	assert.Equal(t, testFlow.FlatYAML, loaded.FlatYAML)
	// Version field is empty for drafts
	assert.Empty(t, loaded.Version)
	assert.Equal(t, "draft", loaded.Status)
	assert.True(t, loaded.IsDraft)
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

	initialFlow := &structs.StoredFlow{
		Name: "Draft From Version Test",
		Nodes: []interface{}{
			map[string]interface{}{"id": "version-node"},
		},
		Edges: []interface{}{
			map[string]interface{}{"id": "version-edge"},
		},
		Tests: []interface{}{
			map[string]interface{}{"id": "version-test"},
		},
		FlatYAML: `flow: version to draft`,
	}

	created, err := s.StoreInitialFlow(initialFlow)
	require.NoError(t, err)

	versionFlow := &structs.StoredFlow{
		BaseID:      created.BaseID,
		Version:     "v1.0",
		Description: "Version to create draft from",
	}

	version, err := s.CreateVersion(versionFlow)
	require.NoError(t, err)

	newDraft, err := s.DraftFromVersion(version.FlowID)
	assert.NoError(t, err)
	assert.NotEqual(t, version.FlowID, newDraft.FlowID)
	assert.Equal(t, created.BaseID, newDraft.BaseID)
	assert.Equal(t, initialFlow.Name, newDraft.Name)
	assert.Equal(t, initialFlow.Nodes, newDraft.Nodes)
	assert.Equal(t, "draft", newDraft.Status)
	// Version field will be empty for drafts as loaded by GetFullFlow
	assert.Empty(t, newDraft.Version)
	assert.True(t, newDraft.IsDraft)
}

func TestSystem_AllFlows(t *testing.T) {
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
		flow := &structs.StoredFlow{
			Name: "Test Flow " + string(rune('A'+i)),
			Nodes: []interface{}{
				map[string]interface{}{"id": "node"},
			},
			Edges: []interface{}{
				map[string]interface{}{"id": "edge"},
			},
			Tests: []interface{}{
				map[string]interface{}{"id": "test"},
			},
			FlatYAML: `flow: test`,
		}
		_, err := s.StoreInitialFlow(flow)
		require.NoError(t, err)
	}

	flows, err := s.AllFlows()
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(flows), 3)

	for _, f := range flows {
		assert.NotEmpty(t, f.BaseID)
		assert.NotEmpty(t, f.Name)
		assert.True(t, f.HasDraft)
	}
}

func TestSystem_GetFlowVersions(t *testing.T) {
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

	initialFlow := &structs.StoredFlow{
		Name: "Multi-Version Flow",
		Nodes: []interface{}{
			map[string]interface{}{"id": "node"},
		},
		Edges: []interface{}{
			map[string]interface{}{"id": "edge"},
		},
		Tests: []interface{}{
			map[string]interface{}{"id": "test"},
		},
		FlatYAML: `flow: multi-version`,
	}

	created, err := s.StoreInitialFlow(initialFlow)
	require.NoError(t, err)

	for i := 1; i <= 3; i++ {
		versionFlow := &structs.StoredFlow{
			BaseID:      created.BaseID,
			Version:     "v" + string(rune('0'+i)) + ".0",
			Description: "Version " + string(rune('0'+i)),
		}
		_, err = s.CreateVersion(versionFlow)
		require.NoError(t, err)

		if i < 3 {
			ctx := context.Background()
			client, err := cfg.Database.GetPGXPoolClient(ctx)
			require.NoError(t, err)

			var baseId sql.NullString
			err = client.QueryRow(ctx, `SELECT base_flow_id FROM flows WHERE base_flow_id = $1 LIMIT 1`, created.BaseID).Scan(&baseId)
			require.NoError(t, err)

			_, err = client.Exec(ctx, `SELECT create_draft_flow_from_version($1, $2)`, created.BaseID, "v"+string(rune('0'+i))+".0")
			require.NoError(t, err)
			client.Close()
		}
	}

	versions, err := s.GetFlowVersions(created.BaseID)
	assert.NoError(t, err)
	assert.Len(t, versions, 3)

	for i, v := range versions {
		assert.Equal(t, "v"+string(rune('0'+i+1))+".0", v.Version)
		assert.False(t, v.IsDraft)
	}
}

func TestSystem_RunTestFlow(t *testing.T) {
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

	customOutcome := "Custom response text"
	testFlow := structs.FlowTestRequest{
		Flow: structs.FlowConfig{
			Flow: structs.Flow{
				Start: []structs.FlowNode{
					{
						ID:          "custom-1",
						Type:        "custom",
						Outcome:     &customOutcome,
						ReturnValue: true,
					},
				},
			},
		},
		Data: map[string]interface{}{
			"test": "data",
		},
	}

	response, err := s.RunTestFlow(testFlow)
	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Len(t, response.NodeResponse, 1)

	// Check that we got a custom response
	assert.Equal(t, "custom", response.NodeResponse[0].NodeType)
	assert.Equal(t, "custom-1", response.NodeResponse[0].NodeID)
	assert.True(t, response.NodeResponse[0].Response.Result)
	assert.Equal(t, customOutcome, response.Result)
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

	flow := &structs.StoredFlow{
		Name: "Complete Workflow Flow",
		Nodes: []interface{}{
			map[string]interface{}{"id": "workflow"},
		},
		Edges: []interface{}{
			map[string]interface{}{"id": "workflow-edge"},
		},
		Tests: []interface{}{
			map[string]interface{}{"id": "workflow-test"},
		},
		FlatYAML: `flow: workflow`,
	}
	created, err := s.StoreInitialFlow(flow)
	require.NoError(t, err)

	updatedFlow := &structs.StoredFlow{
		BaseID: created.BaseID,
		Nodes: []interface{}{
			map[string]interface{}{"id": "workflow-updated"},
		},
		Edges: []interface{}{
			map[string]interface{}{"id": "workflow-edge-updated"},
		},
		Tests: []interface{}{
			map[string]interface{}{"id": "workflow-test-updated"},
		},
		FlatYAML:    `flow: workflow updated`,
		Description: "Updated workflow",
	}
	_, err = s.StoreFlow(updatedFlow)
	require.NoError(t, err)

	versionFlow := &structs.StoredFlow{
		BaseID:      created.BaseID,
		Version:     "v1.0",
		Description: "Initial workflow release",
	}
	version1, err := s.CreateVersion(versionFlow)
	require.NoError(t, err)

	ctx := context.Background()
	client, err := cfg.Database.GetPGXPoolClient(ctx)
	require.NoError(t, err)

	var draftCount int
	err = client.QueryRow(ctx, `SELECT COUNT(*) FROM flows WHERE base_flow_id = $1 AND status = 'draft'`, created.BaseID).Scan(&draftCount)
	require.NoError(t, err)
	assert.Equal(t, 0, draftCount)
	client.Close()

	newDraft, err := s.DraftFromVersion(version1.FlowID)
	require.NoError(t, err)
	assert.Equal(t, "draft", newDraft.Status)

	secondUpdate := &structs.StoredFlow{
		BaseID: created.BaseID,
		Nodes: []interface{}{
			map[string]interface{}{"id": "workflow-v2"},
		},
		Edges: []interface{}{
			map[string]interface{}{"id": "workflow-edge-v2"},
		},
		Tests: []interface{}{
			map[string]interface{}{"id": "workflow-test-v2"},
		},
		FlatYAML:    `flow: workflow v2`,
		Description: "V2 draft",
	}
	_, err = s.StoreFlow(secondUpdate)
	require.NoError(t, err)

	secondVersion := &structs.StoredFlow{
		BaseID:      created.BaseID,
		Version:     "v2.0",
		Description: "Second release with improvements",
	}
	_, err = s.CreateVersion(secondVersion)
	require.NoError(t, err)

	versions, err := s.GetFlowVersions(created.BaseID)
	require.NoError(t, err)
	assert.Len(t, versions, 2)
	assert.Equal(t, "v1.0", versions[0].Version)
	assert.Equal(t, "v2.0", versions[1].Version)
}