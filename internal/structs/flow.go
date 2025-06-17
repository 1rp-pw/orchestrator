package structs

import "time"

type FlowRequest struct {
	Data     interface{} `json:"data"`
	JSONFlow string      `json:"flow"`
	Flow     FlowConfig
}

type FlowConfig struct {
	Flow     Flow         `yaml:"flow" json:"flow"`
	Metadata FlowMetadata `yaml:"metadata" json:"metadata"`
}

type Flow struct {
	Start []FlowNode `yaml:"start" json:"start"`
}
type FlowNode struct {
	ID          string      `yaml:"id" json:"id"`
	Type        string      `yaml:"type" json:"type"`
	PolicyID    string      `yaml:"policyId" json:"policyId"`
	ReturnValue interface{} `yaml:"returnValue" json:"returnValue"`
	Outcome     *string     `yaml:"outcome" json:"outcome"`
	OnTrue      []FlowNode  `yaml:"onTrue" json:"onTrue"`
	OnFalse     []FlowNode  `yaml:"onFalse" json:"onFalse"`
}

type FlowMetadata struct {
	TotalNodes int       `yaml:"totalNodes" json:"totalNodes"`
	TotalEdges int       `yaml:"totalEdges" json:"totalEdges"`
	Timestamp  time.Time `yaml:"timestamp" json:"timestamp"`
}

type FlowResponse struct {
	Result       interface{}        `json:"result"`
	NodeResponse []FlowNodeResponse `json:"nodeResponse"`
}

type FlowNodeResponse struct {
	NodeID   string         `json:"nodeId"`
	NodeType string         `json:"nodeType"`
	Response EngineResponse `json:"response"`
}
