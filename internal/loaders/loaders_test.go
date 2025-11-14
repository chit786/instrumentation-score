package loaders

import (
	"os"
	"testing"
)

func TestLoadCardinalityReport(t *testing.T) {
	// Create a temporary file with test data
	content := `http_requests_total|1500
http_request_duration_seconds|2500
memory_usage_bytes|500`

	tmpFile, err := os.CreateTemp("", "test_cardinality_*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("Failed to write test data: %v", err)
	}
	tmpFile.Close()

	// Test loading the report
	data, err := LoadCardinalityReport(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load cardinality report: %v", err)
	}

	// Verify the data
	if len(data) != 3 {
		t.Errorf("Expected 3 items, got %d", len(data))
	}

	expectedData := []CardinalityData{
		{MetricName: "http_requests_total", Count: 1500},
		{MetricName: "http_request_duration_seconds", Count: 2500},
		{MetricName: "memory_usage_bytes", Count: 500},
	}

	for i, expected := range expectedData {
		if data[i].MetricName != expected.MetricName {
			t.Errorf("Expected metric name %s, got %s", expected.MetricName, data[i].MetricName)
		}
		if data[i].Count != expected.Count {
			t.Errorf("Expected count %d, got %d", expected.Count, data[i].Count)
		}
	}
}

func TestLoadLabelsReport(t *testing.T) {
	// Create a temporary file with test data
	content := `"http_requests_total"|"method,status,path"
"http_request_duration_seconds"|"method,status"
"memory_usage_bytes"|"type"`

	tmpFile, err := os.CreateTemp("", "test_labels_*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("Failed to write test data: %v", err)
	}
	tmpFile.Close()

	// Test loading the report
	data, err := LoadLabelsReport(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load labels report: %v", err)
	}

	// Verify the data
	if len(data) != 3 {
		t.Errorf("Expected 3 items, got %d", len(data))
	}

	expectedLabels := map[string][]string{
		"http_requests_total":           {"method", "status", "path"},
		"http_request_duration_seconds": {"method", "status"},
		"memory_usage_bytes":            {"type"},
	}

	for _, item := range data {
		expected, exists := expectedLabels[item.MetricName]
		if !exists {
			t.Errorf("Unexpected metric: %s", item.MetricName)
			continue
		}

		if len(item.Labels) != len(expected) {
			t.Errorf("Expected %d labels for %s, got %d", len(expected), item.MetricName, len(item.Labels))
			continue
		}

		for i, label := range item.Labels {
			if label != expected[i] {
				t.Errorf("Expected label %s, got %s", expected[i], label)
			}
		}
	}
}

func TestLoadCardinalityReport_InvalidFile(t *testing.T) {
	_, err := LoadCardinalityReport("nonexistent.txt")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestLoadLabelsReport_InvalidFile(t *testing.T) {
	_, err := LoadLabelsReport("nonexistent.txt")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestLoadJobMetricReport(t *testing.T) {
	// Create a temporary file with test data
	content := `JOB|METRIC_NAME|LABELS|CARDINALITY
api-service|http_requests_total|method,status,endpoint|1500
api-service|http_request_duration_seconds|method,endpoint,le|2400
api-service|database_queries_total|query_type,table|800`

	tmpFile, err := os.CreateTemp("", "test_job_metrics_*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("Failed to write test data: %v", err)
	}
	tmpFile.Close()

	// Test loading the report
	data, err := LoadJobMetricReport(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load job metric report: %v", err)
	}

	// Verify the data
	if len(data) != 3 {
		t.Errorf("Expected 3 items, got %d", len(data))
	}

	// Check first entry
	if data[0].Job != "api-service" {
		t.Errorf("Expected job 'api-service', got '%s'", data[0].Job)
	}
	if data[0].MetricName != "http_requests_total" {
		t.Errorf("Expected metric 'http_requests_total', got '%s'", data[0].MetricName)
	}
	if data[0].Cardinality != 1500 {
		t.Errorf("Expected cardinality 1500, got %d", data[0].Cardinality)
	}
	if len(data[0].Labels) != 3 {
		t.Errorf("Expected 3 labels, got %d", len(data[0].Labels))
	}

	expectedLabels := []string{"method", "status", "endpoint"}
	for i, label := range data[0].Labels {
		if label != expectedLabels[i] {
			t.Errorf("Expected label '%s', got '%s'", expectedLabels[i], label)
		}
	}
}

func TestConvertJobMetricToCardinality(t *testing.T) {
	jobData := []JobMetricData{
		{Job: "api-service", MetricName: "http_requests_total", Labels: []string{"method", "status"}, Cardinality: 1500},
		{Job: "api-service", MetricName: "database_queries_total", Labels: []string{"query_type"}, Cardinality: 800},
	}

	cardinalityData := ConvertJobMetricToCardinality(jobData)

	if len(cardinalityData) != 2 {
		t.Errorf("Expected 2 items, got %d", len(cardinalityData))
	}

	if cardinalityData[0].MetricName != "http_requests_total" {
		t.Errorf("Expected metric 'http_requests_total', got '%s'", cardinalityData[0].MetricName)
	}
	if cardinalityData[0].Count != 1500 {
		t.Errorf("Expected count 1500, got %d", cardinalityData[0].Count)
	}
}

func TestConvertJobMetricToLabels(t *testing.T) {
	jobData := []JobMetricData{
		{Job: "api-service", MetricName: "http_requests_total", Labels: []string{"method", "status"}, Cardinality: 1500},
		{Job: "api-service", MetricName: "database_queries_total", Labels: []string{"query_type"}, Cardinality: 800},
	}

	labelsData := ConvertJobMetricToLabels(jobData)

	if len(labelsData) != 2 {
		t.Errorf("Expected 2 items, got %d", len(labelsData))
	}

	if labelsData[0].MetricName != "http_requests_total" {
		t.Errorf("Expected metric 'http_requests_total', got '%s'", labelsData[0].MetricName)
	}
	if len(labelsData[0].Labels) != 2 {
		t.Errorf("Expected 2 labels, got %d", len(labelsData[0].Labels))
	}
	if labelsData[0].Labels[0] != "method" || labelsData[0].Labels[1] != "status" {
		t.Errorf("Expected labels [method, status], got %v", labelsData[0].Labels)
	}
}

func TestLoadJobMetricReport_InvalidFile(t *testing.T) {
	_, err := LoadJobMetricReport("nonexistent.txt")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}
