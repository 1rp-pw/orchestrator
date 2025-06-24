package flow

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/1rp-pw/orchestrator/internal/engine"
	"github.com/1rp-pw/orchestrator/internal/errors"
	"github.com/1rp-pw/orchestrator/internal/policy"
	"github.com/1rp-pw/orchestrator/internal/structs"
	"github.com/bugfixes/go-bugfixes/logs"
	ConfigBuilder "github.com/keloran/go-config"
	"gopkg.in/yaml.v3"
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

func (s *System) RunTestFlow(f structs.FlowTestRequest) (structs.FlowResponse, error) {
	flow := f.Flow
	data := f.Data

	return s.RunFlowInternal(flow, data)
}

func (s *System) RunFlowInternal(flow structs.FlowConfig, data interface{}) (structs.FlowResponse, error) {
	fr := structs.FlowResponse{
		NodeResponse: make([]structs.FlowNodeResponse, 0),
	}

	for _, startNode := range flow.Flow.Start {
		result, responses, err := s.executeNode(startNode, data)
		if err != nil {
			return structs.FlowResponse{}, fmt.Errorf("failed to execute flow: %w", err)
		}

		fr.NodeResponse = append(fr.NodeResponse, responses...)
		fr.Result = result
	}

	return fr, nil
}

// executeNode recursively executes a flow node and all its children
func (s *System) executeNode(node structs.FlowNode, data interface{}) (interface{}, []structs.FlowNodeResponse, error) {
	var allResponses []structs.FlowNodeResponse

	switch node.Type {
	case "start", "policy":
		if node.PolicyID == "" {
			return nil, nil, errors.WrapFlowError(errors.ErrMissingPolicyID, "", node.ID)
		}

		// Execute policy (start nodes also have policyId)
		response, err := s.flowPolicy(node.PolicyID, data)
		if err != nil {
			return nil, nil, logs.Errorf("failed to execute policy node %s: %v", node.ID, err)
		}

		allResponses = append(allResponses, structs.FlowNodeResponse{
			NodeID:   node.ID,
			NodeType: node.Type,
			Response: response,
		})

		// Parse the result based on ReturnValue
		result := s.returnParse(response.Result, node.ReturnValue)

		// Continue execution based on result
		var nextNodes []structs.FlowNode
		if result {
			nextNodes = node.OnTrue
		} else {
			nextNodes = node.OnFalse
		}

		// Execute next nodes
		var lastResult interface{} = result
		for _, nextNode := range nextNodes {
			nextResult, nextResponses, err := s.executeNode(nextNode, data)
			if err != nil {
				return nil, nil, err
			}
			allResponses = append(allResponses, nextResponses...)

			// Update result with the last node's result
			if nextResult != nil {
				lastResult = nextResult
			}
		}

		return lastResult, allResponses, nil

	case "return":
		// Return node - terminates with specified value, no additional response needed
		return node.ReturnValue, allResponses, nil

	case "custom":
		// Custom response node - create a policy-like response structure
		customTrace := map[string]interface{}{
			"execution": []map[string]interface{}{
				{
					"conditions": []interface{}{},
					"outcome": map[string]interface{}{
						"value": *node.Outcome,
					},
					"result": true,
					"selector": map[string]interface{}{
						"value": "custom_response",
					},
				},
			},
		}

		customResponse := structs.EngineResponse{
			Result: true,
			Trace:  customTrace,
			Rule:   []string{fmt.Sprintf("Custom response: %s", *node.Outcome)},
			Data:   data,
			Error:  nil,
		}
		allResponses = append(allResponses, structs.FlowNodeResponse{
			NodeID:   node.ID,
			NodeType: node.Type,
			Response: customResponse,
		})

		// Continue with next nodes if any
		var nextNodes []structs.FlowNode
		nextNodes = append(nextNodes, node.OnTrue...)
		nextNodes = append(nextNodes, node.OnFalse...)

		// For custom nodes, if there are no next nodes, return the outcome
		if len(nextNodes) == 0 {
			return *node.Outcome, allResponses, nil
		}

		// Otherwise, continue with next nodes
		var result interface{} = *node.Outcome
		for _, nextNode := range nextNodes {
			nextResult, nextResponses, err := s.executeNode(nextNode, data)
			if err != nil {
				return nil, nil, err
			}
			allResponses = append(allResponses, nextResponses...)
			if nextResult != nil {
				result = nextResult
			}
		}

		return result, allResponses, nil

	default:
		return nil, nil, logs.Errorf("unknown node type: %s", node.Type)
	}
}

func (s *System) returnParse(ResponseResult bool, ReturnValue interface{}) bool {
	if ReturnValue == nil {
		return ResponseResult
	}

	if ReturnValue.(bool) {
		return ResponseResult
	}

	return false
}

func (s *System) flowPolicy(policyId string, data interface{}) (structs.EngineResponse, error) {
	st := policy.NewSystem(s.Config).SetContext(s.Context)
	p, err := st.LoadPolicy(policyId)
	if err != nil {
		return structs.EngineResponse{}, logs.Errorf("failed to load policy: %v", err)
	}
	p.Data = data

	pe := engine.NewSystem(s.Config).SetContext(s.Context)
	pr, err := pe.RunPolicyInternal(p)
	if err != nil {
		return structs.EngineResponse{}, logs.Errorf("failed to run policy: %v", err)
	}

	return *pr, nil
}

func (s *System) AllFlows() ([]structs.StoredFlow, error) {
	var ff []structs.StoredFlow
	client, err := s.Config.Database.GetPGXPoolClient(s.Context)
	if err != nil {
		return ff, logs.Errorf("failed to connect to database: %v", err)
	}
	defer client.Close()
	rows, err := client.Query(s.Context, `
		SELECT 
			base_flow_id,
			current_name,
			version_count,
			draft_id,
			first_created_date,
			latest_version_date,
			latest_activity_date,
			has_draft
		FROM public.flow_summary`)
	if err != nil {
		return ff, logs.Errorf("failed to query flows: %v", err)
	}
	defer rows.Close()

	type dataStruct struct {
		FlowBaseID         sql.NullString
		CurrentName        sql.NullString
		VersionCount       sql.NullInt32
		DraftID            sql.NullString
		FirstCreatedDate   sql.NullTime
		LatestVersionDate  sql.NullTime
		LatestActivityDate sql.NullTime
		HasDraft           sql.NullBool
	}

	for rows.Next() {
		d := dataStruct{}
		if err := rows.Scan(
			&d.FlowBaseID,
			&d.CurrentName,
			&d.VersionCount,
			&d.DraftID,
			&d.FirstCreatedDate,
			&d.LatestActivityDate,
			&d.LatestVersionDate,
			&d.HasDraft,
		); err != nil {
			return ff, logs.Errorf("failed to load flows: %v", err)
		}

		f := structs.StoredFlow{
			BaseID:    d.FlowBaseID.String,
			Name:      d.CurrentName.String,
			HasDraft:  d.HasDraft.Bool,
			UpdatedAt: d.LatestActivityDate.Time,
			CreatedAt: d.FirstCreatedDate.Time,
		}
		if d.LatestVersionDate.Valid {
			f.LastPublishedAt = d.LatestVersionDate.Time
		}
		ff = append(ff, f)
	}

	return ff, nil
}

func (s *System) GetFlowVersions(baseFlowId string) ([]structs.StoredFlow, error) {
	var ff []structs.StoredFlow

	client, err := s.Config.Database.GetPGXPoolClient(s.Context)
	if err != nil {
		return ff, logs.Errorf("failed to connect to database: %v", err)
	}
	defer client.Close()
	rows, err := client.Query(s.Context, `
		SELECT
			flow_id,
			name,
			version,
			flow,
			nodes,
			edges,
			description,
			status,
			created_at,
			updated_at
		FROM flows
		WHERE base_flow_id = $1
		ORDER BY
			CASE WHEN status = 'draft' THEN 0 ELSE 1 END,
			CASE WHEN version IS NULL THEN '' ELSE version END`, baseFlowId)
	if err != nil {
		return ff, logs.Errorf("failed to load flows: %v", err)
	}
	defer rows.Close()

	type dataStruct struct {
		FlowID      sql.NullString
		Name        sql.NullString
		Version     sql.NullString
		Flow        sql.NullString
		Nodes       sql.NullString
		Edges       sql.NullString
		Description sql.NullString
		Status      sql.NullString
		CreatedAt   sql.NullTime
		UpdatedAt   sql.NullTime
	}
	for rows.Next() {
		d := dataStruct{}
		if err := rows.Scan(
			&d.FlowID,
			&d.Name,
			&d.Version,
			&d.Flow,
			&d.Nodes,
			&d.Edges,
			&d.Description,
			&d.Status,
			&d.CreatedAt,
			&d.UpdatedAt,
		); err != nil {
			return ff, logs.Errorf("failed to load flows: %v", err)
		}

		f := structs.StoredFlow{
			BaseID:      baseFlowId,
			FlowID:      d.FlowID.String,
			Name:        d.Name.String,
			Description: d.Description.String,
			Nodes:       d.Nodes.String,
			Edges:       d.Edges.String,
			Status:      d.Status.String,
			CreatedAt:   d.CreatedAt.Time,
			UpdatedAt:   d.UpdatedAt.Time,
		}
		if d.Status.String == "draft" {
			f.IsDraft = true
		}
		if d.Version.String != "" {
			f.Version = d.Version.String
		}

		ff = append(ff, f)
	}

	return ff, nil
}

func (s *System) StoreInitialFlow(f *structs.StoredFlow) (*structs.StoredFlow, error) {
	client, err := s.Config.Database.GetPGXPoolClient(s.Context)
	if err != nil {
		return f, logs.Errorf("failed to connect to database: %v", err)
	}
	defer client.Close()

	if err := client.QueryRow(s.Context, `SELECT create_flow($1, $2, $3, $4, $5)`, f.Name, f.Nodes, f.Edges, f.Tests, f.FlatYAML).Scan(&f.FlowID); err != nil {
		return nil, logs.Errorf("failed to store initial structs: %v", err)
	}

	if err := client.QueryRow(s.Context, `SELECT base_flow_id FROM flows WHERE flow_id = $1`, f.FlowID).Scan(&f.BaseID); err != nil {
		return nil, logs.Errorf("failed to store initial structs: %v", err)
	}
	f.Version = "draft"

	return f, nil
}

func (s *System) StoreFlow(f *structs.StoredFlow) (*structs.StoredFlow, error) {
	client, err := s.Config.Database.GetPGXPoolClient(s.Context)
	if err != nil {
		return f, logs.Errorf("failed to connect to database: %v", err)
	}
	defer client.Close()

	if _, err := client.Exec(s.Context, `SELECT update_draft_flow($1, $2, $3, $4, $5, $6)`, f.BaseID, f.Nodes, f.Edges, f.Tests, f.FlatYAML, f.Description); err != nil {
		return nil, logs.Errorf("failed to store initial structs: %v", err)
	}

	return f, nil
}

func (s *System) GetStoredFlow(flowId string) (*structs.FlowConfig, error) {
	client, err := s.Config.Database.GetPGXPoolClient(s.Context)
	if err != nil {
		return nil, logs.Errorf("failed to connect to database: %v", err)
	}
	defer client.Close()
	var f structs.FlowConfig
	var x interface{}

	if err := client.QueryRow(s.Context, `SELECT flow FROM flows WHERE flow_id = $1`, flowId).Scan(&x); err != nil {
		return nil, logs.Errorf("failed to get flow: %v", err)
	}

	if err := yaml.Unmarshal([]byte(x.(string)), &f); err != nil {
		return nil, logs.Errorf("failed to unmarshal flow: %v", err)
	}

	return &f, nil
}

func (s *System) GetFullFlow(flowId string) (*structs.StoredFlow, error) {
	client, err := s.Config.Database.GetPGXPoolClient(s.Context)
	if err != nil {
		return nil, logs.Errorf("failed to connect to database: %v", err)
	}
	defer client.Close()
	var f structs.StoredFlow

	if err := client.QueryRow(s.Context, `
		SELECT 
		    base_flow_id, 
		    description, 
		    name, 
		    flow, 
		    nodes, 
		    edges, 
		    tests,
		    version,
		    status,
		    created_at,
		    updated_at
		FROM flows 
		WHERE flow_id = $1`,
		flowId).Scan(
		&f.BaseID,
		&f.DescNull,
		&f.Name,
		&f.FlatYAML,
		&f.Nodes,
		&f.Edges,
		&f.Tests,
		&f.VerNull,
		&f.Status,
		&f.CreatedAt,
		&f.UpdatedAt); err != nil {
		return nil, logs.Errorf("failed to get flow: %v", err)
	}
	f.FlowID = flowId
	if f.DescNull.Valid {
		f.Description = f.DescNull.String
	}
	if f.VerNull.Valid {
		f.Version = f.VerNull.String
	}

	return &f, nil
}
