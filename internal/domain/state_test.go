package domain_test

import (
	"testing"

	"github.com/elug3/fakepal/internal/domain"
)

func TestParseAndFormatAmount(t *testing.T) {
	cents, err := domain.ParseAmountCents("10.00")
	if err != nil || cents != 1000 {
		t.Fatalf("ParseAmountCents(10.00) = %d, %v", cents, err)
	}
	cents, err = domain.ParseAmountCents("10.5")
	if err != nil || cents != 1050 {
		t.Fatalf("ParseAmountCents(10.5) = %d, %v", cents, err)
	}
	if got := domain.FormatCents(1050); got != "10.50" {
		t.Fatalf("FormatCents = %q", got)
	}
	if _, err := domain.ParseAmountCents("-1.00"); err == nil {
		t.Fatal("expected error for negative amount")
	}
}

func TestCaptureStateMachine(t *testing.T) {
	auth := &domain.Authorization{
		ID:     "a1",
		Status: domain.StatusCreated,
		Amount: domain.Amount{CurrencyCode: "USD", Value: "100.00"},
	}

	if err := domain.CanCapture(auth, 4000, "USD"); err != nil {
		t.Fatalf("partial capture should be allowed: %v", err)
	}
	if err := domain.ApplyCapture(auth, "c1", 4000); err != nil {
		t.Fatal(err)
	}
	if auth.Status != domain.StatusPartiallyCaptured {
		t.Fatalf("status = %s", auth.Status)
	}

	if err := domain.CanCapture(auth, 7000, "USD"); err == nil {
		t.Fatal("expected over-capture error")
	}
	if err := domain.CanCapture(auth, 6000, "EUR"); err == nil {
		t.Fatal("expected currency mismatch")
	}
	if err := domain.ApplyCapture(auth, "c2", 6000); err != nil {
		t.Fatal(err)
	}
	if auth.Status != domain.StatusCaptured {
		t.Fatalf("status = %s", auth.Status)
	}
	if err := domain.CanCapture(auth, 1, "USD"); err == nil {
		t.Fatal("expected fully captured error")
	}
}

func TestVoidRules(t *testing.T) {
	auth := &domain.Authorization{
		ID:     "a1",
		Status: domain.StatusCreated,
		Amount: domain.Amount{CurrencyCode: "USD", Value: "10.00"},
	}
	if err := domain.CanVoid(auth); err != nil {
		t.Fatalf("void should be allowed: %v", err)
	}
	_ = domain.ApplyCapture(auth, "c1", 1000)
	if err := domain.CanVoid(auth); err == nil {
		t.Fatal("cannot void after capture")
	}
}

func TestRefundStateMachine(t *testing.T) {
	cap := &domain.Capture{
		ID:     "c1",
		Status: domain.StatusCompleted,
		Amount: domain.Amount{CurrencyCode: "USD", Value: "50.00"},
	}
	if err := domain.CanRefund(cap, 2000, "USD"); err != nil {
		t.Fatal(err)
	}
	_ = domain.ApplyRefund(cap, "r1", 2000)
	if cap.Status != domain.StatusPartiallyRefunded {
		t.Fatalf("status = %s", cap.Status)
	}
	if err := domain.CanRefund(cap, 4000, "USD"); err == nil {
		t.Fatal("expected over-refund error")
	}
	_ = domain.ApplyRefund(cap, "r2", 3000)
	if cap.Status != domain.StatusRefunded {
		t.Fatalf("status = %s", cap.Status)
	}
	if err := domain.CanRefund(cap, 1, "USD"); err == nil {
		t.Fatal("expected fully refunded error")
	}
}

func TestValidateCreateAuthorization(t *testing.T) {
	if err := domain.ValidateCreateAuthorization(domain.Amount{CurrencyCode: "USD", Value: "1.00"}); err != nil {
		t.Fatal(err)
	}
	if err := domain.ValidateCreateAuthorization(domain.Amount{CurrencyCode: "", Value: "1.00"}); err == nil {
		t.Fatal("expected missing currency error")
	}
	if err := domain.ValidateCreateAuthorization(domain.Amount{CurrencyCode: "USD", Value: "0.00"}); err == nil {
		t.Fatal("expected non-positive amount error")
	}
}
