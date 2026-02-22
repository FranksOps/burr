# burr

A Go-based web scraper and bot defense auditor designed to evaluate, map, and document how common web protections (rate limits, fingerprinting, challenge pages) react against sophisticated evasion strategies.

**Disclaimer:** All targets crawled or audited via `burr` must strictly be your own property. It is designed for internal defense auditing and competitive research on your own web properties only.

## Features

- **TLS/HTTP Fingerprinting:** Uses `utls` to mimic Chrome, Firefox, Safari, or randomized TLS fingerprints
- **Header & Identity Rotation:** User-Agent rotation with realistic header orders
- **Concurrent Crawling:** BFS crawler with DOM parsing, scoped to target domains
- **Rate Limiting & Jitter:** Configurable RPS with gaussian-distributed jitter
- **Pluggable Storage:** SQLite, Postgres, JSON, CSV backends
- **Challenge Detection:** Heuristic matching for Cloudflare, Akamai, DataDome, PerimeterX
- **Proxy Support:** File-backed or direct URL proxy rotation with health tracking
- **Prometheus Metrics:** Built-in metrics endpoint for observability
- **Robots.txt Compliance:** Optional respect for robots.txt rules
- **Sitemap Support:** Accept sitemap.xml as seed lists

## Installation

### From Source

```bash
# Clone and build
git clone https://github.com/FranksOps/burr.git
cd burr

# Build static binary (no CGO required)
CGO_ENABLED=0 go build -ldflags="-X main.version=$(git describe --tags --always) -X main.commit=$(git rev-parse HEAD)" -o bin/burr ./cmd/burr

# Verify
./bin/burr --help
```

### Container (Podman/Docker)

```bash
# Build the container
podman build -t burr .

# Run with data volume
podman run --rm --userns=keep-id -v $(pwd)/data:/data burr \
  --store=sqlite --db=/data/burr.db https://example.com
```

**SELinux Note:** On RHEL with SELinux enforcing, use `--userns=keep-id` for rootless Podman to avoid volume permission issues.

## Quick Start

```bash
# Basic single-page scrape
burr https://example.com

# Output to file
burr https://example.com --report html > example_report.html
```

## Usage

### Basic Commands

```bash
# Single URL scrape (depth 0)
burr https://example.com

# Crawl with depth
burr --depth 2 https://example.com

# Respect robots.txt
burr --robots --depth 1 https://example.com
```

### Bypass & Evasion Options

```bash
# Use Firefox TLS fingerprint
burr --tls-fingerprint firefox https://example.com

# Rotate User-Agents (default)
burr --ua rotate https://example.com

# Custom User-Agent
burr --ua "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36" https://example.com

# Rate limiting with jitter
burr --rps 2 --jitter 0.5 https://example.com

# Use proxy from file
burr --proxy ./proxies.txt https://example.com

# Use single proxy
burr --proxy http://proxy.example.com:8080 https://example.com
```

### Storage Backends

```bash
# SQLite (default)
burr --store sqlite --db ./burr.db https://example.com

# PostgreSQL (requires BURR_PG_DSN env var)
export BURR_PG_DSN="postgres://user:pass@localhost/burr"
burr --store postgres https://example.com

# JSON (NDJSON format)
burr --store json --db results.json https://example.com

# CSV
burr --store csv --db results.csv https://example.com
```

### Concurrent Crawling

```bash
# Increase workers
burr --concurrency 5 --depth 3 https://example.com

# Control queue size
burr --concurrency 3 --queue-size 5000 --depth 2 https://example.com
```

### Reporting

```bash
# Text report (default)
burr https://example.com --report text

# JSON report
burr https://example.com --report json

# HTML report
burr https://example.com --report html > report.html
```

### Seed URLs from File

```bash
# Use seed file (newline-separated URLs)
burr --seed ./urls.txt --depth 1

# Combine seed file with command-line URLs
burr --seed ./urls.txt https://example.com https://example.org
```

### Observability

```bash
# Enable verbose logging
burr --verbose https://example.com

# Enable Prometheus metrics
burr --metrics-port 9090 https://example.com
# Metrics available at http://localhost:9090/metrics
```

### Environment Variables

All CLI flags can be set via environment variables prefixed with `BURR_`:

```bash
export BURR_STORE=postgres
export BURR_PG_DSN="postgres://user:pass@localhost/burr"
export BURR_DEPTH=2
export BURR_RPS=2
export BURR_JITTER=0.5
export BURR_PROXY_LIST=/etc/burr/proxies.txt
export BURR_LOG_LEVEL=debug

burr https://example.com
```

## CLI Reference

| Flag | Default | Description |
|------|---------|-------------|
| `--store` | `sqlite` | Storage backend: `sqlite`, `postgres`, `json`, `csv` |
| `--db` | `./burr.db` | File path or DSN for storage |
| `--depth` | `1` | Crawl depth (0 = single page) |
| `--concurrency` | `3` | Parallel workers for crawl queue |
| `--queue-size` | `10000` | Internal BFS queue buffer size |
| `--seed` | - | File containing newline-separated seed URLs |
| `--robots` | `false` | Respect robots.txt rules |
| `--rps` | `1.0` | Maximum requests per second |
| `--jitter` | `0.3` | Jitter factor (0.0-1.0, gaussian distribution) |
| `--ua` | `rotate` | User-Agent string or `rotate` for builtin pool |
| `--proxy` | - | Proxy URL or path to proxy list file |
| `--tls-fingerprint` | `chrome` | TLS fingerprint: `chrome`, `firefox`, `safari`, `go`, `random` |
| `--headless` | `false` | Enable Playwright for JS challenges (requires headless build) |
| `--timeout` | `30s` | Per-request timeout |
| `--report` | `text` | Report format: `text`, `json`, `html` |
| `--verbose` | `false` | Enable debug logging |
| `--metrics-port` | `0` | Prometheus metrics port (0 = disabled) |

## Advanced Usage

### Intel Pipeline (SERP + Content Analysis)

```bash
# Run SERP search, crawl domains, and analyze content
burr intel \
  --query "HVAC repair Dallas" \
  --limit 50 \
  --terms "corrosion protection,heat exchanger,preventive maintenance"
```

This command:
1. Queries Google for the search term
2. Extracts top domains
3. Crawls content pages per domain
4. Analyzes for term matches
5. Writes reports to `reports/` directory

### Proxy List File Format

```
http://proxy1.example.com:8080
http://proxy2.example.com:8080
socks5://proxy3.example.com:1080
```

### Multiple URLs

```bash
burr https://example.com https://example.org https://example.net
```

### Full Audit Example

```bash
#!/bin/bash
# Comprehensive audit script

OUTPUT_DIR="./audit_results"
mkdir -p "$OUTPUT_DIR"

burr \
  --store sqlite \
  --db "$OUTPUT_DIR/audit.db" \
  --depth 3 \
  --concurrency 5 \
  --rps 2 \
  --jitter 0.4 \
  --tls-fingerprint chrome \
  --ua rotate \
  --proxy ./proxies.txt \
  --report html \
  --metrics-port 9090 \
  --robots \
  --verbose \
  "https://target1.example.com" \
  "https://target2.example.com" > "$OUTPUT_DIR/report.html"
```

## Build Tags

```bash
# Default build (static, no CGO)
CGO_ENABLED=0 go build -o bin/burr ./cmd/burr

# With headless support (Playwright for JS challenges)
go build -tags headless -o bin/burr-headless ./cmd/burr
```

## SELinux & Firewall (RHEL)

When enabling the Prometheus metrics endpoint:

```bash
# Open firewall port (if exposing externally)
firewall-cmd --permanent --add-port=9090/tcp
firewall-cmd --reload

# SELinux port permission
semanage port -a -t http_port_t -p tcp 9090
```

## Performance Notes

See `bench_notes.md` for internal analyzer performance benchmarks and optimization details.

## Project Structure

```
burr/
├── cmd/burr/           # CLI entrypoint
├── internal/
│   ├── analyzer/       # Term matching & content analysis
│   ├── bypass/         # Bypass technique implementations
│   ├── fingerprint/    # TLS/HTTP fingerprint profiles
│   ├── metrics/        # Prometheus metrics
│   ├── pipeline/      # SERP → crawl → analyze pipeline
│   ├── report/        # Report generators
│   ├── scraper/       # Core fetch engine & BFS crawler
│   ├── serp/          # SERP provider (Google scrape)
│   └── storage/       # Storage backends (sqlite, postgres, json, csv)
├── pkg/
│   ├── httpclient/    # Hardened HTTP client
│   ├── proxy/        # Proxy rotation
│   ├── ratelimit/    # Rate limiting with jitter
│   └── useragent/    # User-Agent pool
├── test/              # Integration tests
└── bin/               # Compiled binaries
```

## License

MIT License - See LICENSE file for details.
