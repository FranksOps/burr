# burr

`burr` is an expert Go-based web scraper and bot defense auditor. It is designed to evaluate, map, and document how common web protections (rate limits, fingerprinting, challenge pages) react against sophisticated evasion strategies.

**Disclaimer:** All targets crawled or audited via `burr` must strictly be your own property. It is designed for internal defense auditing.

## Features

- **TLS/HTTP Fingerprinting:** Utilizes `utls` to camouflage TLS handshakes as Chrome, Firefox, Safari, or randomized ALPN payloads.
- **Header & Identity Rotation:** Automatic User-Agent rotation alongside realistic header orders and casing implementations.
- **Concurrent Crawling:** BFS (Breadth-First Search) crawler capable of discovering pages via DOM parsing (`goquery`), scoped automatically to the target domain.
- **Rate Limiting & Jitter:** Customizable throughput (`--rps`) combined with randomized request timings (`--jitter`) configured to gaussian distributions.
- **Robust Storage Backends:** Pluggable interface currently supporting:
  - SQLite (Pure Go via `modernc.org/sqlite`)
  - Postgres (`pgx/v5`)
  - JSON (NDJSON format)
  - CSV (Base64 encoded responses for sanity)
- **Challenge Page Detection:** Native heuristic matching to detect Cloudflare, Akamai, DataDome, and PerimeterX (HUMAN) active blocks.
- **Proxy Support:** File-backed or direct-URL proxy rotation capabilities tracking failure/success metrics natively.
- **Prometheus Metrics:** Integrated endpoint exporting scrape success, failure, duration, byte counts, and proxy health stats.
- **Auditing Compliance:** Respects `robots.txt` boundaries and easily accepts `sitemap.xml` files as seed lists.

## Installation

### Static Binary
```bash
go build -ldflags="-w -s" -o bin/burr ./cmd/burr
./bin/burr --help
```

### Container (Podman / Docker)
A completely static, distroless equivalent base utilizing `scratch` is provided via `Containerfile`.
```bash
podman build -t burr .
podman run --rm --userns=keep-id -v $(pwd)/data:/data burr --store=sqlite --db=/data/burr.db https://example.com
```
*Note for RHEL/SELinux:* When running rootless via Podman, ensure you use the `--userns=keep-id` flag to allow the containerized application to correctly permission database files to your native system user.

## Usage

```bash
burr [flags] <url> [url...]
```

### Examples

**Basic Audit:**
```bash
burr https://example.com
```

**Thorough Fingerprinted Crawl:**
```bash
burr --depth 3 --rps 2 --jitter 0.5 --tls-fingerprint chrome --ua rotate --store sqlite --db myaudit.db https://example.com
```

**Proxy Rotation & Reports:**
```bash
burr --proxy ./proxies.txt --report html > report.html https://example.com
```

**Using Environment Variables:**
All flags can be mapped to environment variables prefixed with `BURR_`.
```bash
export BURR_PG_DSN="postgres://user:pass@localhost/burr"
burr --store postgres https://example.com
```

## CLI Flags

| Flag | Default | Description |
|---|---|---|
| `--store` | `sqlite` | Backend to use (`sqlite`, `postgres`, `json`, `csv`) |
| `--db` | `./burr.db` | File path or DSN for storage |
| `--depth` | `1` | Max BFS crawl depth (`0` = single page) |
| `--concurrency` | `3` | Parallel workers processing the crawl queue |
| `--seed` | | File containing newline-separated starting URLs |
| `--robots` | `false` | Whether to respect robots.txt rules |
| `--rps` | `1.0` | Maximum Requests Per Second |
| `--jitter` | `0.3` | Jitter factor applied to timing (`0.0`-`1.0`) |
| `--ua` | `rotate` | User Agent string or `"rotate"` to pull from builtin pool |
| `--proxy` | | Single proxy URL or filepath to proxy list |
| `--tls-fingerprint` | `chrome` | Target browser profile to mock (`chrome`, `firefox`, `safari`, `go`, `random`) |
| `--report` | `text` | Output report format upon completion (`text`, `json`, `html`) |
| `--metrics-port` | `0` | Port to expose Prometheus metrics (`0` to disable) |
| `--verbose` | `false` | Enable verbose `slog` DEBUG logging |

## Build Notes
`burr` defaults to a `CGO_ENABLED=0` build constraint, as all SQLite dependencies utilize purely native Go translations to eliminate painful glibc binding conflicts.
