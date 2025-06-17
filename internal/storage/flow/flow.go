package flow

import (
	"context"
	"fmt"
	"github.com/1rp-pw/orchestrator/internal/engine"
	"github.com/1rp-pw/orchestrator/internal/storage/policy"
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
		Config: cfg,
	}
}

func (s *System) SetContext(ctx context.Context) *System {
	s.Context = ctx
	return s
}

func (s *System) RunTestFlow(f structs.FlowRequest) (structs.FlowResponse, error) {
	fr := structs.FlowResponse{
		NodeResponse: make([]structs.FlowNodeResponse, 0),
	}

	flow := f.Flow
	data := f.Data

	// Execute all start nodes
	for _, startNode := range flow.Flow.Start {
		result, responses, err := s.executeNode(startNode, data)
		if err != nil {
			return structs.FlowResponse{}, logs.Errorf("failed to execute flow: %v", err)
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
