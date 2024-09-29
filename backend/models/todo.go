package models

type Todo struct {
	ID          int    `json:"id"`
	Todo        string `json:"todo"`
	IsCompleted bool   `json:"isCompleted"`
}
