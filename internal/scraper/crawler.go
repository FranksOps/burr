package scraper

import (
	"bytes"
	"context"
	"log/slog"
	"net/url"
	"strings"
	"sync"

	"github.com/FranksOps/burr/internal/metrics"
	"github.com/FranksOps/burr/internal/storage"
	"github.com/FranksOps/burr/pkg/ratelimit"
	"github.com/PuerkitoBio/goquery"
	"golang.org/x/sync/errgroup"
)

// CrawlConfig provides parameters for the BFS crawler.
type CrawlConfig struct {
	MaxDepth    int
	Concurrency int
	Backend     storage.Backend
	// In-scope domains, ensures we don't crawl the whole internet
	Domains []string
	// RespectRobots specifies whether to check robots.txt before fetching
	RespectRobots bool
	// UserAgent is the User-Agent string to use when checking robots.txt
	UserAgent string
	// RequestsPerSecond limits the fetch rate (0 = unlimited)
	RequestsPerSecond float64
	// Jitter applies randomness to the rate limiter (0.0 to 1.0)
	Jitter float64
	// QueueSize limits the depth of the internal BFS queue (0 = default 10000)
	QueueSize int
}

// Crawler coordinates the crawling of web pages starting from seeds.
type Crawler struct {
	cfg     CrawlConfig
	fetcher *Fetcher
	logger  *slog.Logger
	auditor *RobotsTxtAuditor
	limiter *ratelimit.Limiter

	// Track visited URLs to prevent loops
	visitedMu sync.RWMutex
	visited   map[string]struct{}
}

type job struct {
	URL   string
	Depth int
}

// NewCrawler creates a new BFS crawler.
func NewCrawler(cfg CrawlConfig, fetcher *Fetcher, logger *slog.Logger) *Crawler {
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 3
	}
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = "*" // default generic user-agent for robots.txt
	}

	var auditor *RobotsTxtAuditor
	if cfg.RespectRobots {
		auditor = NewRobotsTxtAuditor(fetcher, logger)
	}

	return &Crawler{
		cfg:     cfg,
		fetcher: fetcher,
		logger:  logger,
		auditor: auditor,
		limiter: ratelimit.NewLimiter(cfg.RequestsPerSecond, cfg.Jitter),
		visited: make(map[string]struct{}),
	}
}

// Run starts the BFS crawl starting from the provided seed URLs.
func (c *Crawler) Run(ctx context.Context, seeds []string) error {
	defer c.limiter.Stop()

	queueSize := c.cfg.QueueSize
	if queueSize <= 0 {
		queueSize = 10000 // default buffer size
	}
	queue := make(chan job, queueSize)

	// Add seeds
	for _, seed := range seeds {
		if c.shouldVisit(seed) {
			c.markVisited(seed)
			queue <- job{URL: seed, Depth: 0}
		}
	}

	// We use an errgroup to manage concurrent workers
	g, gCtx := errgroup.WithContext(ctx)

	// A waitgroup just for tracking when all current queue items are processed,
	// allowing us to know when the crawl is truly idle/done.
	// Note: new jobs discovered during processing also increment the WaitGroup (wg.Add(1))
	// before being sent to the queue. This pattern ensures we wait for both seed links
	// and dynamically discovered links.
	var jobsWg sync.WaitGroup
	jobsWg.Add(len(queue))

	for i := 0; i < c.cfg.Concurrency; i++ {
		g.Go(func() error {
			for {
				select {
				case <-gCtx.Done():
					return gCtx.Err()
				case j := <-queue:
					c.processJob(gCtx, j, queue, &jobsWg)
					jobsWg.Done()
				}
			}
		})
	}

	// Wait for all jobs to complete in a separate goroutine
	done := make(chan struct{})
	go func() {
		jobsWg.Wait()
		close(done)
	}()

	select {
	case <-gCtx.Done():
		return gCtx.Err()
	case <-done:
		// all jobs finished
	}

	return nil
}

func (c *Crawler) processJob(ctx context.Context, j job, queue chan<- job, wg *sync.WaitGroup) {
	if c.cfg.RespectRobots && c.auditor != nil {
		allowed, err := c.auditor.IsAllowed(ctx, j.URL, c.cfg.UserAgent)
		if err != nil {
			c.logger.Warn("error checking robots.txt", "url", j.URL, "err", err)
			// Decide policy here: fail open or fail closed? We usually fail open if robots.txt check errors.
		} else if !allowed {
			c.logger.Debug("url blocked by robots.txt", "url", j.URL)
			return
		}
	}

	c.logger.Debug("fetching", "url", j.URL, "depth", j.Depth)

	// Apply rate limit before fetching
	if err := c.limiter.Wait(ctx); err != nil {
		c.logger.Error("rate limiter cancelled", "url", j.URL, "err", err)
		return
	}

	result, err := c.fetcher.Fetch(ctx, j.URL)
	if err != nil {
		c.logger.Error("fetch error", "url", j.URL, "err", err)
		// Fetcher returns partial result on error, we still want to save it
	}

	// Save result
	if c.cfg.Backend != nil && result != nil {
		if err := c.cfg.Backend.Save(ctx, result); err != nil {
			c.logger.Error("failed to save result", "url", j.URL, "err", err)
		}
	}

	// Record metrics
	if result != nil {
		domain := ""
		if parsedURL, parseErr := url.Parse(j.URL); parseErr == nil {
			domain = parsedURL.Hostname()
		}
		metrics.RecordScrape(domain, result)
	}

	// If we hit depth limit or failed, do not extract links
	if j.Depth >= c.cfg.MaxDepth || result == nil || result.Error != "" {
		return
	}

	// Only parse HTML for links
	contentType := ""
	if vals := result.Headers["Content-Type"]; len(vals) > 0 {
		contentType = vals[0]
	}

	if strings.Contains(strings.ToLower(contentType), "text/html") {
		links := c.extractLinks(j.URL, result.Body)
		for _, link := range links {
			if c.shouldVisit(link) {
				c.markVisited(link)
				wg.Add(1)
				select {
				case queue <- job{URL: link, Depth: j.Depth + 1}:
				case <-ctx.Done():
					wg.Done() // Context cancelled, give up
				}
			}
		}
	}
}

func (c *Crawler) shouldVisit(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	// Normalize
	u.Fragment = ""
	normalized := u.String()

	c.visitedMu.RLock()
	_, seen := c.visited[normalized]
	c.visitedMu.RUnlock()

	if seen {
		return false
	}

	// Check domain scope
	if len(c.cfg.Domains) > 0 {
		inScope := false
		host := strings.ToLower(u.Hostname())
		for _, domain := range c.cfg.Domains {
			d := strings.ToLower(domain)
			if host == d || strings.HasSuffix(host, "."+d) {
				inScope = true
				break
			}
		}
		if !inScope {
			return false
		}
	}

	// Only HTTP/HTTPS
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}

	return true
}

func (c *Crawler) markVisited(rawURL string) {
	u, err := url.Parse(rawURL)
	if err == nil {
		u.Fragment = ""
		rawURL = u.String()
	}

	c.visitedMu.Lock()
	c.visited[rawURL] = struct{}{}
	c.visitedMu.Unlock()
}

func (c *Crawler) extractLinks(baseURL string, body []byte) []string {
	base, err := url.Parse(baseURL)
	if err != nil {
		return nil
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil
	}

	var links []string
	doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists {
			return
		}

		u, err := url.Parse(href)
		if err != nil {
			return
		}

		// Resolve relative URLs
		resolved := base.ResolveReference(u)
		links = append(links, resolved.String())
	})

	return links
}
