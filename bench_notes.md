# Benchmark Notes

Performance analysis and optimization results for `burr` internals.

## Table of Contents

- [internal/analyzer](#internalanalyzer) - Term matching and content analysis

---

## internal/analyzer

The `FindTermMatches` function in `internal/analyzer/matcher.go` performs case-insensitive term search with sentence extraction. It was profiled and optimized for the intel pipeline.

### Test Environment

| Component | Details |
|-----------|----------|
| **CPU** | Intel(R) Core(TM) i5-3230M CPU @ 2.60GHz |
| **Go** | 1.22+ |
| **OS** | Linux (RHEL 9.x) |

### Benchmark Results

### Before Optimization

| Benchmark | ns/op | B/op | allocs/op |
|-----------|------|------|-----------|
| SmallContent (1KB) | 66,681 | 8,888 | 102 |
| MediumContent (10KB) | 647,712 | 90,713 | 898 |
| LargeContent (100KB) | 7,546,053 | 980,519 | 9,796 |
| ManyTerms (50KB) | 8,864,535 | 1,051,819 | 12,293 |

### After Optimization

| Benchmark | ns/op | B/op | allocs/op | Improvement |
|-----------|------|------|-----------|-------------|
| SmallContent (1KB) | 36,269 | 5,416 | 40 | **1.8x faster, 1.6x less memory, 2.5x fewer allocs** |
| MediumContent (10KB) | 315,028 | 48,344 | 212 | **2.1x faster, 1.9x less memory, 4.2x fewer allocs** |
| LargeContent (100KB) | 3,102,505 | 452,944 | 1,686 | **2.4x faster, 2.2x less memory, 5.8x fewer allocs** |
| ManyTerms (50KB) | 2,576,743 | 274,236 | 943 | **3.4x faster, 3.8x less memory, 13x fewer allocs** |

### Root Cause Analysis

Running `go tool pprof` on the unoptimized code revealed:

| Function | CPU Time | Issue |
|----------|----------|-------|
| `strings.ToLower` | 41% | Called repeatedly on each term's sentence iteration |
| `splitIntoSentences` | 6% | Called once per term (multiplied by term count) |
| `runtime.memmove` | 3% | String copying overhead |
| `strings.Builder.grow` | 14% | Slice growth without pre-allocation |

### Optimizations Applied

1. **Pre-lowercase content once**: Call `strings.ToLower(content)` only once before the term loop
2. **Pre-split sentences**: Split into sentences once, not per-term
3. **Pre-lowercase sentences**: Cache lowercase versions alongside originals
4. **Pre-lowercase terms**: Cache lowercase terms once
5. **Estimated pre-allocation**: Estimate sentence count (`len(text)/50`) to pre-size slices
6. **Pre-allocate result slice**: `make([]TermMatch, 0, len(terms))`

### Code Changes

**Original (before):**
```go
func FindTermMatches(content, url, domain string, terms []string) []TermMatch {
    // Called per term - inefficient!
    lowerContent := strings.ToLower(content)
    sentences := splitIntoSentences(content)
    
    for _, term := range terms {
        // ToLower called again per term!
        lowerTerm := strings.ToLower(term)
        for _, s := range sentences {
            if strings.Contains(strings.ToLower(s), lowerTerm) { // <- N*M ToLower calls
                // ...
            }
        }
    }
}
```

**Optimized (after):**
```go
func FindTermMatches(content, url, domain string, terms []string) []TermMatch {
    // Pre-compute everything once
    lowerContent := strings.ToLower(content)
    sentenceData := splitIntoSentencesOptimized(content)
    lowerTerms := make([]string, len(terms))
    for i, term := range terms {
        lowerTerms[i] = strings.ToLower(term)
    }
    
    // Search against pre-computed data
    for i, term := range terms {
        // Uses pre-computed lowercase data
        count := strings.Count(lowerContent, lowerTerms[i])
        // ...
    }
}
```

### Remaining Hotspots

| Function | CPU Time | Recommendation |
|----------|----------|-----------------|
| `strings.ToLower()` | ~41% | Unavoidable for case-insensitive search |
| `strings.Count()` | Primary search | Optimal for simple substring; consider `regexp` for wildcards |

### Future Optimization Opportunities

1. **sync.Pool**: Reuse sentence slices across high-frequency calls
2. **Inverted Index**: Pre-index terms for repeated searches across many documents
3. **Parallel Processing**: Use `errgroup` for multi-term search parallelism
4. **Memory Pool**: Pre-allocate buffers for large content processing

### Running Benchmarks

```bash
# Run all benchmarks
go test -bench=. -benchmem -count=3 ./internal/analyzer/...

# Profile CPU
go test -bench=BenchmarkFindTermMatches_LargeContent -cpuprofile=cpu.out ./internal/analyzer/
go tool pprof -text cpu.out

# Profile memory
go test -bench=BenchmarkFindTermMatches_LargeContent -memprofile=mem.out ./internal/analyzer/
go tool pprof -text -mem=mem.out mem.out
```

### Files Modified

| File | Changes |
|------|---------|
| `internal/analyzer/matcher.go` | Optimized implementation with pre-computation |
| `internal/analyzer/matcher_bench_test.go` | Benchmarks + sanity tests |
| `bench_notes.md` | This documentation |

---

## Adding New Benchmarks

To add benchmarks for a new package:

1. Create `_bench_test.go` file in the package directory
2. Use `testing.B` and report allocations with `b.ReportAllocs()`
3. Test realistic data sizes (1KB, 10KB, 100KB)
4. Run with `-count=3` for stable measurements

Example:
```go
func BenchmarkMyFunction_Large(b *testing.B) {
    data := generateTestData(100 * 1024) // 100KB
    
    b.ResetTimer()
    b.ReportAllocs()
    
    for i := 0; i < b.N; i++ {
        MyFunction(data)
    }
}
```
