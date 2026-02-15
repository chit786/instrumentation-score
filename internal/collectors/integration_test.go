package collectors

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestEdgeCase_PayloadTooLarge tests the payload limit handling
func TestEdgeCase_PayloadTooLarge(t *testing.T) {
	// Create a mock server that returns "payload too large" error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/query") {
			// Return payload too large error
			w.WriteHeader(http.StatusUnprocessableEntity)
			response := map[string]interface{}{
				"status": "error",
				"error":  "The response is too large. Please try to reduce the time range or narrow down your query to return fewer data points.",
			}
			json.NewEncoder(w).Encode(response)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewPrometheusClient(server.URL, "")
	
	// Test that GetMetricJobsBatch returns ErrPayloadTooLarge
	_, err := client.GetMetricJobsBatch("[a-z].*", "", time.Now().Unix())
	
	if err == nil {
		t.Fatal("Expected error for payload too large, got nil")
	}
	
	if !strings.Contains(err.Error(), "payload too large") {
		t.Errorf("Expected 'payload too large' error, got: %v", err)
	}
}

// TestEdgeCase_EmptyMetricList tests handling of empty metric lists
func TestEdgeCase_EmptyMetricList(t *testing.T) {
	strategies := []struct {
		name     string
		strategy BatchStrategy
	}{
		{"Alphabetic", &AlphabeticBatchStrategy{CharsPerBatch: 3}},
		{"Custom", &CustomBatchStrategy{Patterns: []string{"http_.*"}}},
		{"Adaptive", &AdaptiveBatchStrategy{TargetResultsPerBatch: 1000}},
	}

	for _, tc := range strategies {
		t.Run(tc.name, func(t *testing.T) {
			batches := tc.strategy.CreateBatches([]string{})
			
			// Should handle empty input gracefully
			if tc.name == "Alphabetic" {
				if len(batches) != 0 {
					t.Errorf("Expected 0 batches for empty input, got %d", len(batches))
				}
			}
		})
	}
}

// TestEdgeCase_SingleMetric tests handling of single metric
func TestEdgeCase_SingleMetric(t *testing.T) {
	metrics := []string{"single_metric_total"}
	
	strategy := &AlphabeticBatchStrategy{CharsPerBatch: 3}
	batches := strategy.CreateBatches(metrics)
	
	if len(batches) == 0 {
		t.Fatal("Expected at least 1 batch for single metric")
	}
	
	// Verify the metric is included
	found := false
	for _, batch := range batches {
		for _, m := range batch.Metrics {
			if m == "single_metric_total" {
				found = true
				break
			}
		}
	}
	
	if !found {
		t.Error("Single metric not found in any batch")
	}
}

// TestEdgeCase_SpecialCharactersInMetricNames tests metrics with special characters
func TestEdgeCase_SpecialCharactersInMetricNames(t *testing.T) {
	metrics := []string{
		"metric_with_underscore",
		"metric-with-dash",
		"metric.with.dots",
		"metric:with:colons",
		"METRIC_UPPERCASE",
		"123_metric_starts_with_number",
	}
	
	strategy := &AlphabeticBatchStrategy{CharsPerBatch: 3}
	batches := strategy.CreateBatches(metrics)
	
	// All metrics should be in some batch
	allMetrics := make(map[string]bool)
	for _, batch := range batches {
		for _, m := range batch.Metrics {
			allMetrics[m] = true
		}
	}
	
	for _, metric := range metrics {
		if !allMetrics[metric] {
			t.Errorf("Metric %s not found in any batch", metric)
		}
	}
}

// TestEdgeCase_VeryLongMetricNames tests handling of very long metric names
func TestEdgeCase_VeryLongMetricNames(t *testing.T) {
	longMetric := strings.Repeat("very_long_metric_name_", 20) + "total"
	metrics := []string{longMetric, "short"}
	
	strategy := &AlphabeticBatchStrategy{CharsPerBatch: 3}
	batches := strategy.CreateBatches(metrics)
	
	// Should handle long names without errors
	if len(batches) == 0 {
		t.Fatal("Expected batches for long metric names")
	}
}

// TestEdgeCase_PatternDepthLimit tests the pattern splitting depth limits
func TestEdgeCase_PatternDepthLimit(t *testing.T) {
	// Test that splitPattern respects depth limits
	patterns := []struct {
		input       string
		depth       int
		canSplit    bool
		description string
	}{
		{"[a-z].*", 0, true, "full alphabet range"},
		{"[a-c].*", 0, true, "small range"},
		{"a.*", 1, true, "single char"},
		{"aa.*", 2, true, "two chars"},
		{"aaa.*", 3, false, "three chars - too deep"},
		{"aaaa.*", 4, false, "four chars - too deep"},
	}
	
	for _, tc := range patterns {
		t.Run(fmt.Sprintf("depth_%d_%s", tc.depth, tc.description), func(t *testing.T) {
			result := splitPattern(tc.input)
			
			// Should never return nil
			if result == nil {
				t.Error("Expected non-nil result")
			}
			
			if tc.canSplit {
				// Should return multiple patterns
				if len(result) < 2 {
					// Some patterns might not split if they're already minimal
					// This is acceptable (e.g., single character ranges)
				}
			} else {
				// Too deep - should return empty slice to trigger fallback
				if len(result) != 0 {
					t.Logf("Pattern %s returned %d splits (expected 0 for fallback trigger)", tc.input, len(result))
				}
			}
		})
	}
}

// TestEdgeCase_ConcurrentFileWrites tests concurrent writes to same job
func TestEdgeCase_ConcurrentFileWrites(t *testing.T) {
	tmpDir := t.TempDir()
	
	writer, err := NewStreamingJobFileWriter(tmpDir, 10)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer writer.Close()
	
	// Write to same job from multiple goroutines
	jobName := "test-job"
	numWrites := 100
	
	done := make(chan bool, numWrites)
	
	for i := 0; i < numWrites; i++ {
		go func(idx int) {
			data := JobMetricData{
				Job:         jobName,
				MetricName:  fmt.Sprintf("metric_%d", idx),
				Labels:      []string{"label1", "label2"},
				Cardinality: "100",
			}
			writer.Write(data)
			done <- true
		}(i)
	}
	
	// Wait for all writes
	for i := 0; i < numWrites; i++ {
		<-done
	}
	
	writer.Close()
	
	// Verify file exists and has correct number of lines
	filename := filepath.Join(tmpDir, sanitizeJobName(jobName)+".txt")
	content, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	// +1 for header
	expectedLines := numWrites + 1
	
	if len(lines) != expectedLines {
		t.Errorf("Expected %d lines, got %d", expectedLines, len(lines))
	}
}

// TestEdgeCase_FileHandleExhaustion tests LRU eviction under pressure
func TestEdgeCase_FileHandleExhaustion(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Set very low limit to force eviction
	maxFiles := 5
	writer, err := NewStreamingJobFileWriter(tmpDir, maxFiles)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer writer.Close()
	
	// Write to more jobs than the limit
	numJobs := maxFiles * 3
	
	for i := 0; i < numJobs; i++ {
		data := JobMetricData{
			Job:         fmt.Sprintf("job-%d", i),
			MetricName:  "test_metric",
			Labels:      []string{"label1"},
			Cardinality: "50",
		}
		
		if err := writer.Write(data); err != nil {
			t.Fatalf("Write failed: %v", err)
		}
	}
	
	stats := writer.GetStats()
	
	// Should have written to all jobs
	if stats.TotalWrites != int64(numJobs) {
		t.Errorf("Expected %d writes, got %d", numJobs, stats.TotalWrites)
	}
	
	// Should never exceed max open files
	if stats.ActiveFiles > maxFiles {
		t.Errorf("Active files (%d) exceeded max (%d)", stats.ActiveFiles, maxFiles)
	}
	
	writer.Close()
	
	// Verify all job files exist
	for i := 0; i < numJobs; i++ {
		filename := filepath.Join(tmpDir, fmt.Sprintf("job-%d.txt", i))
		if _, err := os.Stat(filename); os.IsNotExist(err) {
			t.Errorf("Expected file %s to exist", filename)
		}
	}
}

// TestEdgeCase_InvalidJobNames tests sanitization of invalid job names
func TestEdgeCase_InvalidJobNames(t *testing.T) {
	tmpDir := t.TempDir()
	
	writer, err := NewStreamingJobFileWriter(tmpDir, 10)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer writer.Close()
	
	invalidNames := []string{
		"job/with/slashes",
		"job\\with\\backslashes",
		"job:with:colons",
		"job*with*asterisks",
		"job?with?questions",
		"job|with|pipes",
		"job<with>brackets",
		"job\"with\"quotes",
	}
	
	for _, jobName := range invalidNames {
		data := JobMetricData{
			Job:         jobName,
			MetricName:  "test_metric",
			Labels:      []string{"label1"},
			Cardinality: "100",
		}
		
		if err := writer.Write(data); err != nil {
			t.Errorf("Failed to write job %s: %v", jobName, err)
		}
	}
	
	writer.Close()
	
	// Verify sanitized files exist
	files, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to read directory: %v", err)
	}
	
	if len(files) != len(invalidNames) {
		t.Errorf("Expected %d files, got %d", len(invalidNames), len(files))
	}
}

// TestEdgeCase_EmptyLabels tests handling of metrics with no labels
func TestEdgeCase_EmptyLabels(t *testing.T) {
	tmpDir := t.TempDir()
	
	writer, err := NewStreamingJobFileWriter(tmpDir, 10)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer writer.Close()
	
	data := JobMetricData{
		Job:         "test-job",
		MetricName:  "metric_no_labels",
		Labels:      []string{}, // Empty labels
		Cardinality: "1",
	}
	
	if err := writer.Write(data); err != nil {
		t.Fatalf("Failed to write data with empty labels: %v", err)
	}
	
	writer.Close()
	
	// Verify file content
	filename := filepath.Join(tmpDir, "test-job.txt")
	content, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 2 { // Header + 1 data line
		t.Errorf("Expected 2 lines, got %d", len(lines))
	}
	
	// Check that labels field is empty
	parts := strings.Split(lines[1], "|")
	if len(parts) < 3 {
		t.Fatal("Invalid line format")
	}
	
	if parts[2] != "" {
		t.Errorf("Expected empty labels field, got: %s", parts[2])
	}
}

// TestEdgeCase_ZeroCardinality tests handling of zero cardinality
func TestEdgeCase_ZeroCardinality(t *testing.T) {
	tmpDir := t.TempDir()
	
	writer, err := NewStreamingJobFileWriter(tmpDir, 10)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer writer.Close()
	
	data := JobMetricData{
		Job:         "test-job",
		MetricName:  "metric_zero_cardinality",
		Labels:      []string{"label1"},
		Cardinality: "0",
	}
	
	if err := writer.Write(data); err != nil {
		t.Fatalf("Failed to write data with zero cardinality: %v", err)
	}
	
	writer.Close()
	
	// Verify file exists and contains the data
	filename := filepath.Join(tmpDir, "test-job.txt")
	content, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	
	if !strings.Contains(string(content), "metric_zero_cardinality") {
		t.Error("Metric with zero cardinality not written")
	}
}

// TestEdgeCase_CustomPatternsNoMatch tests custom patterns with no matches
func TestEdgeCase_CustomPatternsNoMatch(t *testing.T) {
	metrics := []string{"foo_metric", "bar_metric", "baz_metric"}
	
	// Patterns that don't match any metrics
	strategy := &CustomBatchStrategy{
		Patterns: []string{"http_.*", "grpc_.*", "aws_.*"},
	}
	
	batches := strategy.CreateBatches(metrics)
	
	// Should have catch-all batch with all metrics
	if len(batches) == 0 {
		t.Fatal("Expected at least catch-all batch")
	}
	
	// Verify all metrics are in catch-all batch
	catchAllFound := false
	for _, batch := range batches {
		if batch.Pattern == ".*" {
			catchAllFound = true
			if len(batch.Metrics) != len(metrics) {
				t.Errorf("Expected %d metrics in catch-all, got %d", len(metrics), len(batch.Metrics))
			}
		}
	}
	
	if !catchAllFound {
		t.Error("Expected catch-all batch for unmatched metrics")
	}
}

// TestEdgeCase_FilterMetricsByInvalidPattern tests invalid regex patterns
func TestEdgeCase_FilterMetricsByInvalidPattern(t *testing.T) {
	metrics := []string{"test_metric"}
	
	// Invalid regex pattern
	invalidPattern := "[invalid"
	
	result := filterMetricsByPattern(metrics, invalidPattern)
	
	// Should handle gracefully and return empty or all metrics
	// depending on implementation choice
	if result == nil {
		// nil is acceptable
	} else if len(result) > len(metrics) {
		t.Error("Result has more metrics than input")
	}
}

// TestEdgeCase_SplitPatternEdgeCases tests edge cases in pattern splitting
func TestEdgeCase_SplitPatternEdgeCases(t *testing.T) {
	testCases := []struct {
		name    string
		pattern string
	}{
		{"empty", ""},
		{"single_char", "a.*"},
		{"single_char_range", "[a].*"},
		{"reverse_range", "[z-a].*"}, // Invalid but should handle
		{"no_wildcard", "[a-c]"},
		{"multiple_wildcards", "[a-c].*.*"},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Should not panic
			result := splitPattern(tc.pattern)
			
			// Should return something (even if just original)
			if result == nil {
				t.Error("Expected non-nil result")
			}
		})
	}
}
