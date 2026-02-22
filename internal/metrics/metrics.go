package metrics

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/FranksOps/burr/internal/storage"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	ScrapeRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "burr_scrape_requests_total",
			Help: "Total number of scrape requests executed",
		},
		[]string{"domain", "status", "detected", "detection_src"},
	)

	ScrapeDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "burr_scrape_duration_seconds",
			Help:    "Duration of scrape requests in seconds",
			Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30},
		},
		[]string{"domain"},
	)

	ScrapeBytesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "burr_scrape_bytes_total",
			Help: "Total bytes downloaded across all scrapes",
		},
		[]string{"domain"},
	)

	ProxyFailures = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "burr_proxy_failures_total",
			Help: "Total number of proxy failures during scrapes",
		},
		[]string{"proxy_url"},
	)
)

// RecordScrape updates the metrics given a ScrapeResult and domain.
func RecordScrape(domain string, res *storage.ScrapeResult) {
	if res == nil {
		return
	}

	detectedStr := "false"
	if res.DetectedBot {
		detectedStr = "true"
	}

	statusStr := strconv.Itoa(res.StatusCode)
	if res.Error != "" {
		statusStr = "error"
	}

	ScrapeRequestsTotal.WithLabelValues(domain, statusStr, detectedStr, res.DetectionSrc).Inc()
	ScrapeDuration.WithLabelValues(domain).Observe(res.Duration.Seconds())
	ScrapeBytesTotal.WithLabelValues(domain).Add(float64(len(res.Body)))
}

// Server encapsulates an HTTP server for Prometheus metrics.
type Server struct {
	srv *http.Server
}

// Start begins listening on the specified port and exposes /metrics.
// The server runs in a background goroutine and must be stopped via Server.Stop()
// to release resources and avoid leaks.
func Start(port int) *Server {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	srv := &http.Server{
		Addr:         fmt.Sprintf("127.0.0.1:%d", port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		// Suppress the error from intentional shutdown
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("metrics server failed: %v\n", err)
		}
	}()

	return &Server{srv: srv}
}

// Stop gracefully shuts down the metrics server.
func (s *Server) Stop(ctx context.Context) error {
	if s.srv == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return s.srv.Shutdown(ctx)
}
