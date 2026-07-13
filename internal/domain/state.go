package domain

import "fmt"

// DomainError is a structured domain/validation error.
type DomainError struct {
	Name    string
	Message string
	Issue   string
	Field   string
}

func (e *DomainError) Error() string {
	if e.Issue != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Name, e.Message, e.Issue)
	}
	return fmt.Sprintf("%s: %s", e.Name, e.Message)
}

func newUnprocessable(issue, message string) *DomainError {
	return &DomainError{
		Name:    "UNPROCESSABLE_ENTITY",
		Message: message,
		Issue:   issue,
	}
}

func newInvalidRequest(issue, message, field string) *DomainError {
	return &DomainError{
		Name:    "INVALID_REQUEST",
		Message: message,
		Issue:   issue,
		Field:   field,
	}
}

// ValidateCreateAuthorization checks create-auth input.
func ValidateCreateAuthorization(amount Amount) error {
	if amount.CurrencyCode == "" {
		return newInvalidRequest("MISSING_REQUIRED_PARAMETER", "currency_code is required", "amount.currency_code")
	}
	cents, err := ParseAmountCents(amount.Value)
	if err != nil || cents <= 0 {
		return newInvalidRequest("INVALID_PARAMETER_VALUE", "amount.value must be a positive decimal", "amount.value")
	}
	return nil
}

// CanCapture reports whether an authorization can accept a capture of captureCents.
func CanCapture(auth *Authorization, captureCents int64, currency string) error {
	if auth == nil {
		return newUnprocessable("AUTHORIZATION_NOT_FOUND", "authorization not found")
	}
	switch auth.Status {
	case StatusCreated, StatusPartiallyCaptured:
		// ok
	case StatusVoided:
		return newUnprocessable("AUTHORIZATION_VOIDED", "cannot capture a voided authorization")
	case StatusCaptured:
		return newUnprocessable("AUTHORIZATION_ALREADY_CAPTURED", "authorization is fully captured")
	case StatusDenied:
		return newUnprocessable("AUTHORIZATION_DENIED", "cannot capture a denied authorization")
	default:
		return newUnprocessable("INVALID_RESOURCE_STATE", "authorization cannot be captured in status "+auth.Status)
	}

	if currency != "" && currency != auth.Amount.CurrencyCode {
		return newUnprocessable("CURRENCY_MISMATCH", "capture currency must match authorization currency")
	}
	if captureCents <= 0 {
		return newInvalidRequest("INVALID_PARAMETER_VALUE", "capture amount must be positive", "amount.value")
	}

	authCents, err := ParseAmountCents(auth.Amount.Value)
	if err != nil {
		return newUnprocessable("INTERNAL_ERROR", "authorization has invalid amount")
	}
	remaining := authCents - auth.CapturedCents
	if captureCents > remaining {
		return newUnprocessable("MAX_CAPTURE_AMOUNT_EXCEEDED", "capture exceeds remaining authorized amount")
	}
	return nil
}

// ApplyCapture updates authorization bookkeeping after a successful capture.
func ApplyCapture(auth *Authorization, captureID string, captureCents int64) error {
	authCents, err := ParseAmountCents(auth.Amount.Value)
	if err != nil {
		return newUnprocessable("INTERNAL_ERROR", "authorization has invalid amount")
	}
	auth.CapturedCents += captureCents
	auth.CaptureIDs = append(auth.CaptureIDs, captureID)
	if auth.CapturedCents >= authCents {
		auth.Status = StatusCaptured
	} else {
		auth.Status = StatusPartiallyCaptured
	}
	return nil
}

// CanVoid reports whether an authorization can be voided.
func CanVoid(auth *Authorization) error {
	if auth == nil {
		return newUnprocessable("AUTHORIZATION_NOT_FOUND", "authorization not found")
	}
	if auth.Status == StatusVoided {
		return newUnprocessable("AUTHORIZATION_VOIDED", "authorization is already voided")
	}
	if len(auth.CaptureIDs) > 0 || auth.CapturedCents > 0 {
		return newUnprocessable("AUTHORIZATION_ALREADY_CAPTURED", "cannot void an authorization with captures")
	}
	if auth.Status != StatusCreated {
		return newUnprocessable("INVALID_RESOURCE_STATE", "authorization cannot be voided in status "+auth.Status)
	}
	return nil
}

// CanRefund reports whether a capture can accept a refund of refundCents.
func CanRefund(cap *Capture, refundCents int64, currency string) error {
	if cap == nil {
		return newUnprocessable("CAPTURE_NOT_FOUND", "capture not found")
	}
	switch cap.Status {
	case StatusCompleted, StatusPartiallyRefunded:
		// ok
	case StatusRefunded:
		return newUnprocessable("CAPTURE_FULLY_REFUNDED", "capture is already fully refunded")
	default:
		return newUnprocessable("INVALID_RESOURCE_STATE", "capture cannot be refunded in status "+cap.Status)
	}

	if currency != "" && currency != cap.Amount.CurrencyCode {
		return newUnprocessable("CURRENCY_MISMATCH", "refund currency must match capture currency")
	}
	if refundCents <= 0 {
		return newInvalidRequest("INVALID_PARAMETER_VALUE", "refund amount must be positive", "amount.value")
	}

	capCents, err := ParseAmountCents(cap.Amount.Value)
	if err != nil {
		return newUnprocessable("INTERNAL_ERROR", "capture has invalid amount")
	}
	remaining := capCents - cap.RefundedCents
	if refundCents > remaining {
		return newUnprocessable("REFUND_AMOUNT_EXCEEDED", "refund exceeds remaining captured amount")
	}
	return nil
}

// ApplyRefund updates capture bookkeeping after a successful refund.
func ApplyRefund(cap *Capture, refundID string, refundCents int64) error {
	capCents, err := ParseAmountCents(cap.Amount.Value)
	if err != nil {
		return newUnprocessable("INTERNAL_ERROR", "capture has invalid amount")
	}
	cap.RefundedCents += refundCents
	cap.RefundIDs = append(cap.RefundIDs, refundID)
	if cap.RefundedCents >= capCents {
		cap.Status = StatusRefunded
	} else {
		cap.Status = StatusPartiallyRefunded
	}
	return nil
}
