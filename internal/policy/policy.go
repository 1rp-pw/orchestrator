package policy

import "time"

type Policy struct {
	ID        string      `json:"id"`
	Rule      string      `json:"rule"`
	Data      interface{} `json:"data"`
	Tests     interface{} `json:"tests"`
	Version   string      `json:"version"`
	CreatedAt time.Time   `json:"created_at"`
	DataModel interface{} `json:"data_model"`
	Name      string      `json:"name"`
}
