package fail_test

import (
	"testing"

	"github.com/elug3/fakepal/internal/domain"
	"github.com/elug3/fakepal/internal/fail"
)

func TestEvaluateHeuristic(t *testing.T) {
	d := fail.Evaluate("", domain.Amount{CurrencyCode: "USD", Value: "1.13"})
	if !d.Deny || d.Error == nil || len(d.Error.Details) == 0 {
		t.Fatalf("expected decline: %#v", d)
	}
	if d.Error.Details[0].Issue != "INSTRUMENT_DECLINED" {
		t.Fatalf("issue = %s", d.Error.Details[0].Issue)
	}
}

func TestEvaluateHeaderJSON(t *testing.T) {
	d := fail.Evaluate(`{"mock_application_codes":"DUPLICATE_INVOICE_ID"}`, domain.Amount{Value: "1.00"})
	if !d.Deny || d.Error.Details[0].Issue != "DUPLICATE_INVOICE_ID" {
		t.Fatalf("%#v", d)
	}
}

func TestEvaluateHeaderBare(t *testing.T) {
	d := fail.Evaluate("TRANSACTION_REFUSED", domain.Amount{Value: "1.00"})
	if !d.Deny || d.Error.Details[0].Issue != "TRANSACTION_REFUSED" {
		t.Fatalf("%#v", d)
	}
}

func TestEvaluateAllow(t *testing.T) {
	d := fail.Evaluate("", domain.Amount{Value: "1.00"})
	if d.Deny {
		t.Fatal("should allow")
	}
}
