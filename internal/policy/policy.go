package policy

import "time"

type Policy struct {
	ID        string      `json:"id"`
	Rule      string      `json:"rule"`
	Data      interface{} `json:"data"`
	Tests     interface{} `json:"tests"`
	Version   string      `json:"version"`
	CreatedAt time.Time   `json:"createdAt"`
	UpdatedAt time.Time   `json:"updatedAt"`
	DataModel interface{} `json:"schema"`
	Name      string      `json:"name"`
}
