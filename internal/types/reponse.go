package types

type ResponseList[T any] struct {
	Page       int `json:"page"`
	PerPage    int `json:"perPage"`
	TotalItems int `json:"totalItems"`
	TotalPages int `json:"totalPages"`
	Items      []T `json:"items"`
}

type ResponseCreate struct {
	ID      string `json:"id"`
	Created string `json:"created"`
	Field   string `json:"field"`
	Updated string `json:"updated"`
}
