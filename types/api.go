package types

type AddRequestBodyType struct {
	Url         string  `json:"url"`
	Index       float64 `json:"index"`
	IndexMinor  float64 `json:"index_minor"`
	Request     string  `json:"request"`
	Response    string  `json:"response"`
	GeneratedBy string  `json:"generated_by"`
}
