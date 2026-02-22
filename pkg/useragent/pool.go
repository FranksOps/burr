package useragent

import (
	"crypto/rand"
	"math/big"
	"sync/atomic"
)

// DefaultPool provides a realistic set of modern User-Agents for desktop browsers.
var DefaultPool = []string{
	// Chrome Windows
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
	// Chrome Mac
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
	// Firefox Windows
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:122.0) Gecko/20100101 Firefox/122.0",
	// Firefox Mac
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:121.0) Gecko/20100101 Firefox/121.0",
	// Safari Mac
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Safari/605.1.15",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.3 Safari/605.1.15",
	// Edge Windows
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0",
}

// Pool represents a collection of User-Agents that can be retrieved sequentially or randomly.
type Pool struct {
	uas     []string
	counter atomic.Uint64
}

// NewPool creates a new User-Agent pool. If the provided slice is empty,
// it falls back to DefaultPool.
func NewPool(uas []string) *Pool {
	if len(uas) == 0 {
		uas = DefaultPool
	}
	// Copy to avoid external mutation
	copied := make([]string, len(uas))
	copy(copied, uas)
	return &Pool{
		uas: copied,
	}
}

// GetSequential returns the next User-Agent in the pool in a round-robin fashion.
// It is safe for concurrent use.
func (p *Pool) GetSequential() string {
	if len(p.uas) == 0 {
		return ""
	}
	idx := p.counter.Add(1) - 1
	return p.uas[idx%uint64(len(p.uas))]
}

// GetRandom returns a random User-Agent from the pool using crypto/rand.
// It is safe for concurrent use.
func (p *Pool) GetRandom() string {
	if len(p.uas) == 0 {
		return ""
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(p.uas))))
	if err != nil {
		// Fallback to sequential if crypto/rand fails
		return p.GetSequential()
	}
	return p.uas[n.Int64()]
}

// GetAll returns a copy of all User-Agents currently in the pool.
func (p *Pool) GetAll() []string {
	copied := make([]string, len(p.uas))
	copy(copied, p.uas)
	return copied
}
