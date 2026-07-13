package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/elug3/fakepal/internal/domain"
	"github.com/elug3/fakepal/internal/fail"
	"github.com/elug3/fakepal/internal/idgen"
	"github.com/elug3/fakepal/internal/store"
)

// Server wires HTTP handlers to the store.
type Server struct {
	Store   store.Store
	BaseURL string
}

// NewServer constructs an API server.
func NewServer(s store.Store, baseURL string) *Server {
	return &Server{Store: s, BaseURL: strings.TrimRight(baseURL, "/")}
}

// Handler returns the root HTTP handler with all routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("POST /v2/payments/authorizations", s.handleCreateAuthorization)
	mux.HandleFunc("GET /v2/payments/authorizations/{id}", s.handleGetAuthorization)
	mux.HandleFunc("POST /v2/payments/authorizations/{id}/capture", s.handleCapture)
	mux.HandleFunc("POST /v2/payments/authorizations/{id}/void", s.handleVoid)
	mux.HandleFunc("GET /v2/payments/captures/{id}", s.handleGetCapture)
	mux.HandleFunc("POST /v2/payments/captures/{id}/refund", s.handleRefund)
	mux.HandleFunc("GET /v2/payments/refunds/{id}", s.handleGetRefund)
	return mux
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type amountBody struct {
	Amount *domain.Amount `json:"amount"`
}

func (s *Server) handleCreateAuthorization(w http.ResponseWriter, r *http.Request) {
	var body amountBody
	if err := decodeJSON(r, &body); err != nil {
		writeDomainError(w, http.StatusBadRequest, &domain.DomainError{
			Name: "INVALID_REQUEST", Message: "Request is not well-formed, syntactically incorrect, or violates schema.",
			Issue: "INVALID_JSON",
		})
		return
	}
	if body.Amount == nil {
		writeDomainError(w, http.StatusBadRequest, &domain.DomainError{
			Name: "INVALID_REQUEST", Message: "amount is required", Issue: "MISSING_REQUIRED_PARAMETER", Field: "amount",
		})
		return
	}
	if err := domain.ValidateCreateAuthorization(*body.Amount); err != nil {
		writeErr(w, err)
		return
	}

	if d := fail.Evaluate(r.Header.Get(fail.HeaderMockResponse), *body.Amount); d.Deny {
		writeJSON(w, http.StatusUnprocessableEntity, d.Error)
		return
	}

	now := time.Now().UTC()
	id := idgen.NewID("AUTH-")
	auth := &domain.Authorization{
		ID:         id,
		Status:     domain.StatusCreated,
		Amount:     *body.Amount,
		CreateTime: now,
		UpdateTime: now,
		Links: []domain.Link{
			{Href: s.BaseURL + "/v2/payments/authorizations/" + id, Rel: "self", Method: "GET"},
			{Href: s.BaseURL + "/v2/payments/authorizations/" + id + "/capture", Rel: "capture", Method: "POST"},
			{Href: s.BaseURL + "/v2/payments/authorizations/" + id + "/void", Rel: "void", Method: "POST"},
		},
	}
	_ = s.Store.SaveAuthorization(auth)
	writeJSON(w, http.StatusCreated, auth)
}

func (s *Server) handleGetAuthorization(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	auth, ok := s.Store.GetAuthorization(id)
	if !ok {
		writeNotFound(w, "AUTHORIZATION_NOT_FOUND", "The specified resource does not exist.")
		return
	}
	writeJSON(w, http.StatusOK, auth)
}

func (s *Server) handleCapture(w http.ResponseWriter, r *http.Request) {
	authID := r.PathValue("id")
	idemKey := strings.TrimSpace(r.Header.Get("PayPal-Request-Id"))
	if idemKey != "" {
		if typ, rid, ok := s.Store.GetIdempotent("capture:" + idemKey); ok && typ == "capture" {
			if cap, found := s.Store.GetCapture(rid); found {
				writeJSON(w, http.StatusCreated, cap)
				return
			}
		}
	}

	auth, ok := s.Store.GetAuthorization(authID)
	if !ok {
		writeNotFound(w, "AUTHORIZATION_NOT_FOUND", "The specified resource does not exist.")
		return
	}

	var body amountBody
	_ = decodeJSON(r, &body) // empty body = full remaining capture

	var captureAmount domain.Amount
	var captureCents int64
	authCents, _ := domain.ParseAmountCents(auth.Amount.Value)
	remaining := authCents - auth.CapturedCents

	if body.Amount == nil {
		captureCents = remaining
		captureAmount = domain.Amount{
			CurrencyCode: auth.Amount.CurrencyCode,
			Value:        domain.FormatCents(captureCents),
		}
	} else {
		captureAmount = *body.Amount
		if captureAmount.CurrencyCode == "" {
			captureAmount.CurrencyCode = auth.Amount.CurrencyCode
		}
		var err error
		captureCents, err = domain.ParseAmountCents(captureAmount.Value)
		if err != nil {
			writeDomainError(w, http.StatusBadRequest, &domain.DomainError{
				Name: "INVALID_REQUEST", Message: "invalid amount.value", Issue: "INVALID_PARAMETER_VALUE", Field: "amount.value",
			})
			return
		}
	}

	if d := fail.Evaluate(r.Header.Get(fail.HeaderMockResponse), captureAmount); d.Deny {
		writeJSON(w, http.StatusUnprocessableEntity, d.Error)
		return
	}

	if err := domain.CanCapture(auth, captureCents, captureAmount.CurrencyCode); err != nil {
		writeErr(w, err)
		return
	}

	now := time.Now().UTC()
	capID := idgen.NewID("CAP-")
	cap := &domain.Capture{
		ID:              capID,
		Status:          domain.StatusCompleted,
		Amount:          captureAmount,
		AuthorizationID: authID,
		CreateTime:      now,
		UpdateTime:      now,
		Links: []domain.Link{
			{Href: s.BaseURL + "/v2/payments/captures/" + capID, Rel: "self", Method: "GET"},
			{Href: s.BaseURL + "/v2/payments/captures/" + capID + "/refund", Rel: "refund", Method: "POST"},
			{Href: s.BaseURL + "/v2/payments/authorizations/" + authID, Rel: "up", Method: "GET"},
		},
	}
	_ = domain.ApplyCapture(auth, capID, captureCents)
	auth.UpdateTime = now
	_ = s.Store.SaveAuthorization(auth)
	_ = s.Store.SaveCapture(cap)
	if idemKey != "" {
		s.Store.PutIdempotent("capture:"+idemKey, "capture", capID)
	}
	writeJSON(w, http.StatusCreated, cap)
}

func (s *Server) handleVoid(w http.ResponseWriter, r *http.Request) {
	authID := r.PathValue("id")
	auth, ok := s.Store.GetAuthorization(authID)
	if !ok {
		writeNotFound(w, "AUTHORIZATION_NOT_FOUND", "The specified resource does not exist.")
		return
	}
	if err := domain.CanVoid(auth); err != nil {
		writeErr(w, err)
		return
	}
	auth.Status = domain.StatusVoided
	auth.UpdateTime = time.Now().UTC()
	_ = s.Store.SaveAuthorization(auth)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleGetCapture(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	cap, ok := s.Store.GetCapture(id)
	if !ok {
		writeNotFound(w, "CAPTURE_NOT_FOUND", "The specified resource does not exist.")
		return
	}
	writeJSON(w, http.StatusOK, cap)
}

func (s *Server) handleRefund(w http.ResponseWriter, r *http.Request) {
	capID := r.PathValue("id")
	idemKey := strings.TrimSpace(r.Header.Get("PayPal-Request-Id"))
	if idemKey != "" {
		if typ, rid, ok := s.Store.GetIdempotent("refund:" + idemKey); ok && typ == "refund" {
			if ref, found := s.Store.GetRefund(rid); found {
				writeJSON(w, http.StatusCreated, ref)
				return
			}
		}
	}

	cap, ok := s.Store.GetCapture(capID)
	if !ok {
		writeNotFound(w, "CAPTURE_NOT_FOUND", "The specified resource does not exist.")
		return
	}

	var body amountBody
	_ = decodeJSON(r, &body)

	var refundAmount domain.Amount
	var refundCents int64
	capCents, _ := domain.ParseAmountCents(cap.Amount.Value)
	remaining := capCents - cap.RefundedCents

	if body.Amount == nil {
		refundCents = remaining
		refundAmount = domain.Amount{
			CurrencyCode: cap.Amount.CurrencyCode,
			Value:        domain.FormatCents(refundCents),
		}
	} else {
		refundAmount = *body.Amount
		if refundAmount.CurrencyCode == "" {
			refundAmount.CurrencyCode = cap.Amount.CurrencyCode
		}
		var err error
		refundCents, err = domain.ParseAmountCents(refundAmount.Value)
		if err != nil {
			writeDomainError(w, http.StatusBadRequest, &domain.DomainError{
				Name: "INVALID_REQUEST", Message: "invalid amount.value", Issue: "INVALID_PARAMETER_VALUE", Field: "amount.value",
			})
			return
		}
	}

	if d := fail.Evaluate(r.Header.Get(fail.HeaderMockResponse), refundAmount); d.Deny {
		writeJSON(w, http.StatusUnprocessableEntity, d.Error)
		return
	}

	if err := domain.CanRefund(cap, refundCents, refundAmount.CurrencyCode); err != nil {
		writeErr(w, err)
		return
	}

	now := time.Now().UTC()
	refID := idgen.NewID("REF-")
	ref := &domain.Refund{
		ID:         refID,
		Status:     domain.StatusCompleted,
		Amount:     refundAmount,
		CaptureID:  capID,
		CreateTime: now,
		UpdateTime: now,
		Links: []domain.Link{
			{Href: s.BaseURL + "/v2/payments/refunds/" + refID, Rel: "self", Method: "GET"},
			{Href: s.BaseURL + "/v2/payments/captures/" + capID, Rel: "up", Method: "GET"},
		},
	}
	_ = domain.ApplyRefund(cap, refID, refundCents)
	cap.UpdateTime = now
	_ = s.Store.SaveCapture(cap)
	_ = s.Store.SaveRefund(ref)
	if idemKey != "" {
		s.Store.PutIdempotent("refund:"+idemKey, "refund", refID)
	}
	writeJSON(w, http.StatusCreated, ref)
}

func (s *Server) handleGetRefund(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ref, ok := s.Store.GetRefund(id)
	if !ok {
		writeNotFound(w, "REFUND_NOT_FOUND", "The specified resource does not exist.")
		return
	}
	writeJSON(w, http.StatusOK, ref)
}

func decodeJSON(r *http.Request, dst any) error {
	if r.Body == nil {
		return nil
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	err := dec.Decode(dst)
	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeNotFound(w http.ResponseWriter, name, message string) {
	writeJSON(w, http.StatusNotFound, domain.ErrorResponse{Name: name, Message: message})
}

func writeDomainError(w http.ResponseWriter, status int, err *domain.DomainError) {
	resp := domain.ErrorResponse{
		Name:    err.Name,
		Message: err.Message,
	}
	if err.Issue != "" || err.Field != "" {
		resp.Details = []domain.ErrorDetail{{
			Issue:       err.Issue,
			Description: err.Message,
			Field:       err.Field,
		}}
	}
	writeJSON(w, status, resp)
}

func writeErr(w http.ResponseWriter, err error) {
	if de, ok := err.(*domain.DomainError); ok {
		status := http.StatusUnprocessableEntity
		if de.Name == "INVALID_REQUEST" {
			status = http.StatusBadRequest
		}
		writeDomainError(w, status, de)
		return
	}
	writeJSON(w, http.StatusInternalServerError, domain.ErrorResponse{
		Name:    "INTERNAL_SERVER_ERROR",
		Message: err.Error(),
	})
}
