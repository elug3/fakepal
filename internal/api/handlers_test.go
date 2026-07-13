package api_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/elug3/fakepal/internal/api"
	"github.com/elug3/fakepal/internal/auth"
	"github.com/elug3/fakepal/internal/domain"
	"github.com/elug3/fakepal/internal/store"
)

func newTestServer() http.Handler {
	s := api.NewServer(store.NewMemoryStore(), "http://example.test")
	return auth.Middleware("test-api-key")(s.Handler())
}

func doJSON(t *testing.T, h http.Handler, method, path, key string, body any, headers map[string]string) (int, map[string]any) {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		rdr = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	var out map[string]any
	if rr.Body.Len() > 0 && rr.Header().Get("Content-Type") == "application/json" {
		if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
			// void returns empty; some errors may still be JSON objects
			_ = err
		}
	}
	return rr.Code, out
}

func TestHealthzNoAuth(t *testing.T) {
	h := newTestServer()
	code, body := doJSON(t, h, http.MethodGet, "/healthz", "", nil, nil)
	if code != http.StatusOK || body["status"] != "ok" {
		t.Fatalf("healthz: %d %#v", code, body)
	}
}

func TestUnauthorized(t *testing.T) {
	h := newTestServer()
	code, _ := doJSON(t, h, http.MethodPost, "/v2/payments/authorizations", "bad", map[string]any{
		"amount": map[string]string{"currency_code": "USD", "value": "10.00"},
	}, nil)
	if code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", code)
	}
}

func TestFullPaymentFlow(t *testing.T) {
	h := newTestServer()
	key := "test-api-key"

	code, authBody := doJSON(t, h, http.MethodPost, "/v2/payments/authorizations", key, map[string]any{
		"amount": map[string]string{"currency_code": "USD", "value": "100.00"},
	}, nil)
	if code != http.StatusCreated {
		t.Fatalf("create auth: %d %#v", code, authBody)
	}
	authID, _ := authBody["id"].(string)
	if authBody["status"] != domain.StatusCreated || authID == "" {
		t.Fatalf("bad auth: %#v", authBody)
	}

	code, capBody := doJSON(t, h, http.MethodPost, "/v2/payments/authorizations/"+authID+"/capture", key, map[string]any{
		"amount": map[string]string{"currency_code": "USD", "value": "40.00"},
	}, nil)
	if code != http.StatusCreated || capBody["status"] != domain.StatusCompleted {
		t.Fatalf("capture: %d %#v", code, capBody)
	}
	capID, _ := capBody["id"].(string)

	code, gotAuth := doJSON(t, h, http.MethodGet, "/v2/payments/authorizations/"+authID, key, nil, nil)
	if code != http.StatusOK || gotAuth["status"] != domain.StatusPartiallyCaptured {
		t.Fatalf("get auth after partial: %d %#v", code, gotAuth)
	}

	code, voidBody := doJSON(t, h, http.MethodPost, "/v2/payments/authorizations/"+authID+"/void", key, nil, nil)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("void after capture should fail: %d %#v", code, voidBody)
	}

	code, refBody := doJSON(t, h, http.MethodPost, "/v2/payments/captures/"+capID+"/refund", key, map[string]any{
		"amount": map[string]string{"currency_code": "USD", "value": "10.00"},
	}, nil)
	if code != http.StatusCreated || refBody["status"] != domain.StatusCompleted {
		t.Fatalf("refund: %d %#v", code, refBody)
	}
	refID, _ := refBody["id"].(string)

	code, gotCap := doJSON(t, h, http.MethodGet, "/v2/payments/captures/"+capID, key, nil, nil)
	if code != http.StatusOK || gotCap["status"] != domain.StatusPartiallyRefunded {
		t.Fatalf("get capture: %d %#v", code, gotCap)
	}

	code, gotRef := doJSON(t, h, http.MethodGet, "/v2/payments/refunds/"+refID, key, nil, nil)
	if code != http.StatusOK || gotRef["id"] != refID {
		t.Fatalf("get refund: %d %#v", code, gotRef)
	}
}

func TestVoidHappyPath(t *testing.T) {
	h := newTestServer()
	key := "test-api-key"
	_, authBody := doJSON(t, h, http.MethodPost, "/v2/payments/authorizations", key, map[string]any{
		"amount": map[string]string{"currency_code": "USD", "value": "25.00"},
	}, nil)
	authID := authBody["id"].(string)

	code, _ := doJSON(t, h, http.MethodPost, "/v2/payments/authorizations/"+authID+"/void", key, nil, nil)
	if code != http.StatusNoContent {
		t.Fatalf("void: %d", code)
	}
	code, got := doJSON(t, h, http.MethodGet, "/v2/payments/authorizations/"+authID, key, nil, nil)
	if code != http.StatusOK || got["status"] != domain.StatusVoided {
		t.Fatalf("voided auth: %d %#v", code, got)
	}
}

func TestFullCaptureAndFullRefund(t *testing.T) {
	h := newTestServer()
	key := "test-api-key"
	_, authBody := doJSON(t, h, http.MethodPost, "/v2/payments/authorizations", key, map[string]any{
		"amount": map[string]string{"currency_code": "USD", "value": "12.34"},
	}, nil)
	authID := authBody["id"].(string)

	code, capBody := doJSON(t, h, http.MethodPost, "/v2/payments/authorizations/"+authID+"/capture", key, nil, nil)
	if code != http.StatusCreated {
		t.Fatalf("full capture: %d %#v", code, capBody)
	}
	capID := capBody["id"].(string)
	amt := capBody["amount"].(map[string]any)
	if amt["value"] != "12.34" {
		t.Fatalf("capture amount = %#v", amt)
	}

	code, gotAuth := doJSON(t, h, http.MethodGet, "/v2/payments/authorizations/"+authID, key, nil, nil)
	if gotAuth["status"] != domain.StatusCaptured {
		t.Fatalf("expected CAPTURED: %#v", gotAuth)
	}

	code, refBody := doJSON(t, h, http.MethodPost, "/v2/payments/captures/"+capID+"/refund", key, nil, nil)
	if code != http.StatusCreated {
		t.Fatalf("full refund: %d %#v", code, refBody)
	}
	code, gotCap := doJSON(t, h, http.MethodGet, "/v2/payments/captures/"+capID, key, nil, nil)
	if gotCap["status"] != domain.StatusRefunded {
		t.Fatalf("expected REFUNDED: %#v", gotCap)
	}
}

func TestFailureInjection(t *testing.T) {
	h := newTestServer()
	key := "test-api-key"

	code, body := doJSON(t, h, http.MethodPost, "/v2/payments/authorizations", key, map[string]any{
		"amount": map[string]string{"currency_code": "USD", "value": "10.13"},
	}, nil)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf(".13 heuristic: %d %#v", code, body)
	}

	code, body = doJSON(t, h, http.MethodPost, "/v2/payments/authorizations", key, map[string]any{
		"amount": map[string]string{"currency_code": "USD", "value": "10.00"},
	}, map[string]string{
		"PayPal-Mock-Response": `{"mock_application_codes":"INSTRUMENT_DECLINED"}`,
	})
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("mock header: %d %#v", code, body)
	}
	details, _ := body["details"].([]any)
	if len(details) == 0 {
		t.Fatalf("expected details: %#v", body)
	}
}

func TestIdempotentCapture(t *testing.T) {
	h := newTestServer()
	key := "test-api-key"
	_, authBody := doJSON(t, h, http.MethodPost, "/v2/payments/authorizations", key, map[string]any{
		"amount": map[string]string{"currency_code": "USD", "value": "20.00"},
	}, nil)
	authID := authBody["id"].(string)

	headers := map[string]string{"PayPal-Request-Id": "idem-1"}
	code1, cap1 := doJSON(t, h, http.MethodPost, "/v2/payments/authorizations/"+authID+"/capture", key, map[string]any{
		"amount": map[string]string{"currency_code": "USD", "value": "5.00"},
	}, headers)
	code2, cap2 := doJSON(t, h, http.MethodPost, "/v2/payments/authorizations/"+authID+"/capture", key, map[string]any{
		"amount": map[string]string{"currency_code": "USD", "value": "5.00"},
	}, headers)
	if code1 != http.StatusCreated || code2 != http.StatusCreated {
		t.Fatalf("codes %d %d", code1, code2)
	}
	if cap1["id"] != cap2["id"] {
		t.Fatalf("idempotent capture IDs differ: %v vs %v", cap1["id"], cap2["id"])
	}

	// Remaining should still allow another distinct capture.
	code3, cap3 := doJSON(t, h, http.MethodPost, "/v2/payments/authorizations/"+authID+"/capture", key, map[string]any{
		"amount": map[string]string{"currency_code": "USD", "value": "5.00"},
	}, map[string]string{"PayPal-Request-Id": "idem-2"})
	if code3 != http.StatusCreated || cap3["id"] == cap1["id"] {
		t.Fatalf("second capture: %d %#v", code3, cap3)
	}
}

func TestNotFound(t *testing.T) {
	h := newTestServer()
	key := "test-api-key"
	code, _ := doJSON(t, h, http.MethodGet, "/v2/payments/authorizations/missing", key, nil, nil)
	if code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", code)
	}
}
