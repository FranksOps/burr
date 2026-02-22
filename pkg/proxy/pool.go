package proxy

import (
	"bufio"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// Proxy represents a single proxy endpoint with health tracking.
type Proxy struct {
	URL           *url.URL
	Failures      int
	Successes     int
	LastUsed      time.Time
	Disabled      bool
	DisabledUntil time.Time
}

// Pool manages a collection of proxies.
type Pool struct {
	mu           sync.Mutex
	proxies      []*Proxy
	currentIndex int
	maxFailures  int
	cooldown     time.Duration
}

// Config defines settings for the Proxy Pool.
type Config struct {
	// MaxFailures before disabling a proxy temporarily.
	MaxFailures int
	// Cooldown is how long a proxy remains disabled after hitting MaxFailures.
	Cooldown time.Duration
}

// NewPool creates a new proxy pool. If config values are zero, reasonable defaults are used.
func NewPool(cfg Config) *Pool {
	if cfg.MaxFailures <= 0 {
		cfg.MaxFailures = 3
	}
	if cfg.Cooldown <= 0 {
		cfg.Cooldown = 5 * time.Minute
	}
	return &Pool{
		maxFailures: cfg.MaxFailures,
		cooldown:    cfg.Cooldown,
	}
}

// LoadFile reads proxies from a file, expecting one URL per line.
// Lines starting with '#' or empty lines are ignored.
func (p *Pool) LoadFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("context: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var urls []string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		urls = append(urls, line)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("context: %w", err)
	}

	return p.Add(urls...)
}

// Add parses raw URL strings and adds them to the pool.
func (p *Pool) Add(rawURLs ...string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, raw := range rawURLs {
		if !strings.Contains(raw, "://") {
			// default to http if scheme is missing
			raw = "http://" + raw
		}
		u, err := url.Parse(raw)
		if err != nil {
			return fmt.Errorf("context: %w", err)
		}
		p.proxies = append(p.proxies, &Proxy{
			URL: u,
		})
	}
	return nil
}

// Next returns the next healthy proxy URL in the pool. It returns nil if no proxies
// are available or if all proxies are currently cooling down.
func (p *Pool) Next() *url.URL {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.proxies) == 0 {
		return nil
	}

	now := time.Now()
	startIndex := p.currentIndex

	for {
		prx := p.proxies[p.currentIndex]

		// Advance index
		p.currentIndex = (p.currentIndex + 1) % len(p.proxies)

		// Check if it's eligible to be re-enabled
		if prx.Disabled && now.After(prx.DisabledUntil) {
			prx.Disabled = false
			prx.Failures = 0 // reset failures on revival
		}

		if !prx.Disabled {
			prx.LastUsed = now
			return prx.URL
		}

		// If we looped all the way around, no proxies are healthy
		if p.currentIndex == startIndex {
			return nil
		}
	}
}

// MarkSuccess records a successful request for the given proxy URL.
func (p *Pool) MarkSuccess(proxyURL *url.URL) error {
	if proxyURL == nil {
		return errors.New("context: proxyURL cannot be nil")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	prx := p.findProxy(proxyURL)
	if prx == nil {
		return errors.New("context: proxy not found in pool")
	}

	prx.Successes++
	// Only decrease failures but don't go below 0
	if prx.Failures > 0 {
		prx.Failures--
	}
	return nil
}

// MarkFailure records a failure for the given proxy URL. If failures exceed
// the configured maximum, the proxy is temporarily disabled.
func (p *Pool) MarkFailure(proxyURL *url.URL) error {
	if proxyURL == nil {
		return errors.New("context: proxyURL cannot be nil")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	prx := p.findProxy(proxyURL)
	if prx == nil {
		return errors.New("context: proxy not found in pool")
	}

	prx.Failures++
	if prx.Failures >= p.maxFailures {
		prx.Disabled = true
		prx.DisabledUntil = time.Now().Add(p.cooldown)
	}
	return nil
}

// findProxy locates a proxy by its String() representation. Must be called with lock held.
func (p *Pool) findProxy(u *url.URL) *Proxy {
	target := u.String()
	for _, prx := range p.proxies {
		if prx.URL.String() == target {
			return prx
		}
	}
	return nil
}
