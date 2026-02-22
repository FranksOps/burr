package proxy

import (
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPool_AddAndNext(t *testing.T) {
	pool := NewPool(Config{})

	// Add URLs, should add schemes if missing
	err := pool.Add("127.0.0.1:8080", "http://127.0.0.1:8081", "socks5://127.0.0.1:9050")
	if err != nil {
		t.Fatalf("unexpected error adding proxies: %v", err)
	}

	u1 := pool.Next()
	if u1 == nil || u1.String() != "http://127.0.0.1:8080" {
		t.Errorf("expected http://127.0.0.1:8080, got %v", u1)
	}

	u2 := pool.Next()
	if u2 == nil || u2.String() != "http://127.0.0.1:8081" {
		t.Errorf("expected http://127.0.0.1:8081, got %v", u2)
	}

	u3 := pool.Next()
	if u3 == nil || u3.String() != "socks5://127.0.0.1:9050" {
		t.Errorf("expected socks5://127.0.0.1:9050, got %v", u3)
	}

	u4 := pool.Next()
	if u4 == nil || u4.String() != "http://127.0.0.1:8080" {
		t.Errorf("expected http://127.0.0.1:8080 (wrap around), got %v", u4)
	}
}

func TestPool_HealthTracking(t *testing.T) {
	pool := NewPool(Config{
		MaxFailures: 2,
		Cooldown:    10 * time.Millisecond,
	})

	err := pool.Add("http://a", "http://b")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// a is next
	uA := pool.Next()
	if uA.String() != "http://a" {
		t.Fatalf("expected http://a, got %v", uA)
	}

	// mark A as failed twice
	pool.MarkFailure(uA)
	pool.MarkFailure(uA)

	// next should be b
	uB := pool.Next()
	if uB.String() != "http://b" {
		t.Fatalf("expected http://b, got %v", uB)
	}

	// next should still be b because a is cooling down
	uB2 := pool.Next()
	if uB2.String() != "http://b" {
		t.Fatalf("expected http://b, got %v", uB2)
	}

	// wait for a to cool down
	time.Sleep(15 * time.Millisecond)

	// next should be a again
	uA2 := pool.Next()
	if uA2.String() != "http://a" {
		t.Fatalf("expected http://a, got %v", uA2)
	}
}

func TestPool_AllDisabled(t *testing.T) {
	pool := NewPool(Config{
		MaxFailures: 1,
		Cooldown:    1 * time.Hour, // long cooldown
	})

	pool.Add("http://a")

	uA := pool.Next()
	pool.MarkFailure(uA)

	// None available
	if u := pool.Next(); u != nil {
		t.Errorf("expected nil when all proxies disabled, got %v", u)
	}
}

func TestPool_LoadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "proxies.txt")

	content := `
# some comment
http://proxy1.com
proxy2.com:80

socks5://proxy3.com:1080
`
	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to write proxy file: %v", err)
	}

	pool := NewPool(Config{})
	err = pool.LoadFile(path)
	if err != nil {
		t.Fatalf("failed to load file: %v", err)
	}

	var urls []string
	for i := 0; i < 3; i++ {
		u := pool.Next()
		if u == nil {
			t.Fatalf("expected proxy, got nil")
		}
		urls = append(urls, u.String())
	}

	expected := []string{"http://proxy1.com", "http://proxy2.com:80", "socks5://proxy3.com:1080"}
	for i, e := range expected {
		if urls[i] != e {
			t.Errorf("expected %s, got %s", e, urls[i])
		}
	}
}

func TestPool_MarkUnknown(t *testing.T) {
	pool := NewPool(Config{})
	pool.Add("http://a")

	uUnknown, _ := url.Parse("http://unknown")

	err := pool.MarkSuccess(uUnknown)
	if err == nil || err.Error() != "context: proxy not found in pool" {
		t.Errorf("expected error marking unknown proxy success, got %v", err)
	}

	err = pool.MarkFailure(uUnknown)
	if err == nil || err.Error() != "context: proxy not found in pool" {
		t.Errorf("expected error marking unknown proxy failure, got %v", err)
	}
}

func TestPool_Empty(t *testing.T) {
	pool := NewPool(Config{})
	if u := pool.Next(); u != nil {
		t.Errorf("expected nil on empty pool, got %v", u)
	}
}
