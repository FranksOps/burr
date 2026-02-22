package analyzer

import (
	"strings"
	"testing"
)

// benchmarkContent generates a realistic HTML content string for benchmarking.
func benchmarkContent(size int) string {
	sb := strings.Builder{}
	sb.Grow(size)

	// Simulate a realistic blog/article page with repeated content
	paragraphs := []string{
		"HVAC maintenance is critical for industrial facilities. Regular preventive maintenance helps prevent corrosion protection issues.",
		"Heat exchanger systems require careful attention to prevent failures. Proper maintenance extends equipment life significantly.",
		"Commercial HVAC repair services offer comprehensive solutions. Emergency repairs are available 24/7 for critical systems.",
		"Corrosion protection is essential in marine environments. Specialized coatings provide long-lasting defense against salt water damage.",
		"Industrial heat exchangers benefit from quarterly inspections. Early detection of issues prevents costly downtime.",
	}

	for sb.Len() < size {
		for _, p := range paragraphs {
			sb.WriteString(p)
			sb.WriteString(". ")
		}
	}
	return sb.String()
}

func BenchmarkFindTermMatches_SmallContent(b *testing.B) {
	content := benchmarkContent(1024) // 1KB
	terms := []string{"HVAC", "corrosion", "heat exchanger", "maintenance"}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		FindTermMatches(content, "https://example.com/blog/test", "example.com", terms)
	}
}

func BenchmarkFindTermMatches_MediumContent(b *testing.B) {
	content := benchmarkContent(10 * 1024) // 10KB
	terms := []string{"HVAC", "corrosion", "heat exchanger", "maintenance", "repair"}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		FindTermMatches(content, "https://example.com/blog/test", "example.com", terms)
	}
}

func BenchmarkFindTermMatches_LargeContent(b *testing.B) {
	content := benchmarkContent(100 * 1024) // 100KB
	terms := []string{"HVAC", "corrosion", "heat exchanger", "maintenance", "repair", "preventive"}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		FindTermMatches(content, "https://example.com/blog/test", "example.com", terms)
	}
}

func BenchmarkFindTermMatches_ManyTerms(b *testing.B) {
	content := benchmarkContent(50 * 1024) // 50KB
	terms := []string{
		"HVAC", "corrosion", "heat exchanger", "maintenance", "repair",
		"preventive", "industrial", "commercial", "marine", "emergency",
		"inspection", "detection", "coatings", "equipment", "facilities",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		FindTermMatches(content, "https://example.com/blog/test", "example.com", terms)
	}
}

// Benchmark the optimized version
func BenchmarkFindTermMatchesOptimized_LargeContent(b *testing.B) {
	content := benchmarkContent(100 * 1024) // 100KB
	terms := []string{"HVAC", "corrosion", "heat exchanger", "maintenance", "repair", "preventive"}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		FindTermMatchesOptimized(content, "https://example.com/blog/test", "example.com", terms)
	}
}

func BenchmarkFindTermMatchesOptimized_ManyTerms(b *testing.B) {
	content := benchmarkContent(50 * 1024) // 50KB
	terms := []string{
		"HVAC", "corrosion", "heat exchanger", "maintenance", "repair",
		"preventive", "industrial", "commercial", "marine", "emergency",
		"inspection", "detection", "coatings", "equipment", "facilities",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		FindTermMatchesOptimized(content, "https://example.com/blog/test", "example.com", terms)
	}
}

func BenchmarkSplitIntoSentences(b *testing.B) {
	content := benchmarkContent(50 * 1024) // 50KB

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		splitIntoSentences(content)
	}
}

func BenchmarkSplitIntoSentences_Short(b *testing.B) {
	content := "This is a short sentence. Here is another one! And a third?"

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		splitIntoSentences(content)
	}
}

func BenchmarkSplitIntoSentencesOptimized(b *testing.B) {
	content := benchmarkContent(50 * 1024) // 50KB

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		splitIntoSentencesOptimized(content)
	}
}

// TestFindTermMatchesBasic is a sanity check for the matcher functions
func TestFindTermMatchesBasic(t *testing.T) {
	content := "HVAC maintenance is critical. HVAC systems need repair. Corrosion protection is important."
	terms := []string{"HVAC", "corrosion"}

	results := FindTermMatches(content, "https://example.com", "example.com", terms)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Check first result (HVAC)
	if results[0].Term != "HVAC" {
		t.Errorf("expected term HVAC, got %s", results[0].Term)
	}
	if results[0].Count != 2 {
		t.Errorf("expected count 2, got %d", results[0].Count)
	}

	// Check second result (corrosion)
	if results[1].Term != "corrosion" {
		t.Errorf("expected term corrosion, got %s", results[1].Term)
	}
	if results[1].Count != 1 {
		t.Errorf("expected count 1, got %d", results[1].Count)
	}
}

// TestFindTermMatchesOptimizedBasic sanity checks the optimized version
func TestFindTermMatchesOptimizedBasic(t *testing.T) {
	content := "HVAC maintenance is critical. HVAC systems need repair. Corrosion protection is important."
	terms := []string{"HVAC", "corrosion"}

	results := FindTermMatchesOptimized(content, "https://example.com", "example.com", terms)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	if results[0].Term != "HVAC" || results[0].Count != 2 {
		t.Errorf("HVAC: expected count 2, got %d", results[0].Count)
	}
	if results[1].Term != "corrosion" || results[1].Count != 1 {
		t.Errorf("corrosion: expected count 1, got %d", results[1].Count)
	}
}

// TestSplitIntoSentencesBasic tests sentence splitting
func TestSplitIntoSentencesBasic(t *testing.T) {
	content := "First sentence. Second one! Third?"
	sentences := splitIntoSentences(content)

	if len(sentences) != 3 {
		t.Fatalf("expected 3 sentences, got %d", len(sentences))
	}

	if sentences[0] != "First sentence." {
		t.Errorf("expected 'First sentence.', got '%s'", sentences[0])
	}
	if sentences[1] != "Second one!" {
		t.Errorf("expected 'Second one!', got '%s'", sentences[1])
	}
	if sentences[2] != "Third?" {
		t.Errorf("expected 'Third?', got '%s'", sentences[2])
	}
}
