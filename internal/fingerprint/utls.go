package fingerprint

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"

	utls "github.com/refraction-networking/utls"
)

// Profile represents a recognized TLS fingerprint profile.
type Profile string

const (
	ProfileChrome  Profile = "chrome"
	ProfileFirefox Profile = "firefox"
	ProfileSafari  Profile = "safari"
	ProfileGo      Profile = "go"     // standard go TLS
	ProfileRandom  Profile = "random" // randomized uTLS profile
)

// Transport returns an http.RoundTripper configured with the specified
// TLS fingerprint profile. If the profile is "go", it returns a standard
// http.Transport. Otherwise, it wraps http.Transport to use utls.UClient.
// proxyFunc is optional. If provided, it configures the underlying transport's Proxy.
func Transport(p Profile, proxyFunc func(*http.Request) (*url.URL, error)) (http.RoundTripper, error) {
	if p == ProfileGo {
		// Standard Go standard library Transport
		transport := http.DefaultTransport.(*http.Transport).Clone()
		if proxyFunc != nil {
			transport.Proxy = proxyFunc
		}
		return transport, nil
	}

	var clientHelloID utls.ClientHelloID
	switch p {
	case ProfileChrome:
		clientHelloID = utls.HelloChrome_Auto
	case ProfileFirefox:
		clientHelloID = utls.HelloFirefox_Auto
	case ProfileSafari:
		clientHelloID = utls.HelloIOS_Auto
	case ProfileRandom:
		clientHelloID = utls.HelloRandomizedALPN
	default:
		return nil, fmt.Errorf("context: unknown profile %q", p)
	}

	// We create a custom DialTLSContext function that wraps the standard TCP dialer
	// and then performs the uTLS handshake.
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if proxyFunc != nil {
		transport.Proxy = proxyFunc
	}

	transport.DialTLSContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		// Dial the underlying TCP connection
		tcpConn, err := transport.DialContext(ctx, network, addr)
		if err != nil {
			return nil, err
		}

		// Parse the host from addr
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			host = addr // fallback if no port
		}

		// Configure uTLS client
		uConn := utls.UClient(tcpConn, &utls.Config{ServerName: host}, clientHelloID)
		if err := uConn.HandshakeContext(ctx); err != nil {
			_ = tcpConn.Close()
			return nil, fmt.Errorf("context: utls handshake failed: %w", err)
		}

		return uConn, nil
	}

	return transport, nil
}
