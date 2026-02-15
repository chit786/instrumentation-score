package collectors

import (
	"testing"
)

// TestCaseSensitiveMetrics tests that batch patterns match both uppercase and lowercase metrics
func TestCaseSensitiveMetrics(t *testing.T) {
	tests := []struct {
		name     string
		metrics  []string
		expected map[string][]string // pattern -> metrics
	}{
		{
			name: "MySQL metrics with mixed case",
			metrics: []string{
				"MySQLMonitor_AbortedClientMonitor_aborted_clients_total",
				"MySQLMonitor_AvailibilityMonitor_availability_status",
				"mysql_global_status_connections",
				"mysql_global_status_threads_connected",
				"_MySQLMonitor_Connection_active_total",
			},
			expected: map[string][]string{
				"[mM].*": {
					"MySQLMonitor_AbortedClientMonitor_aborted_clients_total",
					"MySQLMonitor_AvailibilityMonitor_availability_status",
					"mysql_global_status_connections",
					"mysql_global_status_threads_connected",
				},
				"_.*": { // Underscore is not a letter, so no case variation
					"_MySQLMonitor_Connection_active_total",
				},
			},
		},
		{
			name: "Prometheus metrics with capital P",
			metrics: []string{
				"Prometheus_build_info",
				"prometheus_http_requests_total",
				"process_cpu_seconds_total",
			},
			expected: map[string][]string{
				"[pP].*": {
					"Prometheus_build_info",
					"prometheus_http_requests_total",
					"process_cpu_seconds_total",
				},
			},
		},
		{
			name: "Go metrics with capital G",
			metrics: []string{
				"Go_memstats_alloc_bytes",
				"go_goroutines",
				"go_threads",
			},
			expected: map[string][]string{
				"[gG].*": {
					"Go_memstats_alloc_bytes",
					"go_goroutines",
					"go_threads",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy := &AlphabeticBatchStrategy{CharsPerBatch: 1}
			batches := strategy.CreateBatches(tt.metrics)

			// Create a map of pattern -> metrics for comparison
			result := make(map[string][]string)
			for _, batch := range batches {
				result[batch.Pattern] = batch.Metrics
			}

			// Verify each expected pattern exists and contains the right metrics
			for expectedPattern, expectedMetrics := range tt.expected {
				actualMetrics, found := result[expectedPattern]
				if !found {
					t.Errorf("Expected pattern %s not found in batches", expectedPattern)
					continue
				}

				if len(actualMetrics) != len(expectedMetrics) {
					t.Errorf("Pattern %s: expected %d metrics, got %d",
						expectedPattern, len(expectedMetrics), len(actualMetrics))
				}

				// Check each expected metric is in the batch
				metricSet := make(map[string]bool)
				for _, m := range actualMetrics {
					metricSet[m] = true
				}

				for _, expectedMetric := range expectedMetrics {
					if !metricSet[expectedMetric] {
						t.Errorf("Pattern %s: missing expected metric %s",
							expectedPattern, expectedMetric)
					}
				}
			}
		})
	}
}

// TestCreateCharRangePattern_CaseInsensitive tests that patterns are case-insensitive
func TestCreateCharRangePattern_CaseInsensitive(t *testing.T) {
	tests := []struct {
		name     string
		chars    []rune
		expected string
	}{
		{
			name:     "single lowercase letter",
			chars:    []rune{'m'},
			expected: "[mM].*",
		},
		{
			name:     "consecutive letters",
			chars:    []rune{'a', 'b', 'c'},
			expected: "[a-cA-C].*",
		},
		{
			name:     "non-consecutive letters",
			chars:    []rune{'a', 'c', 'e'},
			expected: "[aAcCeE].*",
		},
		{
			name:     "underscore (non-letter)",
			chars:    []rune{'_'},
			expected: "_.*",
		},
		{
			name:     "digit (non-letter)",
			chars:    []rune{'0'},
			expected: "0.*",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := createCharRangePattern(tt.chars)
			if result != tt.expected {
				t.Errorf("createCharRangePattern(%v) = %s, want %s",
					tt.chars, result, tt.expected)
			}
		})
	}
}

// TestPatternMatchesBothCases tests that generated patterns actually match both cases
func TestPatternMatchesBothCases(t *testing.T) {
	testMetrics := []string{
		"MySQLMonitor_test",
		"mysql_test",
		"Prometheus_test",
		"prometheus_test",
		"Go_test",
		"go_test",
	}

	strategy := &AlphabeticBatchStrategy{CharsPerBatch: 1}
	batches := strategy.CreateBatches(testMetrics)

	// Check that metrics starting with both uppercase and lowercase are in the same batch
	for _, batch := range batches {
		hasUpper := false
		hasLower := false

		for _, metric := range batch.Metrics {
			firstChar := metric[0]
			if firstChar >= 'A' && firstChar <= 'Z' {
				hasUpper = true
			} else if firstChar >= 'a' && firstChar <= 'z' {
				hasLower = true
			}
		}

		// If batch has letters, it should have both cases or the pattern should handle both
		if (hasUpper || hasLower) && len(batch.Metrics) > 1 {
			// Pattern should be case-insensitive
			if !containsBothCases(batch.Pattern) && hasUpper && hasLower {
				t.Errorf("Batch pattern %s has both uppercase and lowercase metrics but pattern is not case-insensitive",
					batch.Pattern)
			}
		}
	}
}

// Helper function to check if pattern includes both cases
func containsBothCases(pattern string) bool {
	// Patterns like [mM].* or [a-cA-C].* are case-insensitive
	hasLower := false
	hasUpper := false
	for _, char := range pattern {
		if char >= 'a' && char <= 'z' {
			hasLower = true
		}
		if char >= 'A' && char <= 'Z' {
			hasUpper = true
		}
	}
	return hasLower && hasUpper
}
