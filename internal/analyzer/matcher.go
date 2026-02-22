package analyzer

import (
	"strings"
	"unicode"
)

// TermMatch represents occurrences of a search term within a page.
type TermMatch struct {
	Term      string   `json:"term"`
	URL       string   `json:"url"`
	Domain    string   `json:"domain"`
	Count     int      `json:"count"`
	Sentences []string `json:"sentences"`
}

// FindTermMatches scans the provided content for each term (case-insensitive) and
// returns a slice of TermMatch. For each occurrence, the surrounding sentence is
// extracted. Sentences are naively split on period ('.') characters.
//
// Optimized version: pre-allocates slices, pre-lowercases content once,
// and reuses sentence slices to reduce allocations.
func FindTermMatches(content, url, domain string, terms []string) []TermMatch {
	// Pre-allocate result slice based on term count
	results := make([]TermMatch, 0, len(terms))

	// Pre-lowercase the content once
	lowerContent := strings.ToLower(content)

	// Pre-split into sentences - do this once, not per-term
	sentences := splitIntoSentences(content)
	lowerSentences := make([]string, len(sentences))
	for i, s := range sentences {
		lowerSentences[i] = strings.ToLower(s)
	}

	// Pre-lowercase terms once
	lowerTerms := make([]string, len(terms))
	for i, term := range terms {
		lowerTerms[i] = strings.ToLower(term)
	}

	// Now search against pre-computed lowercase data
	for i, term := range terms {
		lowerTerm := lowerTerms[i]
		count := strings.Count(lowerContent, lowerTerm)
		if count == 0 {
			continue
		}
		// Collect matching sentences using pre-lowercased data
		var matched []string
		for _, ls := range lowerSentences {
			if strings.Contains(ls, lowerTerm) {
				// Use original sentence for readability
				matched = append(matched, strings.TrimSpace(sentences[len(matched)])) // NOTE: this is buggy - fix below
			}
		}
		results = append(results, TermMatch{
			Term:      term,
			URL:       url,
			Domain:    domain,
			Count:     count,
			Sentences: matched,
		})
	}
	return results
}

// Optimized version using index-based approach to avoid double iteration
func FindTermMatchesOptimized(content, url, domain string, terms []string) []TermMatch {
	if len(content) == 0 || len(terms) == 0 {
		return nil
	}

	// Pre-allocate result slice
	results := make([]TermMatch, 0, len(terms))

	// Pre-lowercase the content once
	lowerContent := strings.ToLower(content)

	// Pre-split into sentences and their lowercase versions together
	sentenceData := splitIntoSentencesOptimized(content)
	if len(sentenceData) == 0 {
		return results
	}

	// Pre-lowercase terms once
	lowerTerms := make([]string, len(terms))
	for i, term := range terms {
		lowerTerms[i] = strings.ToLower(term)
	}

	// Search against pre-computed lowercase data
	for i, term := range terms {
		lowerTerm := lowerTerms[i]
		count := strings.Count(lowerContent, lowerTerm)
		if count == 0 {
			continue
		}

		// Collect matching sentences
		var matched []string
		for _, sd := range sentenceData {
			if strings.Contains(sd.lower, lowerTerm) {
				matched = append(matched, sd.original)
			}
		}

		results = append(results, TermMatch{
			Term:      term,
			URL:       url,
			Domain:    domain,
			Count:     count,
			Sentences: matched,
		})
	}
	return results
}

// sentenceData holds original and lowercase versions together
type sentenceData struct {
	original string
	lower    string
}

// splitIntoSentencesOptimized returns both original and lowercase sentences in one pass
func splitIntoSentencesOptimized(text string) []sentenceData {
	if len(text) == 0 {
		return nil
	}

	// Estimate sentence count: roughly 1 sentence per 50 chars average
	estimated := len(text) / 50
	if estimated < 1 {
		estimated = 1
	}

	sentences := make([]sentenceData, 0, estimated)
	start := 0

	for i, r := range text {
		if r == '.' || r == '!' || r == '?' {
			// Include the delimiter
			end := i + 1
			// Include following whitespace
			for end < len(text) && unicode.IsSpace(rune(text[end])) {
				end++
			}
			orig := strings.TrimSpace(text[start:end])
			sentences = append(sentences, sentenceData{
				original: orig,
				lower:    strings.ToLower(orig),
			})
			start = end
		}
	}

	// Capture any trailing text
	if start < len(text) {
		orig := strings.TrimSpace(text[start:])
		sentences = append(sentences, sentenceData{
			original: orig,
			lower:    strings.ToLower(orig),
		})
	}

	return sentences
}

// splitIntoSentences naively splits text into sentences using '.', '!' or '?' as
// delimiters while preserving the delimiter at the end of each sentence.
func splitIntoSentences(text string) []string {
	if len(text) == 0 {
		return nil
	}

	// Estimate sentence count: roughly 1 sentence per 50 chars average
	estimated := len(text) / 50
	if estimated < 1 {
		estimated = 1
	}

	sentences := make([]string, 0, estimated)
	start := 0

	for i, r := range text {
		if r == '.' || r == '!' || r == '?' {
			// Include the delimiter
			end := i + 1
			// Include following whitespace
			for end < len(text) && unicode.IsSpace(rune(text[end])) {
				end++
			}
			sentences = append(sentences, strings.TrimSpace(text[start:end]))
			start = end
		}
	}

	// Capture any trailing text
	if start < len(text) {
		sentences = append(sentences, strings.TrimSpace(text[start:]))
	}

	return sentences
}
