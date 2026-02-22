package httpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClient_Timeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := Config{
		Timeout: 10 * time.Millisecond,
	}
	client, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req, _ := http.NewRequest(http.MethodGet, ts.URL, nil)
	_, err = client.Do(context.Background(), req)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestClient_Redirects(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/1" {
			http.Redirect(w, r, "/2", http.StatusFound)
			return
		}
		if r.URL.Path == "/2" {
			http.Redirect(w, r, "/3", http.StatusFound)
			return
		}
		if r.URL.Path == "/3" {
			w.WriteHeader(http.StatusOK)
			return
		}
	}))
	defer ts.Close()

	// Test default redirect limit
	cfg := Config{
		MaxRedirects: 1,
	}
	client, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/1", nil)
	_, err = client.Do(context.Background(), req)
	if err == nil {
		t.Fatal("expected redirect limit error")
	}

	// Test no redirects
	cfgNoRedir := Config{
		MaxRedirects: -1,
	}
	clientNoRedir, _ := New(cfgNoRedir)
	req2, _ := http.NewRequest(http.MethodGet, ts.URL+"/1", nil)
	resp, err := clientNoRedir.Do(context.Background(), req2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Errorf("expected 302 StatusFound, got %d", resp.StatusCode)
	}
}

func TestClient_Cookies(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/set" {
			http.SetCookie(w, &http.Cookie{Name: "burr", Value: "test"})
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path == "/check" {
			c, err := r.Cookie("burr")
			if err != nil || c.Value != "test" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		}
	}))
	defer ts.Close()

	cfg := Config{
		UseCookieJar: true,
	}
	client, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req1, _ := http.NewRequest(http.MethodGet, ts.URL+"/set", nil)
	resp1, err := client.Do(context.Background(), req1)
	if err != nil {
		t.Fatalf("unexpected error on /set: %v", err)
	}
	resp1.Body.Close()

	req2, _ := http.NewRequest(http.MethodGet, ts.URL+"/check", nil)
	resp2, err := client.Do(context.Background(), req2)
	if err != nil {
		t.Fatalf("unexpected error on /check: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK from /check, got %d. Cookies not persisted?", resp2.StatusCode)
	}
}

func TestClient_Context(t *testing.T) {
	cfg := Config{}
	client, _ := New(cfg)

	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)

	// Should fail with a typed error containing "context cannot be nil"
	_, err := client.Do(nil, req)
	if err == nil || err.Error() != "context: context cannot be nil" {
		t.Errorf("expected nil context error, got %v", err)
	}

	// Should honor context cancellation
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	req2, _ := http.NewRequest(http.MethodGet, ts.URL, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err = client.Do(ctx, req2)
	if err == nil {
		t.Fatal("expected cancellation error")
	}
}
