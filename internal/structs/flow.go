package structs

import (
	"database/sql"
	"time"
)

type FlowTestRequest struct {
	Data     interface{} `json:"data"`
	FlowYAML string      `json:"flow"`
	Flow     FlowConfig
}

type FlowRequest struct {
	ID          string      `json:"id"`
	BaseID      string      `json:"baseId"`
	Tests       interface{} `json:"tests"`
	Name        string      `json:"name"`
	FlowYAML    string      `json:"flowFlat"`
	Nodes       interface{} `json:"nodes"`
	Edges       interface{} `json:"edges"`
	Description string      `json:"description"`
	Version     string      `json:"version"`
	Status      string      `json:"status"`
	Flow        FlowConfig
}

type StoredFlow struct {
	BaseID          string `yaml:"baseId" json:"baseId"`
	FlowID          string `yaml:"id" json:"id"`
	Name            string `yaml:"name" json:"name"`
	Description     string `yaml:"description" json:"description"`
	DescNull        sql.NullString
	Nodes           interface{} `yaml:"nodes" json:"nodes"`
	Edges           interface{} `yaml:"edges" json:"edges"`
	Tests           interface{} `yaml:"tests" json:"tests"`
	Version         string      `yaml:"version" json:"version"`
	VerNull         sql.NullString
	IsDraft         bool      `yaml:"draft" json:"draft"`
	Status          string    `yaml:"status" json:"status"`
	CreatedAt       time.Time `yaml:"createdAt" json:"createdAt"`
	UpdatedAt       time.Time `yaml:"updatedAt" json:"updatedAt"`
	LastPublishedAt time.Time `yaml:"lastPublishedAt" json:"lastPublishedAt"`
	HasDraft        bool      `yaml:"hasDraft" json:"hasDraft"`
	FlatYAML        string    `yaml:"flowFlat" json:"flowFlat"`
	FlowConfig      FlowConfig
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
