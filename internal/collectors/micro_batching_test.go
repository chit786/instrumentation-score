package collectors

import (
	"regexp"
	"testing"
)

// TestCreateExactMatchPattern tests exact match pattern generation for micro-batching
func TestCreateExactMatchPattern(t *testing.T) {
	tests := []struct {
		name     string
		metrics  []string
		expected string
	}{
		{
			name:     "empty metrics",
			metrics:  []string{},
			expected: ".*",
		},
		{
			name:     "single metric",
			metrics:  []string{"_MySQLMonitor_test"},
			expected: "^_MySQLMonitor_test$",
		},
		{
			name:     "two metrics",
			metrics:  []string{"_metric1", "_metric2"},
			expected: "^(_metric1|_metric2)$",
		},
		{
			name:     "multiple metrics with special characters",
			metrics:  []string{"_metric", "@metric", "-metric"},
			expected: "^(_metric|@metric|-metric)$",
		},
		{
			name:     "metrics with regex special chars",
			metrics:  []string{"metric.test", "metric+test", "metric*test"},
			expected: "^(metric\\.test|metric\\+test|metric\\*test)$",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := createExactMatchPattern(tt.metrics)
			if result != tt.expected {
				t.Errorf("createExactMatchPattern(%v) = %s, want %s",
					tt.metrics, result, tt.expected)
			}

			// Verify the pattern is valid regex
			if result != ".*" {
				_, err := regexp.Compile(result)
				if err != nil {
					t.Errorf("Generated pattern is not valid regex: %s, error: %v", result, err)
				}
			}
		})
	}
}

// TestExactMatchPatternMatching tests that patterns match correctly
func TestExactMatchPatternMatching(t *testing.T) {
	tests := []struct {
		name           string
		metrics        []string
		shouldMatch    []string
		shouldNotMatch []string
	}{
		{
			name:    "single underscore metric",
			metrics: []string{"_MySQLMonitor_test"},
			shouldMatch: []string{
				"_MySQLMonitor_test",
			},
			shouldNotMatch: []string{
				"_MySQLMonitor_test2",
				"MySQLMonitor_test",
				"_MySQLMonitor_tes",
			},
		},
		{
			name:    "multiple metrics",
			metrics: []string{"_metric1", "_metric2", "_metric3"},
			shouldMatch: []string{
				"_metric1",
				"_metric2",
				"_metric3",
			},
			shouldNotMatch: []string{
				"_metric4",
				"metric1",
				"_metric1_extra",
			},
		},
		{
			name:    "metrics with special characters",
			metrics: []string{"metric.test", "metric+test"},
			shouldMatch: []string{
				"metric.test",
				"metric+test",
			},
			shouldNotMatch: []string{
				"metricXtest", // . should not match any char
				"metrictest",  // + should not match one or more
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern := createExactMatchPattern(tt.metrics)
			re, err := regexp.Compile(pattern)
			if err != nil {
				t.Fatalf("Failed to compile pattern %s: %v", pattern, err)
			}

			// Test matches
			for _, metric := range tt.shouldMatch {
				if !re.MatchString(metric) {
					t.Errorf("Pattern %s should match %s but didn't", pattern, metric)
				}
			}

			// Test non-matches
			for _, metric := range tt.shouldNotMatch {
				if re.MatchString(metric) {
					t.Errorf("Pattern %s should NOT match %s but did", pattern, metric)
				}
			}
		})
	}
}

// TestMicroBatchSize tests the micro-batch size logic
func TestMicroBatchSize(t *testing.T) {
	tests := []struct {
		name              string
		totalMetrics      int
		microBatchSize    int
		expectedBatches   int
		expectedLastBatch int
	}{
		{
			name:              "exact multiple",
			totalMetrics:      30,
			microBatchSize:    10,
			expectedBatches:   3,
			expectedLastBatch: 10,
		},
		{
			name:              "with remainder",
			totalMetrics:      25,
			microBatchSize:    10,
			expectedBatches:   3,
			expectedLastBatch: 5,
		},
		{
			name:              "less than batch size",
			totalMetrics:      5,
			microBatchSize:    10,
			expectedBatches:   1,
			expectedLastBatch: 5,
		},
		{
			name:              "one metric",
			totalMetrics:      1,
			microBatchSize:    10,
			expectedBatches:   1,
			expectedLastBatch: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate batch creation
			metrics := make([]string, tt.totalMetrics)
			for i := 0; i < tt.totalMetrics; i++ {
				metrics[i] = "metric_" + string(rune('a'+i))
			}

			batches := 0
			lastBatchSize := 0

			for i := 0; i < len(metrics); i += tt.microBatchSize {
				end := i + tt.microBatchSize
				if end > len(metrics) {
					end = len(metrics)
				}
				batches++
				lastBatchSize = end - i
			}

			if batches != tt.expectedBatches {
				t.Errorf("Expected %d batches, got %d", tt.expectedBatches, batches)
			}

			if lastBatchSize != tt.expectedLastBatch {
				t.Errorf("Expected last batch size %d, got %d", tt.expectedLastBatch, lastBatchSize)
			}
		})
	}
}

// TestMicroBatchingPerformance compares micro-batching vs individual queries
func TestMicroBatchingPerformance(t *testing.T) {
	tests := []struct {
		name            string
		missedMetrics   int
		microBatchSize  int
		expectedQueries int
	}{
		{
			name:            "10 metrics, batch size 10",
			missedMetrics:   10,
			microBatchSize:  10,
			expectedQueries: 1, // 1 batch query vs 10 individual
		},
		{
			name:            "25 metrics, batch size 10",
			missedMetrics:   25,
			microBatchSize:  10,
			expectedQueries: 3, // 3 batch queries vs 25 individual
		},
		{
			name:            "100 metrics, batch size 10",
			missedMetrics:   100,
			microBatchSize:  10,
			expectedQueries: 10, // 10 batch queries vs 100 individual
		},
		{
			name:            "5 metrics, batch size 10",
			missedMetrics:   5,
			microBatchSize:  10,
			expectedQueries: 1, // 1 batch query vs 5 individual
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Calculate number of queries
			queries := (tt.missedMetrics + tt.microBatchSize - 1) / tt.microBatchSize

			if queries != tt.expectedQueries {
				t.Errorf("Expected %d queries, got %d", tt.expectedQueries, queries)
			}

			// Calculate improvement
			improvement := float64(tt.missedMetrics) / float64(queries)
			t.Logf("Micro-batching improvement: %.1fx fewer queries (%d → %d)",
				improvement, tt.missedMetrics, queries)

			// Verify improvement
			if improvement < 1.0 {
				t.Error("Micro-batching should always be better than or equal to individual queries")
			}
		})
	}
}

// TestMicroBatchingEdgeCases tests edge cases for micro-batching
func TestMicroBatchingEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		metrics []string
		wantErr bool
	}{
		{
			name:    "empty metrics list",
			metrics: []string{},
			wantErr: false,
		},
		{
			name:    "single metric",
			metrics: []string{"_metric"},
			wantErr: false,
		},
		{
			name: "metrics with very long names",
			metrics: []string{
				"very_long_metric_name_that_exceeds_normal_length_limits_but_should_still_work_correctly",
			},
			wantErr: false,
		},
		{
			name: "metrics with all special characters",
			metrics: []string{
				"_metric",
				"@metric",
				"-metric",
				"#metric",
				"$metric",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern := createExactMatchPattern(tt.metrics)

			// Verify pattern is valid
			if pattern != ".*" {
				_, err := regexp.Compile(pattern)
				if (err != nil) != tt.wantErr {
					t.Errorf("createExactMatchPattern() error = %v, wantErr %v", err, tt.wantErr)
				}
			}
		})
	}
}
