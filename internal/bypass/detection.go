package bypass

import (
	"bytes"
	"net/http"
	"strings"

	"github.com/FranksOps/burr/internal/storage"
)

// Detector examines a scrape result to determine if a bot protection mechanism
// blocked or challenged the request.
type Detector func(res *storage.ScrapeResult) (detected bool, source string)

// DefaultDetectors returns the standard list of bot protection detectors.
func DefaultDetectors() []Detector {
	return []Detector{
		detectCloudflare,
		detectAkamai,
		detectDataDome,
		detectPerimeterX,
	}
}

// Analyze runs the result through all provided detectors. It updates the result
// in place with the detection status and returns true if any detection triggered.
func Analyze(res *storage.ScrapeResult, detectors []Detector) bool {
	if res == nil {
		return false
	}
	for _, d := range detectors {
		if detected, source := d(res); detected {
			res.DetectedBot = true
			res.DetectionSrc = source
			return true
		}
	}
	res.DetectedBot = false
	res.DetectionSrc = ""
	return false
}

func getHeader(headers map[string][]string, key string) string {
	if vals, ok := headers[key]; ok && len(vals) > 0 {
		return vals[0]
	}
	// Case-insensitive fallback
	lowerKey := strings.ToLower(key)
	for k, vals := range headers {
		if strings.ToLower(k) == lowerKey && len(vals) > 0 {
			return vals[0]
		}
	}
	return ""
}

// detectCloudflare looks for common Cloudflare challenge/block signatures.
func detectCloudflare(res *storage.ScrapeResult) (bool, string) {
	// Status codes 403 or 503 are common for CF challenges
	if res.StatusCode == http.StatusForbidden || res.StatusCode == http.StatusServiceUnavailable {
		// Check headers
		server := strings.ToLower(getHeader(res.Headers, "Server"))
		if strings.Contains(server, "cloudflare") {
			return true, "Cloudflare"
		}

		// Check body signatures
		if bytes.Contains(res.Body, []byte("cf-browser-verification")) ||
			bytes.Contains(res.Body, []byte("cloudflare-nginx")) ||
			bytes.Contains(res.Body, []byte("cf-turnstile")) ||
			bytes.Contains(res.Body, []byte("Attention Required! | Cloudflare")) {
			return true, "Cloudflare"
		}
	}
	return false, ""
}

// detectAkamai looks for Akamai Bot Manager signatures.
func detectAkamai(res *storage.ScrapeResult) (bool, string) {
	if res.StatusCode == http.StatusForbidden {
		server := strings.ToLower(getHeader(res.Headers, "Server"))
		if strings.Contains(server, "akamai") {
			return true, "Akamai"
		}

		// Akamai often returns a generic "Reference #" block page
		if bytes.Contains(res.Body, []byte("Reference #")) && bytes.Contains(res.Body, []byte("Access Denied")) {
			return true, "Akamai"
		}
	}
	return false, ""
}

// detectDataDome looks for DataDome challenge/block signatures.
func detectDataDome(res *storage.ScrapeResult) (bool, string) {
	// DataDome often returns 403
	if res.StatusCode == http.StatusForbidden {
		server := strings.ToLower(getHeader(res.Headers, "Server"))
		if strings.Contains(server, "datadome") {
			return true, "DataDome"
		}

		// Look for DataDome specific headers
		if getHeader(res.Headers, "X-DataDome") != "" || getHeader(res.Headers, "X-DataDome-Response") != "" {
			return true, "DataDome"
		}

		// Body signatures
		if bytes.Contains(res.Body, []byte("geo.captcha-delivery.com")) || bytes.Contains(res.Body, []byte("datadome")) {
			return true, "DataDome"
		}
	}
	return false, ""
}

// detectPerimeterX looks for PerimeterX (HUMAN) signatures.
func detectPerimeterX(res *storage.ScrapeResult) (bool, string) {
	if res.StatusCode == http.StatusForbidden {
		// Look for PX specific cookies or headers
		if getHeader(res.Headers, "X-Px-Captcha") != "" {
			return true, "PerimeterX"
		}

		// Body signatures
		if bytes.Contains(res.Body, []byte("client.perimeterx.net")) ||
			bytes.Contains(res.Body, []byte("px-captcha")) ||
			bytes.Contains(res.Body, []byte("_pxBlock")) {
			return true, "PerimeterX"
		}
	}
	return false, ""
}
