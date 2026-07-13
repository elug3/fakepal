package store_test

import (
	"sync"
	"testing"
	"time"

	"github.com/elug3/fakepal/internal/domain"
	"github.com/elug3/fakepal/internal/store"
)

func TestMemoryStoreCRUD(t *testing.T) {
	s := store.NewMemoryStore()
	now := time.Now().UTC()
	auth := &domain.Authorization{
		ID: "a1", Status: domain.StatusCreated,
		Amount: domain.Amount{CurrencyCode: "USD", Value: "10.00"},
		CreateTime: now, UpdateTime: now,
	}
	if err := s.SaveAuthorization(auth); err != nil {
		t.Fatal(err)
	}
	got, ok := s.GetAuthorization("a1")
	if !ok || got.Status != domain.StatusCreated {
		t.Fatalf("get auth failed: %#v", got)
	}

	cap := &domain.Capture{
		ID: "c1", Status: domain.StatusCompleted, AuthorizationID: "a1",
		Amount: domain.Amount{CurrencyCode: "USD", Value: "10.00"},
		CreateTime: now, UpdateTime: now,
	}
	_ = s.SaveCapture(cap)
	if _, ok := s.GetCapture("c1"); !ok {
		t.Fatal("missing capture")
	}

	ref := &domain.Refund{
		ID: "r1", Status: domain.StatusCompleted, CaptureID: "c1",
		Amount: domain.Amount{CurrencyCode: "USD", Value: "5.00"},
		CreateTime: now, UpdateTime: now,
	}
	_ = s.SaveRefund(ref)
	if _, ok := s.GetRefund("r1"); !ok {
		t.Fatal("missing refund")
	}

	s.PutIdempotent("k1", "capture", "c1")
	typ, id, ok := s.GetIdempotent("k1")
	if !ok || typ != "capture" || id != "c1" {
		t.Fatalf("idempotent mismatch: %s %s %v", typ, id, ok)
	}
}

func TestMemoryStoreConcurrent(t *testing.T) {
	s := store.NewMemoryStore()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := string(rune('A'+(i%26))) + string(rune('0'+i%10))
			_ = s.SaveAuthorization(&domain.Authorization{
				ID: id, Status: domain.StatusCreated,
				Amount: domain.Amount{CurrencyCode: "USD", Value: "1.00"},
				CreateTime: time.Now().UTC(), UpdateTime: time.Now().UTC(),
			})
			_, _ = s.GetAuthorization(id)
		}(i)
	}
	wg.Wait()
}
