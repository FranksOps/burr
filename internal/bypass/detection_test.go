package bypass

import (
	"testing"

	"github.com/FranksOps/burr/internal/storage"
)

func TestDetectCloudflare(t *testing.T) {
	// Not blocked
	res := &storage.ScrapeResult{
		StatusCode: 200,
		Headers:    map[string][]string{"Server": {"nginx"}},
		Body:       []byte("OK"),
	}
	if detected, _ := detectCloudflare(res); detected {
		t.Errorf("expected not detected")
	}

	// CF Server Header
	res = &storage.ScrapeResult{
		StatusCode: 403,
		Headers:    map[string][]string{"Server": {"cloudflare"}},
		Body:       []byte("Access Denied"),
	}
	if detected, src := detectCloudflare(res); !detected || src != "Cloudflare" {
		t.Errorf("expected Cloudflare detection by header")
	}

	// CF Body signature
	res = &storage.ScrapeResult{
		StatusCode: 503,
		Headers:    map[string][]string{},
		Body:       []byte("<html>... cf-turnstile ...</html>"),
	}
	if detected, src := detectCloudflare(res); !detected || src != "Cloudflare" {
		t.Errorf("expected Cloudflare detection by body")
	}
}

func TestDetectAkamai(t *testing.T) {
	res := &storage.ScrapeResult{
		StatusCode: 403,
		Headers:    map[string][]string{"Server": {"AkamaiGHost"}},
		Body:       []byte(""),
	}
	if detected, src := detectAkamai(res); !detected || src != "Akamai" {
		t.Errorf("expected Akamai detection by header")
	}

	res = &storage.ScrapeResult{
		StatusCode: 403,
		Headers:    map[string][]string{},
		Body:       []byte("Access Denied... Reference #123.456"),
	}
	if detected, src := detectAkamai(res); !detected || src != "Akamai" {
		t.Errorf("expected Akamai detection by body")
	}
}

func TestDetectDataDome(t *testing.T) {
	res := &storage.ScrapeResult{
		StatusCode: 403,
		Headers:    map[string][]string{"X-DataDome": {"1"}},
		Body:       []byte(""),
	}
	if detected, src := detectDataDome(res); !detected || src != "DataDome" {
		t.Errorf("expected DataDome detection by header")
	}

	res = &storage.ScrapeResult{
		StatusCode: 403,
		Headers:    map[string][]string{},
		Body:       []byte("script src='https://geo.captcha-delivery.com/...'"),
	}
	if detected, src := detectDataDome(res); !detected || src != "DataDome" {
		t.Errorf("expected DataDome detection by body")
	}
}

func TestDetectPerimeterX(t *testing.T) {
	res := &storage.ScrapeResult{
		StatusCode: 403,
		Headers:    map[string][]string{"X-Px-Captcha": {"required"}},
		Body:       []byte(""),
	}
	if detected, src := detectPerimeterX(res); !detected || src != "PerimeterX" {
		t.Errorf("expected PerimeterX detection by header")
	}

	res = &storage.ScrapeResult{
		StatusCode: 403,
		Headers:    map[string][]string{},
		Body:       []byte("window._pxBlock = true;"),
	}
	if detected, src := detectPerimeterX(res); !detected || src != "PerimeterX" {
		t.Errorf("expected PerimeterX detection by body")
	}
}

func TestAnalyze(t *testing.T) {
	detectors := DefaultDetectors()

	res := &storage.ScrapeResult{
		StatusCode: 403,
		Headers:    map[string][]string{"X-DataDome": {"1"}},
		Body:       []byte(""),
	}

	detected := Analyze(res, detectors)
	if !detected {
		t.Errorf("expected detection to return true")
	}

	if !res.DetectedBot || res.DetectionSrc != "DataDome" {
		t.Errorf("expected result to be updated: %v, %s", res.DetectedBot, res.DetectionSrc)
	}

	resSafe := &storage.ScrapeResult{
		StatusCode: 200,
		Headers:    map[string][]string{},
		Body:       []byte("hello"),
	}

	detectedSafe := Analyze(resSafe, detectors)
	if detectedSafe {
		t.Errorf("expected safe result to return false")
	}
	if resSafe.DetectedBot || resSafe.DetectionSrc != "" {
		t.Errorf("expected safe result fields to be cleared")
	}
}
