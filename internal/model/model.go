package model

type Type string

const (
	NORMAL  Type = "normal"
	EXPRESS Type = "express"
)

type SMS struct {
	CustomerID int64    `json:"customer_id"`
	Text       string   `json:"text"`
	Recipients []string `json:"recipients"`
	Type       Type     `json:"type"`
}
