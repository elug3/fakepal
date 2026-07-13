package store

import (
	"sync"

	"github.com/elug3/fakepal/internal/domain"
)

// Store persists payment resources.
type Store interface {
	SaveAuthorization(a *domain.Authorization) error
	GetAuthorization(id string) (*domain.Authorization, bool)
	SaveCapture(c *domain.Capture) error
	GetCapture(id string) (*domain.Capture, bool)
	SaveRefund(r *domain.Refund) error
	GetRefund(id string) (*domain.Refund, bool)

	// Idempotency maps a request key to a previously produced resource ID.
	GetIdempotent(key string) (resourceType, resourceID string, ok bool)
	PutIdempotent(key, resourceType, resourceID string)
}

// MemoryStore is a concurrency-safe in-memory Store.
type MemoryStore struct {
	mu      sync.RWMutex
	auths   map[string]*domain.Authorization
	caps    map[string]*domain.Capture
	refunds map[string]*domain.Refund
	idem    map[string]idemEntry
}

type idemEntry struct {
	Type string
	ID   string
}

// NewMemoryStore creates an empty memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		auths:   make(map[string]*domain.Authorization),
		caps:    make(map[string]*domain.Capture),
		refunds: make(map[string]*domain.Refund),
		idem:    make(map[string]idemEntry),
	}
}

func (s *MemoryStore) SaveAuthorization(a *domain.Authorization) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *a
	cp.CaptureIDs = append([]string(nil), a.CaptureIDs...)
	s.auths[a.ID] = &cp
	return nil
}

func (s *MemoryStore) GetAuthorization(id string) (*domain.Authorization, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a, ok := s.auths[id]
	if !ok {
		return nil, false
	}
	cp := *a
	cp.CaptureIDs = append([]string(nil), a.CaptureIDs...)
	return &cp, true
}

func (s *MemoryStore) SaveCapture(c *domain.Capture) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *c
	cp.RefundIDs = append([]string(nil), c.RefundIDs...)
	s.caps[c.ID] = &cp
	return nil
}

func (s *MemoryStore) GetCapture(id string) (*domain.Capture, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.caps[id]
	if !ok {
		return nil, false
	}
	cp := *c
	cp.RefundIDs = append([]string(nil), c.RefundIDs...)
	return &cp, true
}

func (s *MemoryStore) SaveRefund(r *domain.Refund) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *r
	s.refunds[r.ID] = &cp
	return nil
}

func (s *MemoryStore) GetRefund(id string) (*domain.Refund, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.refunds[id]
	if !ok {
		return nil, false
	}
	cp := *r
	return &cp, true
}

func (s *MemoryStore) GetIdempotent(key string) (string, string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.idem[key]
	if !ok {
		return "", "", false
	}
	return e.Type, e.ID, true
}

func (s *MemoryStore) PutIdempotent(key, resourceType, resourceID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.idem[key] = idemEntry{Type: resourceType, ID: resourceID}
}
