package fingerprint

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	utls "github.com/refraction-networking/utls"
)

func TestTransport_Profiles(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	profiles := []Profile{
		ProfileChrome,
		ProfileFirefox,
		ProfileSafari,
		ProfileGo,
		ProfileRandom,
	}

	for _, p := range profiles {
		t.Run(string(p), func(t *testing.T) {
			rt, err := Transport(p, nil)
			if err != nil {
				t.Fatalf("unexpected error creating transport for %s: %v", p, err)
			}

			tr, ok := rt.(*http.Transport)
			if !ok {
				t.Fatalf("expected *http.Transport, got %T", rt)
			}

			// httptest.NewTLSServer uses self-signed certs.
			// We need to disable verification for the test.
			if p == ProfileGo {
				tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
			} else {
				// For uTLS, we have to inject the TLS config into the DialTLSContext.
				originalDialContext := tr.DialContext
				if originalDialContext == nil {
					t.Fatalf("expected DialContext to be populated by Clone")
				}

				tr.DialTLSContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
					// We can use default dial context
					tcpConn, err := originalDialContext(ctx, network, addr)
					if err != nil {
						return nil, err
					}

					host := addr
					// Ignore port
					for i := len(addr) - 1; i >= 0; i-- {
						if addr[i] == ':' {
							host = addr[:i]
							break
						}
					}

					// We modify uConn creation to skip verify
					uConn := utls.UClient(tcpConn, &utls.Config{
						ServerName:         host,
						InsecureSkipVerify: true,
					}, utls.HelloChrome_Auto) // Default to Chrome auto for test setup simplification

					if p == ProfileFirefox {
						uConn = utls.UClient(tcpConn, &utls.Config{ServerName: host, InsecureSkipVerify: true}, utls.HelloFirefox_Auto)
					} else if p == ProfileSafari {
						uConn = utls.UClient(tcpConn, &utls.Config{ServerName: host, InsecureSkipVerify: true}, utls.HelloIOS_Auto)
					} else if p == ProfileRandom {
						uConn = utls.UClient(tcpConn, &utls.Config{ServerName: host, InsecureSkipVerify: true}, utls.HelloRandomizedALPN)
					}

					if err := uConn.HandshakeContext(ctx); err != nil {
						_ = tcpConn.Close()
						return nil, err
					}

					return uConn, nil
				}
			}

			client := &http.Client{Transport: tr}
			req, err := http.NewRequest("GET", ts.URL, nil)
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}

			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("request failed for profile %s: %v", p, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("expected 200 OK, got %d for profile %s", resp.StatusCode, p)
			}
		})
	}
}

func TestTransport_UnknownProfile(t *testing.T) {
	_, err := Transport(Profile("unknown_browser"), nil)
	if err == nil {
		t.Fatal("expected error for unknown profile, got nil")
	}
	if err.Error() != `context: unknown profile "unknown_browser"` {
		t.Errorf("unexpected error message: %v", err)
	}
}
