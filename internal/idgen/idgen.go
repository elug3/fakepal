package idgen

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync/atomic"
	"time"
)

var counter uint64

// NewID returns a PayPal-ish uppercase alphanumeric ID with an optional prefix.
func NewID(prefix string) string {
	n := atomic.AddUint64(&counter, 1)
	var b [6]byte
	_, _ = rand.Read(b[:])
	ts := time.Now().UTC().Format("150405")
	id := fmt.Sprintf("%s%06d%s", ts, n%1000000, hex.EncodeToString(b[:]))
	if prefix != "" {
		return prefix + id
	}
	return id
}
