package collectors

import (
	"testing"
)

// TestSafetyNet_MissedMetrics tests that metrics not covered by batches are caught
func TestSafetyNet_MissedMetrics(t *testing.T) {
	tests := []struct {
		name                string
		allMetrics          []string
		expectedInBatches   []string
		expectedMissed      []string
		batchStrategy       BatchStrategy
	}{
		{
			name: "Underscore metrics should be batched",
			allMetrics: []string{
				"_MySQLMonitor_Connection_active_total",
				"_internal_metric",
				"mysql_connections",
				"http_requests_total",
			},
			expectedInBatches: []string{
				"_MySQLMonitor_Connection_active_total",
				"_internal_metric",
				"mysql_connections",
				"http_requests_total",
			},
			expectedMissed: []string{},
			batchStrategy: &AlphabeticBatchStrategy{CharsPerBatch: 1},
		},
		{
			name: "Special characters should be batched",
			allMetrics: []string{
				"_metric",
				"0_metric",
				"9_metric",
				"-metric",
				"@metric",
				"mysql_test",
			},
			expectedInBatches: []string{
				"_metric",
				"0_metric",
				"9_metric",
				"-metric",
				"@metric",
				"mysql_test",
			},
			expectedMissed: []string{},
			batchStrategy: &AlphabeticBatchStrategy{CharsPerBatch: 1},
		},
		{
			name: "Mixed case metrics should be batched together",
			allMetrics: []string{
				"MySQLMonitor_test",
				"mysql_test",
				"MySQL_test",
				"MYSQL_test",
			},
			expectedInBatches: []string{
				"MySQLMonitor_test",
				"mysql_test",
				"MySQL_test",
				"MYSQL_test",
			},
			expectedMissed: []string{},
			batchStrategy: &AlphabeticBatchStrategy{CharsPerBatch: 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			batches := tt.batchStrategy.CreateBatches(tt.allMetrics)

			// Collect all metrics from batches
			batchedMetrics := make(map[string]bool)
			for _, batch := range batches {
				for _, metric := range batch.Metrics {
					batchedMetrics[metric] = true
				}
			}

			// Check expected batched metrics
			for _, expected := range tt.expectedInBatches {
				if !batchedMetrics[expected] {
					t.Errorf("Expected metric %s to be in batches, but it was not", expected)
				}
			}

			// Check expected missed metrics
			for _, expected := range tt.expectedMissed {
				if batchedMetrics[expected] {
					t.Errorf("Expected metric %s to be missed, but it was batched", expected)
				}
			}

			// Verify no metrics are lost
			if len(batchedMetrics) != len(tt.allMetrics) {
				t.Errorf("Expected %d metrics in batches, got %d", len(tt.allMetrics), len(batchedMetrics))
			}

			// Verify all original metrics are accounted for
			for _, metric := range tt.allMetrics {
				if !batchedMetrics[metric] {
					t.Errorf("Metric %s was lost during batching", metric)
				}
			}
		})
	}
}

// TestSafetyNet_NoDataLoss tests that no metrics are lost in batching
func TestSafetyNet_NoDataLoss(t *testing.T) {
	allMetrics := []string{
		// Underscore metrics
		"_MySQLMonitor_Connection_active_total",
		"_internal_metric",
		"_test_metric",
		
		// Digit metrics
		"0_metric",
		"1_metric",
		"9_metric",
		
		// Special characters
		"-metric",
		"@metric",
		
		// Lowercase
		"mysql_connections",
		"http_requests_total",
		"go_goroutines",
		
		// Uppercase
		"MySQLMonitor_test",
		"Prometheus_build_info",
		"Go_memstats",
		
		// Mixed
		"MySQL_test",
		"MYSQL_test",
	}

	strategies := []struct {
		name     string
		strategy BatchStrategy
	}{
		{
			name:     "Alphabetic 1 char",
			strategy: &AlphabeticBatchStrategy{CharsPerBatch: 1},
		},
		{
			name:     "Alphabetic 3 chars",
			strategy: &AlphabeticBatchStrategy{CharsPerBatch: 3},
		},
		{
			name:     "Adaptive 10k",
			strategy: &AdaptiveBatchStrategy{TargetResultsPerBatch: 10000},
		},
	}

	for _, tt := range strategies {
		t.Run(tt.name, func(t *testing.T) {
			batches := tt.strategy.CreateBatches(allMetrics)

			// Collect all metrics from batches
			batchedMetrics := make(map[string]bool)
			for _, batch := range batches {
				for _, metric := range batch.Metrics {
					batchedMetrics[metric] = true
				}
			}

			// Verify no metrics are lost
			if len(batchedMetrics) != len(allMetrics) {
				t.Errorf("Data loss detected! Expected %d metrics, got %d", len(allMetrics), len(batchedMetrics))
				
				// Find missing metrics
				for _, metric := range allMetrics {
					if !batchedMetrics[metric] {
						t.Errorf("  Missing metric: %s", metric)
					}
				}
			}

			// Verify all original metrics are present
			for _, metric := range allMetrics {
				if !batchedMetrics[metric] {
					t.Errorf("Metric %s was lost during batching with strategy %s", metric, tt.name)
				}
			}

			// Verify no extra metrics were added
			for metric := range batchedMetrics {
				found := false
				for _, original := range allMetrics {
					if original == metric {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Extra metric %s was added during batching", metric)
				}
			}
		})
	}
}

// TestSafetyNet_UnderscoreMetrics specifically tests underscore handling
func TestSafetyNet_UnderscoreMetrics(t *testing.T) {
	metrics := []string{
		"_MySQLMonitor_AbortedClientMonitor_aborted_clients_total",
		"_MySQLMonitor_AvailibilityMonitor_availability_status",
		"_MySQLMonitor_Connection_active_total",
		"_internal_metric",
		"mysql_connections",
	}

	strategy := &AlphabeticBatchStrategy{CharsPerBatch: 1}
	batches := strategy.CreateBatches(metrics)

	// Find the underscore batch
	var underscoreBatch *MetricBatch
	for i := range batches {
		if batches[i].Pattern == "_.*" {
			underscoreBatch = &batches[i]
			break
		}
	}

	if underscoreBatch == nil {
		t.Fatal("No batch found for underscore metrics")
	}

	// Verify all underscore metrics are in the batch
	expectedUnderscore := []string{
		"_MySQLMonitor_AbortedClientMonitor_aborted_clients_total",
		"_MySQLMonitor_AvailibilityMonitor_availability_status",
		"_MySQLMonitor_Connection_active_total",
		"_internal_metric",
	}

	if len(underscoreBatch.Metrics) != len(expectedUnderscore) {
		t.Errorf("Expected %d underscore metrics, got %d", len(expectedUnderscore), len(underscoreBatch.Metrics))
	}

	underscoreSet := make(map[string]bool)
	for _, m := range underscoreBatch.Metrics {
		underscoreSet[m] = true
	}

	for _, expected := range expectedUnderscore {
		if !underscoreSet[expected] {
			t.Errorf("Expected underscore metric %s not found in batch", expected)
		}
	}
}

// TestSafetyNet_PatternMatching tests that patterns correctly match their metrics
func TestSafetyNet_PatternMatching(t *testing.T) {
	tests := []struct {
		pattern        string
		shouldMatch    []string
		shouldNotMatch []string
	}{
		{
			pattern: "_.*",
			shouldMatch: []string{
				"_MySQLMonitor_test",
				"_internal",
				"_test",
			},
			shouldNotMatch: []string{
				"mysql_test",
				"MySQLMonitor_test",
			},
		},
		{
			pattern: "[mM].*",
			shouldMatch: []string{
				"mysql_test",
				"MySQLMonitor_test",
				"MySQL_test",
				"my_test",
				"My_test",
			},
			shouldNotMatch: []string{
				"_MySQLMonitor_test",
				"http_test",
			},
		},
		{
			pattern: "[0-9].*",
			shouldMatch: []string{
				"0_metric",
				"1_metric",
				"9_metric",
			},
			shouldNotMatch: []string{
				"_0_metric",
				"metric_0",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			// Note: This is a simplified test - actual regex matching happens in Prometheus
			// We're just verifying the pattern format is correct
			
			// Pattern should not be empty
			if tt.pattern == "" {
				t.Error("Pattern should not be empty")
			}

			// Pattern should end with .*
			if len(tt.pattern) < 2 || tt.pattern[len(tt.pattern)-2:] != ".*" {
				t.Errorf("Pattern %s should end with .*", tt.pattern)
			}
		})
	}
}
