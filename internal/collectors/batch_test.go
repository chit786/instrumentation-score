package collectors

import (
	"reflect"
	"testing"
)

func TestAlphabeticBatchStrategy_CreateBatches(t *testing.T) {
	tests := []struct {
		name          string
		charsPerBatch int
		metrics       []string
		wantBatches   int
		checkFirst    bool
		firstPattern  string
	}{
		{
			name:          "empty metrics",
			charsPerBatch: 3,
			metrics:       []string{},
			wantBatches:   0,
		},
		{
			name:          "single metric",
			charsPerBatch: 3,
			metrics:       []string{"api_calls_total"},
			wantBatches:   1,
			checkFirst:    true,
			firstPattern:  "[aA].*", // Case-insensitive pattern
		},
		{
			name:          "metrics across alphabet",
			charsPerBatch: 3,
			metrics: []string{
				"api_calls_total",
				"auth_errors_total",
				"cache_hits_total",
				"db_queries_total",
				"errors_total",
			},
			wantBatches:  2, // [acd] and [e]
			checkFirst:   true,
			firstPattern: "[aAcCdD].*", // Case-insensitive non-consecutive chars
		},
		{
			name:          "default chars per batch",
			charsPerBatch: 0, // Should default to 3
			metrics: []string{
				"api_calls",
				"auth_errors",
				"cache_hits",
			},
			wantBatches: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy := &AlphabeticBatchStrategy{
				CharsPerBatch: tt.charsPerBatch,
			}

			batches := strategy.CreateBatches(tt.metrics)

			if len(batches) != tt.wantBatches {
				t.Errorf("CreateBatches() returned %d batches, want %d", len(batches), tt.wantBatches)
			}

			if tt.checkFirst && len(batches) > 0 {
				if batches[0].Pattern != tt.firstPattern {
					t.Errorf("First batch pattern = %s, want %s", batches[0].Pattern, tt.firstPattern)
				}
			}

			// Verify all metrics are included
			totalMetrics := 0
			for _, batch := range batches {
				totalMetrics += len(batch.Metrics)
			}
			if totalMetrics != len(tt.metrics) {
				t.Errorf("Total metrics in batches = %d, want %d", totalMetrics, len(tt.metrics))
			}
		})
	}
}

func TestCustomBatchStrategy_CreateBatches(t *testing.T) {
	tests := []struct {
		name        string
		patterns    []string
		metrics     []string
		wantBatches int
	}{
		{
			name:        "empty patterns",
			patterns:    []string{},
			metrics:     []string{"api_calls"},
			wantBatches: 1, // Should return catch-all batch
		},
		{
			name:     "single pattern match",
			patterns: []string{"http_.*"},
			metrics: []string{
				"http_requests_total",
				"http_errors_total",
				"grpc_requests_total",
			},
			wantBatches: 2, // http_.* and catch-all
		},
		{
			name:     "multiple patterns",
			patterns: []string{"http_.*", "grpc_.*", "aws_.*"},
			metrics: []string{
				"http_requests_total",
				"grpc_requests_total",
				"aws_s3_requests",
				"custom_metric",
			},
			wantBatches: 4, // 3 patterns + catch-all
		},
		{
			name:     "all metrics matched",
			patterns: []string{".*_total"},
			metrics: []string{
				"http_requests_total",
				"grpc_requests_total",
			},
			wantBatches: 1, // Only the pattern, no catch-all
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy := &CustomBatchStrategy{
				Patterns: tt.patterns,
			}

			batches := strategy.CreateBatches(tt.metrics)

			if len(batches) != tt.wantBatches {
				t.Errorf("CreateBatches() returned %d batches, want %d", len(batches), tt.wantBatches)
			}

			// Verify all metrics are included
			totalMetrics := 0
			for _, batch := range batches {
				totalMetrics += len(batch.Metrics)
			}
			if totalMetrics != len(tt.metrics) {
				t.Errorf("Total metrics in batches = %d, want %d", totalMetrics, len(tt.metrics))
			}
		})
	}
}

func TestAdaptiveBatchStrategy_CreateBatches(t *testing.T) {
	tests := []struct {
		name                  string
		targetResultsPerBatch int
		metrics               []string
		wantMinBatches        int
	}{
		{
			name:                  "small dataset",
			targetResultsPerBatch: 100,
			metrics:               []string{"api_calls", "auth_errors"},
			wantMinBatches:        1,
		},
		{
			name:                  "default values",
			targetResultsPerBatch: 0,
			metrics:               []string{"api_calls", "auth_errors", "cache_hits"},
			wantMinBatches:        1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy := &AdaptiveBatchStrategy{
				TargetResultsPerBatch: tt.targetResultsPerBatch,
			}

			batches := strategy.CreateBatches(tt.metrics)

			if len(batches) < tt.wantMinBatches {
				t.Errorf("CreateBatches() returned %d batches, want at least %d", len(batches), tt.wantMinBatches)
			}

			// Verify all metrics are included
			totalMetrics := 0
			for _, batch := range batches {
				totalMetrics += len(batch.Metrics)
			}
			if totalMetrics != len(tt.metrics) {
				t.Errorf("Total metrics in batches = %d, want %d", totalMetrics, len(tt.metrics))
			}
		})
	}
}

func TestSplitPattern(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		expected []string
	}{
		{
			name:     "split range",
			pattern:  "[a-c].*",
			expected: []string{"[a-b].*", "c.*"},
		},
		{
			name:     "split single char",
			pattern:  "c.*",
			expected: []string{"c[a-m].*", "c[n-z].*"},
		},
		{
			name:     "split two chars",
			pattern:  "ca.*",
			expected: []string{"ca[a-m].*", "ca[n-z].*"},
		},
		{
			name:     "cannot split further",
			pattern:  "cab.*",
			expected: []string{}, // Triggers fallback
		},
		{
			name:     "small range",
			pattern:  "[a-b].*",
			expected: []string{}, // Cannot split 2-char range
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitPattern(tt.pattern)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("splitPattern(%s) = %v, want %v", tt.pattern, result, tt.expected)
			}
		})
	}
}

func TestDetectPatternDepth(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		expected int
	}{
		{
			name:     "range pattern",
			pattern:  "[a-c].*",
			expected: 0,
		},
		{
			name:     "single char",
			pattern:  "c.*",
			expected: 1,
		},
		{
			name:     "two chars",
			pattern:  "ca.*",
			expected: 2,
		},
		{
			name:     "three chars",
			pattern:  "cab.*",
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectPatternDepth(tt.pattern)
			if result != tt.expected {
				t.Errorf("detectPatternDepth(%s) = %d, want %d", tt.pattern, result, tt.expected)
			}
		})
	}
}

func TestFilterMetricsByPattern(t *testing.T) {
	tests := []struct {
		name     string
		metrics  []string
		pattern  string
		expected []string
	}{
		{
			name:     "simple pattern",
			metrics:  []string{"api_calls", "auth_errors", "db_queries"},
			pattern:  "a.*",
			expected: []string{"api_calls", "auth_errors"},
		},
		{
			name:     "range pattern",
			metrics:  []string{"api_calls", "auth_errors", "cache_hits", "db_queries"},
			pattern:  "[a-c].*",
			expected: []string{"api_calls", "auth_errors", "cache_hits"},
		},
		{
			name:     "no matches",
			metrics:  []string{"api_calls", "auth_errors"},
			pattern:  "z.*",
			expected: nil,
		},
		{
			name:     "invalid pattern",
			metrics:  []string{"api_calls"},
			pattern:  "[invalid",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterMetricsByPattern(tt.metrics, tt.pattern)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("filterMetricsByPattern() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCreateCharRangePattern(t *testing.T) {
	tests := []struct {
		name     string
		chars    []rune
		expected string
	}{
		{
			name:     "empty",
			chars:    []rune{},
			expected: ".*",
		},
		{
			name:     "single char",
			chars:    []rune{'a'},
			expected: "[aA].*", // Case-insensitive
		},
		{
			name:     "consecutive chars",
			chars:    []rune{'a', 'b', 'c'},
			expected: "[a-cA-C].*", // Case-insensitive range
		},
		{
			name:     "non-consecutive chars",
			chars:    []rune{'a', 'c', 'e'},
			expected: "[aAcCeE].*", // Case-insensitive character class
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := createCharRangePattern(tt.chars)
			if result != tt.expected {
				t.Errorf("createCharRangePattern(%v) = %s, want %s", tt.chars, result, tt.expected)
			}
		})
	}
}
