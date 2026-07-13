package domain

import "time"

// Amount is a monetary value in PayPal-style shape.
type Amount struct {
	CurrencyCode string `json:"currency_code"`
	Value        string `json:"value"`
}

// Link is a HATEOAS link.
type Link struct {
	Href   string `json:"href"`
	Rel    string `json:"rel"`
	Method string `json:"method"`
}

// Authorization statuses.
const (
	StatusCreated            = "CREATED"
	StatusCaptured           = "CAPTURED"
	StatusPartiallyCaptured  = "PARTIALLY_CAPTURED"
	StatusVoided             = "VOIDED"
	StatusDenied             = "DENIED"
	StatusCompleted          = "COMPLETED"
	StatusPartiallyRefunded  = "PARTIALLY_REFUNDED"
	StatusRefunded           = "REFUNDED"
)

// Authorization represents an authorized payment.
type Authorization struct {
	ID         string    `json:"id"`
	Status     string    `json:"status"`
	Amount     Amount    `json:"amount"`
	CreateTime time.Time `json:"create_time"`
	UpdateTime time.Time `json:"update_time"`
	Links      []Link    `json:"links,omitempty"`

	// Internal bookkeeping (not always serialized the same way).
	CapturedCents int64    `json:"-"`
	CaptureIDs    []string `json:"-"`
}

// Capture represents a captured payment.
type Capture struct {
	ID             string    `json:"id"`
	Status         string    `json:"status"`
	Amount         Amount    `json:"amount"`
	AuthorizationID string   `json:"-"`
	CreateTime     time.Time `json:"create_time"`
	UpdateTime     time.Time `json:"update_time"`
	Links          []Link    `json:"links,omitempty"`

	RefundedCents int64    `json:"-"`
	RefundIDs     []string `json:"-"`
}

// Refund represents a refund of a capture.
type Refund struct {
	ID         string    `json:"id"`
	Status     string    `json:"status"`
	Amount     Amount    `json:"amount"`
	CaptureID  string    `json:"-"`
	CreateTime time.Time `json:"create_time"`
	UpdateTime time.Time `json:"update_time"`
	Links      []Link    `json:"links,omitempty"`
}

// ErrorResponse matches a PayPal-like error body.
type ErrorResponse struct {
	Name    string         `json:"name"`
	Message string         `json:"message"`
	DebugID string         `json:"debug_id,omitempty"`
	Details []ErrorDetail  `json:"details,omitempty"`
}

// ErrorDetail is a single error detail entry.
type ErrorDetail struct {
	Issue       string `json:"issue,omitempty"`
	Description string `json:"description,omitempty"`
	Field       string `json:"field,omitempty"`
}
