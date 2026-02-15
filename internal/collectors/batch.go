package collectors

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// =============================================================================
// TYPES AND INTERFACES
// =============================================================================

// BatchStrategy defines the interface for creating metric batches.
// Implementations determine how metrics are grouped for batch queries.
type BatchStrategy interface {
	CreateBatches(metrics []string) []MetricBatch
}

// MetricBatch represents a group of metrics to query together.
type MetricBatch struct {
	Pattern         string   // Regex pattern for Prometheus query
	Metrics         []string // Metrics in this batch
	ExplicitMetrics bool     // If true, filter results to only these metrics
	BatchID         string   // Identifier for logging (used when batches share same pattern)
}

// =============================================================================
// ADAPTIVE BATCH STRATEGY (Recommended for RPS control)
// =============================================================================

// AdaptiveBatchStrategy creates balanced batches by recursively splitting
// large groups until each batch is under the target size.
//
// Time Complexity: O(n * d) where n = metrics count, d = max prefix depth
// Space Complexity: O(n) for grouping maps
type AdaptiveBatchStrategy struct {
	TargetResultsPerBatch int // Target max metrics per batch (default: 10000)
}

// CreateBatches groups metrics into balanced batches.
//
// Algorithm:
//  1. Group metrics by first character: O(n)
//  2. Sort groups by size for optimal combining: O(k log k) where k = unique chars
//  3. Recursively split large groups by deeper characters: O(n * d)
//  4. Combine small groups until reaching target: O(n)
func (s *AdaptiveBatchStrategy) CreateBatches(metrics []string) []MetricBatch {
	if len(metrics) == 0 {
		return nil
	}

	target := s.TargetResultsPerBatch
	if target <= 0 {
		target = 10000
	}

	// Step 1: Group by first character - O(n)
	groups := groupByChar(metrics, 0)

	// Step 2: Sort by count for optimal bin packing - O(k log k)
	sorted := sortGroupsByCount(groups)

	// Step 3: Process groups - combine small, split large
	var batches []MetricBatch
	var pending []rune
	var pendingMetrics []string

	for _, g := range sorted {
		if g.count > target {
			// Flush pending before processing large group
			if len(pending) > 0 {
				batches = append(batches, MetricBatch{
					Pattern: buildCharClassPattern(pending),
					Metrics: pendingMetrics,
				})
				pending, pendingMetrics = nil, nil
			}
			// Recursively split large group
			subBatches := splitGroup(string(g.char), groups[g.char], target, 1)
			batches = append(batches, subBatches...)
		} else if len(pendingMetrics)+g.count > target {
			// Flush pending, start new batch
			if len(pending) > 0 {
				batches = append(batches, MetricBatch{
					Pattern: buildCharClassPattern(pending),
					Metrics: pendingMetrics,
				})
			}
			pending = []rune{g.char}
			pendingMetrics = groups[g.char]
		} else {
			// Combine into pending batch
			pending = append(pending, g.char)
			pendingMetrics = append(pendingMetrics, groups[g.char]...)
		}
	}

	// Flush remaining
	if len(pending) > 0 {
		batches = append(batches, MetricBatch{
			Pattern: buildCharClassPattern(pending),
			Metrics: pendingMetrics,
		})
	}

	return batches
}

// =============================================================================
// RECURSIVE SPLITTING (Core algorithm)
// =============================================================================

const maxPrefixDepth = 50 // Handles very long common prefixes like airflow_local_task_*

// splitGroup recursively splits metrics by prefix until under target size.
// Falls back to count-based splitting when prefix splitting is exhausted.
func splitGroup(prefix string, metrics []string, target, depth int) []MetricBatch {
	// Base case: small enough or can't split further
	if len(metrics) <= target {
		return []MetricBatch{{
			Pattern: buildPrefixPattern(prefix),
			Metrics: metrics,
		}}
	}

	// Safety limit reached - fall back to count-based split
	if depth >= maxPrefixDepth {
		return splitByCount(prefix, metrics, target)
	}

	// Group by next character
	groups := groupByChar(metrics, depth)

	// Single group means all metrics share this character - go deeper
	if len(groups) == 1 {
		for char, groupMetrics := range groups {
			if char == 0 { // End of string marker
				return splitByCount(prefix, metrics, target)
			}
			return splitGroup(prefix+string(char), groupMetrics, target, depth+1)
		}
	}

	// Multiple groups - combine small, split large
	sorted := sortGroupsByCount(groups)
	var batches []MetricBatch
	var pending []rune
	var pendingMetrics []string

	for _, g := range sorted {
		if g.count > target {
			// Flush pending
			if len(pending) > 0 {
				batches = append(batches, MetricBatch{
					Pattern: buildPrefixWithCharsPattern(prefix, pending),
					Metrics: pendingMetrics,
				})
				pending, pendingMetrics = nil, nil
			}
			// Recurse into large group
			newPrefix := prefix
			if g.char != 0 {
				newPrefix = prefix + string(g.char)
			}
			subBatches := splitGroup(newPrefix, groups[g.char], target, depth+1)
			batches = append(batches, subBatches...)
		} else if len(pendingMetrics)+g.count > target && len(pending) > 0 {
			// Flush and restart
			batches = append(batches, MetricBatch{
				Pattern: buildPrefixWithCharsPattern(prefix, pending),
				Metrics: pendingMetrics,
			})
			pending = []rune{g.char}
			pendingMetrics = groups[g.char]
		} else {
			// Combine
			pending = append(pending, g.char)
			pendingMetrics = append(pendingMetrics, groups[g.char]...)
		}
	}

	// Flush remaining
	if len(pending) > 0 {
		batches = append(batches, MetricBatch{
			Pattern: buildPrefixWithCharsPattern(prefix, pending),
			Metrics: pendingMetrics,
		})
	}

	return batches
}

// splitByCount creates fixed-size batches when prefix splitting is exhausted.
// All batches share the same query pattern but filter to their assigned metrics.
func splitByCount(prefix string, metrics []string, target int) []MetricBatch {
	pattern := buildPrefixPattern(prefix)
	batches := make([]MetricBatch, 0, (len(metrics)+target-1)/target)

	for i := 0; i < len(metrics); i += target {
		end := min(i+target, len(metrics))
		batches = append(batches, MetricBatch{
			Pattern:         pattern,
			Metrics:         metrics[i:end],
			ExplicitMetrics: true,
			BatchID:         fmt.Sprintf("%s_part_%d", pattern, (i/target)+1),
		})
	}

	return batches
}

// =============================================================================
// HELPER FUNCTIONS - Grouping and Sorting
// =============================================================================

type charGroup struct {
	char  rune
	count int
}

// groupByChar groups metrics by character at given index. O(n)
func groupByChar(metrics []string, charIndex int) map[rune][]string {
	groups := make(map[rune][]string)
	for _, m := range metrics {
		var char rune
		if len(m) > charIndex {
			char = unicode.ToLower(rune(m[charIndex]))
		} // char=0 for end of string
		groups[char] = append(groups[char], m)
	}
	return groups
}

// sortGroupsByCount returns groups sorted by count ascending. O(k log k)
func sortGroupsByCount(groups map[rune][]string) []charGroup {
	result := make([]charGroup, 0, len(groups))
	for char, metrics := range groups {
		result = append(result, charGroup{char: char, count: len(metrics)})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].count < result[j].count
	})
	return result
}

// =============================================================================
// HELPER FUNCTIONS - Pattern Building
// =============================================================================

// buildCharClassPattern creates "[aAbBcC].*" from chars. Case-insensitive.
func buildCharClassPattern(chars []rune) string {
	if len(chars) == 0 {
		return ".*"
	}
	if len(chars) == 1 {
		return buildCaseInsensitiveChar(chars[0]) + ".*"
	}

	// Sort for potential range optimization
	sort.Slice(chars, func(i, j int) bool { return chars[i] < chars[j] })

	var b strings.Builder
	b.WriteString("[")

	// Check if consecutive for range notation
	if isConsecutive(chars) && len(chars) > 2 {
		writeCharRange(&b, chars[0], chars[len(chars)-1])
	} else {
		for _, c := range chars {
			b.WriteRune(c)
			if upper := unicode.ToUpper(c); upper != c {
				b.WriteRune(upper)
			}
		}
	}

	b.WriteString("].*")
	return b.String()
}

// buildPrefixPattern creates case-insensitive pattern for prefix.
func buildPrefixPattern(prefix string) string {
	var b strings.Builder
	for _, c := range prefix {
		b.WriteString(buildCaseInsensitiveChar(c))
	}
	b.WriteString(".*")
	return b.String()
}

// buildPrefixWithCharsPattern creates "prefix[chars].*"
func buildPrefixWithCharsPattern(prefix string, chars []rune) string {
	var b strings.Builder
	for _, c := range prefix {
		b.WriteString(buildCaseInsensitiveChar(c))
	}
	b.WriteString(buildCharClass(chars))
	b.WriteString(".*")
	return b.String()
}

// buildCaseInsensitiveChar creates "[aA]" for letters, "a" for others.
func buildCaseInsensitiveChar(c rune) string {
	if upper := unicode.ToUpper(c); upper != c {
		return fmt.Sprintf("[%c%c]", c, upper)
	}
	return string(c)
}

// buildCharClass creates "[abc]" or "[a-c]" from chars.
func buildCharClass(chars []rune) string {
	if len(chars) == 0 {
		return ""
	}
	if len(chars) == 1 {
		return buildCaseInsensitiveChar(chars[0])
	}

	sort.Slice(chars, func(i, j int) bool { return chars[i] < chars[j] })

	var b strings.Builder
	b.WriteString("[")

	if isConsecutive(chars) && len(chars) > 2 {
		writeCharRange(&b, chars[0], chars[len(chars)-1])
	} else {
		for _, c := range chars {
			b.WriteRune(c)
			if upper := unicode.ToUpper(c); upper != c {
				b.WriteRune(upper)
			}
		}
	}

	b.WriteString("]")
	return b.String()
}

// isConsecutive checks if runes are consecutive.
func isConsecutive(chars []rune) bool {
	for i := 1; i < len(chars); i++ {
		if chars[i]-chars[i-1] != 1 {
			return false
		}
	}
	return true
}

// writeCharRange writes "a-zA-Z" style range to builder.
func writeCharRange(b *strings.Builder, start, end rune) {
	b.WriteRune(start)
	b.WriteString("-")
	b.WriteRune(end)
	if upper := unicode.ToUpper(start); upper != start {
		b.WriteRune(upper)
		b.WriteString("-")
		b.WriteRune(unicode.ToUpper(end))
	}
}


// =============================================================================
// LEGACY STRATEGIES (Kept for backward compatibility)
// =============================================================================

// AlphabeticBatchStrategy groups metrics by first N characters.
// Deprecated: Use AdaptiveBatchStrategy for better load distribution.
type AlphabeticBatchStrategy struct {
	CharsPerBatch int
}

func (s *AlphabeticBatchStrategy) CreateBatches(metrics []string) []MetricBatch {
	if len(metrics) == 0 {
		return nil
	}

	charsPerBatch := s.CharsPerBatch
	if charsPerBatch <= 0 {
		charsPerBatch = 3
	}

	// Group by first character
	groups := groupByChar(metrics, 0)

	// Sort characters
	var chars []rune
	for c := range groups {
		chars = append(chars, c)
	}
	sort.Slice(chars, func(i, j int) bool { return chars[i] < chars[j] })

	// Create batches
	var batches []MetricBatch
	var currentChars []rune
	var currentMetrics []string

	for _, c := range chars {
		currentChars = append(currentChars, c)
		currentMetrics = append(currentMetrics, groups[c]...)

		if len(currentChars) >= charsPerBatch {
			batches = append(batches, MetricBatch{
				Pattern: buildCharClassPattern(currentChars),
				Metrics: currentMetrics,
			})
			currentChars, currentMetrics = nil, nil
		}
	}

	if len(currentChars) > 0 {
		batches = append(batches, MetricBatch{
			Pattern: buildCharClassPattern(currentChars),
			Metrics: currentMetrics,
		})
	}

	return batches
}

// CustomBatchStrategy uses user-defined regex patterns.
type CustomBatchStrategy struct {
	Patterns []string
}

func (s *CustomBatchStrategy) CreateBatches(metrics []string) []MetricBatch {
	if len(metrics) == 0 {
		return nil
	}

	if len(s.Patterns) == 0 {
		return []MetricBatch{{Pattern: ".*", Metrics: metrics}}
	}

	var batches []MetricBatch
	matched := make(map[string]bool, len(metrics))

	for _, pattern := range s.Patterns {
		var batchMetrics []string
		for _, m := range metrics {
			if matched[m] {
				continue
			}
			if matchesPattern(m, pattern) {
				batchMetrics = append(batchMetrics, m)
				matched[m] = true
			}
		}
		if len(batchMetrics) > 0 {
			batches = append(batches, MetricBatch{
				Pattern: pattern,
				Metrics: batchMetrics,
			})
		}
	}

	// Catch-all for unmatched
	var unmatched []string
	for _, m := range metrics {
		if !matched[m] {
			unmatched = append(unmatched, m)
		}
	}
	if len(unmatched) > 0 {
		batches = append(batches, MetricBatch{
			Pattern: ".*",
			Metrics: unmatched,
		})
	}

	return batches
}

// matchesPattern is a simple prefix matcher for common patterns.
func matchesPattern(metric, pattern string) bool {
	// Handle common patterns without regex overhead
	if strings.HasSuffix(pattern, ".*") {
		prefix := strings.TrimSuffix(pattern, ".*")
		return strings.HasPrefix(metric, prefix)
	}
	return metric == pattern
}

// =============================================================================
// PATTERN UTILITIES (Used by prometheus.go for auto-split)
// =============================================================================

// splitPattern splits a regex pattern into smaller patterns for query splitting.
func splitPattern(pattern string) []string {
	depth := detectPatternDepth(pattern)

	if depth == 0 {
		return splitCharRange(pattern)
	} else if depth == 1 {
		return splitToSecondChar(pattern)
	} else if depth == 2 {
		return splitToThirdChar(pattern)
	}

	return []string{}
}

// detectPatternDepth determines how specific the pattern is.
func detectPatternDepth(pattern string) int {
	pattern = strings.TrimSuffix(pattern, ".*")

	if strings.HasPrefix(pattern, "[") && strings.HasSuffix(pattern, "]") {
		return 0
	}

	count := 0
	for _, char := range pattern {
		if char != '[' && char != ']' && char != '-' && char != '*' && char != '.' {
			count++
		}
	}

	return count
}

// splitCharRange splits a character range pattern.
func splitCharRange(pattern string) []string {
	pattern = strings.TrimSuffix(pattern, ".*")
	pattern = strings.Trim(pattern, "[]")

	if len(pattern) == 3 && pattern[1] == '-' {
		start := rune(pattern[0])
		end := rune(pattern[2])

		if end-start <= 1 {
			return []string{}
		}

		mid := start + (end-start)/2

		if mid+1 == end {
			return []string{
				fmt.Sprintf("[%c-%c].*", start, mid),
				fmt.Sprintf("%c.*", end),
			}
		}

		return []string{
			fmt.Sprintf("[%c-%c].*", start, mid),
			fmt.Sprintf("[%c-%c].*", mid+1, end),
		}
	}

	return splitToSecondChar(pattern + ".*")
}

// splitToSecondChar splits by adding second character constraint.
func splitToSecondChar(pattern string) []string {
	pattern = strings.TrimSuffix(pattern, ".*")
	pattern = strings.Trim(pattern, "[]")

	return []string{
		fmt.Sprintf("%s[a-m].*", pattern),
		fmt.Sprintf("%s[n-z].*", pattern),
	}
}

// splitToThirdChar splits by adding third character constraint.
func splitToThirdChar(pattern string) []string {
	pattern = strings.TrimSuffix(pattern, ".*")

	return []string{
		fmt.Sprintf("%s[a-m].*", pattern),
		fmt.Sprintf("%s[n-z].*", pattern),
	}
}

// filterMetricsByPattern filters metrics that match a regex pattern.
func filterMetricsByPattern(metrics []string, pattern string) []string {
	regexPattern := "^" + pattern + "$"
	re, err := regexp.Compile(regexPattern)
	if err != nil {
		return []string{}
	}

	var matching []string
	for _, metric := range metrics {
		if re.MatchString(metric) {
			matching = append(matching, metric)
		}
	}

	return matching
}

// createCharRangePattern creates a regex pattern from a list of characters.
// Kept for backward compatibility with tests.
func createCharRangePattern(chars []rune) string {
	return buildCharClassPattern(chars)
}
