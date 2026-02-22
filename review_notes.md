# Code Review Notes

Prioritized findings (CRITICAL / HIGH / MEDIUM / LOW) based on the current codebase.

---

## CRITICAL

*None identified.*

---

## HIGH

*None identified.*

---

## MEDIUM

1. **Dynamic proxy context injection not implemented**
   - **Location:** `internal/scraper/fetcher.go`
   - **Issue:** Code comments indicate that per‑request proxy rotation via request context is planned, but the implementation is incomplete. The `activeProxy` variable is set but not used to override the transport's proxy function.
   - **Impact:** Proxy rotation per request does not currently work; all requests will use the same proxy if set via the pool.
   - **Suggestion:** Either remove the dead code or implement the proxy override using a custom `http.Transport` that reads from `req.Context().Value(proxyKey)`.

2. **CrawlConfig.QueueSize not exposed via CLI**
   - **Location:** `internal/scraper/crawler.go`, `cmd/burr/main.go`
   - **Issue:** The queue size is configurable via struct but has no CLI flag (`--queue-size`).
   - **Impact:** Users cannot tune the BFS queue depth without code changes.
   - **Suggestion:** Add a flag in `newRootCmd()` to set `cfg.QueueSize` if needed.

---

## LOW

1. **Unused import in pipeline stub**
   - **Location:** `internal/pipeline/pipeline.go`
   - **Issue:** `analyzer.FindTermMatches` is imported and used only to silence the "imported and not used" compiler error; it's not actually called.
   - **Impact:** Code compiles but contains dead code.
   - **Suggestion:** Replace the line with `_ = analyzer.FindTermMatches` or remove import and use a no‑op placeholder comment.

2. **Missing test for proxy scheme validation**
   - **Location:** `pkg/proxy/pool_test.go` (does not exist)
   - **Issue:** After adding scheme validation, there is no unit test confirming that invalid schemes (e.g., `ftp://`, `file://`) are rejected.
   - **Suggestion:** Add a test that calls `Add("ftp://proxy")` and expects an error.

3. **Metrics server still uses default HTTP server timeouts**
   - **Location:** `internal/metrics/metrics.go`
   - **Issue:** `http.Server` is created without explicit `ReadTimeout`, `WriteTimeout`, `IdleTimeout`. The defaults may be fine but could lead to slow‑client resource exhaustion.
   - **Impact:** Low – metrics endpoint is localhost only.
   - **Suggestion:** Consider setting timeouts for production hardening.

4. **Potential resource leak in pipeline test**
   - **Location:** `internal/pipeline/pipeline_test.go`
   - **Issue:** Test creates a real `scraper.Fetcher` but never closes its associated resources (e.g., client, transport). The test runs quickly but could leak file descriptors if expanded.
   - **Impact:** Minimal in current scope.
   - **Suggestion:** Add `defer fetcher.(? close` – but note that `Fetcher` lacks a `Close()` method; consider adding one if longer‑lived usage is expected.

5. **SELinux / firewall docs now present**
   - **Status:** ✅ Added in previous iteration – no further action required.

---

## Verification

- **Race detection:** `go test -race ./...` passes with no data races.
- **Static analysis:** `go vet ./...` reports no issues.
- **Deprecated code:** No usage of `ioutil` or other deprecated packages.
- **Context propagation:** All I/O functions accept `context.Context` as first argument.
- **Error handling:** All errors are wrapped with `fmt.Errorf("context: %w", ...)` and never silently ignored.
- **Logging:** Uses structured `log/slog` throughout.
- **Goroutine lifecycle:** All background goroutines (metrics server, rate limiter ticker) have explicit `Stop()` methods and are called by callers.

---

## New Findings (2026-02-22)

### LOW

6. **fmt.Printf in metrics server goroutine**
   - **Location:** `internal/metrics/metrics.go:99`
   - **Issue:** Uses `fmt.Printf` instead of structured `slog` for logging server errors
   - **Impact:** Deviation from AGENTS.md standard (should use `log/slog` in production)
   - **Suggestion:** Replace with `slogger.Error("metrics server failed", "err", err)`

7. **os.ReadFile without context**
   - **Location:** `cmd/burr/main.go:114`
   - **Issue:** File read does not use context, though it's local filesystem I/O not network
   - **Impact:** Minor - context cancellation won't abort this file read
   - **Suggestion:** Consider adding context-aware file read or document this as an exception

---

## Summary

- **CRITICAL:** 0
- **HIGH:** 0
- **MEDIUM:** 2 (proxy context injection, missing CLI flag)
- **LOW:** 6 (fmt.Printf, os.ReadFile without context, dead code, missing test, HTTP timeouts, test resource leak)

The codebase is in good shape; the MEDIUM items are minor functional gaps rather than bugs. The LOW items are improvements that would increase robustness and test coverage.
