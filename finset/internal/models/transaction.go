package models

import "time"

// Transaction represents a single financial transaction.
type Transaction struct {
	ID       string    `json:"id"`
	Type     string    `json:"type"`     // "income" | "expense"
	Amount   float64   `json:"amount"`
	Category string    `json:"category"`
	Method   string    `json:"method"`
	Date     string    `json:"date"`     // "YYYY-MM-DD"
	Note     string    `json:"note"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateTransactionRequest is the body expected for POST /api/transactions.
type CreateTransactionRequest struct {
	Type     string  `json:"type"`
	Amount   float64 `json:"amount"`
	Category string  `json:"category"`
	Method   string  `json:"method"`
	Date     string  `json:"date"`
	Note     string  `json:"note"`
}

// Validate returns an error string if the request is invalid, empty string otherwise.
func (r *CreateTransactionRequest) Validate() string {
	if r.Type != "income" && r.Type != "expense" {
		return "type must be 'income' or 'expense'"
	}
	if r.Amount <= 0 {
		return "amount must be positive"
	}
	if r.Category == "" {
		return "category is required"
	}
	if r.Date == "" {
		return "date is required"
	}
	return ""
}
