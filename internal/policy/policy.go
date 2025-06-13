package policy

import "time"

type Policy struct {
	PolicyID        string      `json:"id"`
	BaseID          string      `json:"baseId"`
	Rule            string      `json:"rule"`
	Data            interface{} `json:"data"`
	Tests           interface{} `json:"tests"`
	Version         string      `json:"version"`
	CreatedAt       time.Time   `json:"createdAt"`
	UpdatedAt       time.Time   `json:"updatedAt"`
	LastPublishedAt time.Time   `json:"lastPublishedAt"`
	DataModel       interface{} `json:"schema"`
	Name            string      `json:"name"`
	IsDraft         bool        `json:"draft"`
	Description     string      `json:"description"`
	DraftID         string      `json:"draftId"`
	Status          string      `json:"status"`
	HasDraft        bool        `json:"hasDraft"`
}
