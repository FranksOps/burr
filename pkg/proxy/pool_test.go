package proxy

import (
	"testing"
)

func TestPool_Add_SchemeValidation(t *testing.T) {
	tests := []struct {
		name      string
		rawURL    string
		wantErr   bool
		errContains string
	}{
		{"valid http", "http://proxy.example.com:8080", false, ""},
		{"valid https", "https://proxy.example.com:8080", false, ""},
		{"valid socks5", "socks5://proxy.example.com:1080", false, ""},
		{"valid http without scheme", "proxy.example.com:8080", false, ""},
		{"invalid ftp", "ftp://proxy.example.com:8080", true, "unsupported proxy scheme"},
		{"invalid file", "file:///tmp/proxy", true, "unsupported proxy scheme"},
		{"invalid javascript", "javascript://proxy", true, "unsupported proxy scheme"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pLocal := NewPool(Config{MaxFailures: 1, Cooldown: 0})
			err := pLocal.Add(tt.rawURL)
			if (err != nil) != tt.wantErr {
				t.Errorf("Add() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil {
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("Add() error = %v, should contain %v", err, tt.errContains)
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
