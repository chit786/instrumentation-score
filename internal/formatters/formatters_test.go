package formatters_test

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"instrumentation-score/internal/engine"
	"instrumentation-score/internal/formatters"
)

func TestPrometheusMetrics(t *testing.T) {
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Test data
	serviceName := "test-service"
	score := 87.5
	results := []engine.RuleResult{
		{RuleID: "TEST-001", Impact: "Important", PassedChecks: 1, TotalChecks: 1},
		{RuleID: "TEST-002", Impact: "Critical", PassedChecks: 2, TotalChecks: 2},
	}

	// Call function
	formatters.PrometheusMetrics(serviceName, score, results)

	// Restore stdout
	w.Close()
	os.Stdout = old

	// Read output
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("Failed to read output: %v", err)
	}
	output := buf.String()

	// Verify output contains expected metrics
	expectedMetrics := []string{
		"instrumentation_score{service_name=\"test-service\"} 87.5",
		"instrumentation_rule_checks_total{service_name=\"test-service\",rule_id=\"TEST-001\",impact=\"Important\"} 1",
		"instrumentation_rule_checks_total{service_name=\"test-service\",rule_id=\"TEST-002\",impact=\"Critical\"} 2",
		"instrumentation_rule_failures_total{service_name=\"test-service\",rule_id=\"TEST-001\",impact=\"Important\"} 0",
		"instrumentation_rule_failures_total{service_name=\"test-service\",rule_id=\"TEST-002\",impact=\"Critical\"} 0",
	}

	for _, expected := range expectedMetrics {
		if !contains(output, expected) {
			t.Errorf("Expected output to contain: %s", expected)
		}
	}
}

func TestJSON(t *testing.T) {
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Test data
	serviceName := "test-service"
	score := 87.5
	results := []engine.RuleResult{
		{RuleID: "TEST-001", Impact: "Important", PassedChecks: 1, TotalChecks: 1, FailedChecks: []string{}},
	}

	// Call function
	formatters.JSON(serviceName, score, results)

	// Restore stdout
	w.Close()
	os.Stdout = old

	// Read output
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("Failed to read output: %v", err)
	}
	jsonOutput := buf.String()

	// Verify JSON structure - check for key-value pairs with flexible spacing
	expectedFields := []string{
		"\"service_name\":",
		"\"test-service\"",
		"\"score\":",
		"87.5",
		"\"category\":",
		"\"Good\"",
		"\"rule_results\":",
		"\"RuleID\":",
		"\"TEST-001\"",
		"\"Impact\":",
		"\"Important\"",
		"\"PassedChecks\":",
		"1",
		"\"TotalChecks\":",
		"1",
	}

	for _, field := range expectedFields {
		if !contains(jsonOutput, field) {
			t.Errorf("Expected JSON to contain: %s\nGot: %s", field, jsonOutput)
		}
	}
}

func TestText(t *testing.T) {
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Test data
	serviceName := "test-service"
	score := 87.5
	results := []engine.RuleResult{
		{RuleID: "TEST-001", Impact: "Important", PassedMetrics: 1, TotalMetrics: 1, FailedChecks: []string{}},
		{RuleID: "TEST-002", Impact: "Critical", PassedMetrics: 1, TotalMetrics: 2, FailedChecks: []string{"check1"}},
	}

	// Call function
	formatters.Text(serviceName, score, results)

	// Restore stdout
	w.Close()
	os.Stdout = old

	// Read output
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("Failed to read output: %v", err)
	}
	textOutput := buf.String()

	// Verify text output
	expectedLines := []string{
		"Instrumentation Score Report for test-service",
		"Overall Score: 87.5/100 (Good)",
		"Rule Evaluation Results:",
		"Rule TEST-001 (Important): 1/1 metrics passed (100.0%)",
		"Rule TEST-002 (Critical): 1/2 metrics passed (50.0%)",
	}

	for _, line := range expectedLines {
		if !contains(textOutput, line) {
			t.Errorf("Expected text output to contain: %s", line)
		}
	}
}

func TestGetScoreCategory(t *testing.T) {
	tests := []struct {
		score    float64
		expected string
	}{
		{95.0, "Excellent"},
		{85.0, "Good"},
		{65.0, "Needs Improvement"},
		{25.0, "Poor"},
		{90.0, "Excellent"},
		{75.0, "Good"},
		{50.0, "Needs Improvement"},
		{0.0, "Poor"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("score_%.1f", tt.score), func(t *testing.T) {
			// We need to test the private function through the public interface
			// Since getScoreCategory is private, we'll test it indirectly through Text output
			old := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			formatters.Text("test", tt.score, []engine.RuleResult{})

			w.Close()
			os.Stdout = old

			var buf bytes.Buffer
			if _, err := buf.ReadFrom(r); err != nil {
				t.Fatalf("Failed to read output: %v", err)
			}
			textOutput := buf.String()

			expectedCategory := fmt.Sprintf("Overall Score: %.1f/100 (%s)", tt.score, tt.expected)
			if !contains(textOutput, expectedCategory) {
				t.Errorf("Expected category %s for score %.1f, got output: %s", tt.expected, tt.score, textOutput)
			}
		})
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && (s[:len(substr)] == substr ||
			s[len(s)-len(substr):] == substr ||
			containsSubstring(s, substr))))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
