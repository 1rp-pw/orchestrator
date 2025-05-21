package policy

type Policy struct {
	ID      string      `json:"id"`
	Rule    string      `json:"rule"`
	Data    interface{} `json:"data"`
	Tests   interface{} `json:"tests"`
	Version string      `json:"version"`
}
