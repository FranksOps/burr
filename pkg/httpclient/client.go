package httpclient

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"time"
)

// Config defines the setup for the HTTP Client.
type Config struct {
	Timeout      time.Duration
	MaxRedirects int
	UseCookieJar bool
	// Provide a custom Transport, e.g. for proxies or uTLS fingerprinting
	Transport http.RoundTripper
}

// Client wraps a standard http.Client to provide configurable timeouts,
// redirect policies, and cookie management.
type Client struct {
	*http.Client
}

// New creates a new HTTP client based on the provided configuration.
func New(cfg Config) (*Client, error) {
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	c := &http.Client{
		Timeout: cfg.Timeout,
	}

	// Setup custom redirect policy
	if cfg.MaxRedirects >= 0 {
		c.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			if len(via) >= cfg.MaxRedirects {
				return fmt.Errorf("context: stopped after %d redirects", cfg.MaxRedirects)
			}
			return nil
		}
	} else {
		// Don't follow any redirects if max < 0
		c.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	// Cookie jar persistence
	if cfg.UseCookieJar {
		jar, err := cookiejar.New(nil)
		if err != nil {
			return nil, fmt.Errorf("context: %w", err)
		}
		c.Jar = jar
	}

	if cfg.Transport != nil {
		c.Transport = cfg.Transport
	}

	return &Client{Client: c}, nil
}

// Do executes an HTTP request. The provided context.Context should control
// the overarching request timeout/cancellation independent of the client timeout.
func (c *Client) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	if ctx == nil {
		return nil, errors.New("context: context cannot be nil")
	}

	// Always clone the request with the provided context
	reqWithCtx := req.Clone(ctx)

	resp, err := c.Client.Do(reqWithCtx)
	if err != nil {
		return nil, fmt.Errorf("context: %w", err)
	}
	return resp, nil
}
