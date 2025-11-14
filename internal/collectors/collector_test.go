package collectors

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWritePerJobFiles(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "collector_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name     string
		data     []JobMetricData
		wantErr  bool
		wantJobs []string
	}{
		{
			name: "write single job",
			data: []JobMetricData{
				{
					Job:         "api-service",
					MetricName:  "http_requests_total",
					Labels:      []string{"method", "status"},
					Cardinality: "100",
				},
				{
					Job:         "api-service",
					MetricName:  "http_request_duration_seconds",
					Labels:      []string{"method", "status", "endpoint"},
					Cardinality: "250",
				},
			},
			wantErr:  false,
			wantJobs: []string{"api-service.txt"},
		},
		{
			name: "write multiple jobs",
			data: []JobMetricData{
				{
					Job:         "api-service",
					MetricName:  "http_requests_total",
					Labels:      []string{"method"},
					Cardinality: "100",
				},
				{
					Job:         "web-service",
					MetricName:  "http_requests_total",
					Labels:      []string{"method"},
					Cardinality: "200",
				},
			},
			wantErr:  false,
			wantJobs: []string{"api-service.txt", "web-service.txt"},
		},
		{
			name:     "empty data",
			data:     []JobMetricData{},
			wantErr:  false,
			wantJobs: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDir := filepath.Join(tmpDir, tt.name)
			if err := os.MkdirAll(testDir, 0700); err != nil {
				t.Fatalf("failed to create test dir: %v", err)
			}

			err := WritePerJobFiles(testDir, tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("WritePerJobFiles() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Verify files were created
			for _, jobFile := range tt.wantJobs {
				filePath := filepath.Join(testDir, jobFile)
				if _, err := os.Stat(filePath); os.IsNotExist(err) {
					t.Errorf("expected file %s to exist", jobFile)
				}

				// Verify file content
				content, err := os.ReadFile(filePath)
				if err != nil {
					t.Errorf("failed to read file %s: %v", jobFile, err)
				}
				if len(content) == 0 {
					t.Errorf("file %s is empty", jobFile)
				}

				// Check for header
				contentStr := string(content)
				if len(contentStr) > 0 && contentStr[:3] != "JOB" {
					t.Errorf("file %s missing header", jobFile)
				}
			}
		})
	}
}

func TestWriteErrorsToFile(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "collector_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name    string
		errors  []ErrorRecord
		wantErr bool
	}{
		{
			name: "write single error",
			errors: []ErrorRecord{
				{
					MetricName: "test_metric",
					Operation:  "fetch",
					Error:      "connection timeout",
					Timestamp:  testTime,
				},
			},
			wantErr: false,
		},
		{
			name: "write multiple errors",
			errors: []ErrorRecord{
				{
					MetricName: "metric1",
					Operation:  "fetch",
					Error:      "error1",
					Timestamp:  testTime,
				},
				{
					MetricName: "metric2",
					Operation:  "parse",
					Error:      "error2",
					Timestamp:  testTime,
				},
			},
			wantErr: false,
		},
		{
			name:    "empty errors",
			errors:  []ErrorRecord{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errorFile := filepath.Join(tmpDir, tt.name+".txt")

			err := WriteErrorsToFile(errorFile, tt.errors)
			if (err != nil) != tt.wantErr {
				t.Errorf("WriteErrorsToFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Verify file was created
			if _, err := os.Stat(errorFile); os.IsNotExist(err) {
				t.Errorf("expected error file to exist")
			}

			// Verify file content
			content, err := os.ReadFile(errorFile)
			if err != nil {
				t.Errorf("failed to read error file: %v", err)
			}

			contentStr := string(content)
			if len(tt.errors) > 0 {
				// Check for header
				if len(contentStr) == 0 || contentStr[:9] != "TIMESTAMP" {
					t.Errorf("error file missing header")
				}

				// Check that each error is present
				for _, errRec := range tt.errors {
					if len(contentStr) > 0 && !contains(contentStr, errRec.MetricName) {
						t.Errorf("error file missing metric name: %s", errRec.MetricName)
					}
				}
			}
		})
	}
}

func TestNewCollector(t *testing.T) {
	tests := []struct {
		name         string
		baseURL      string
		login        string
		queryFilters string
	}{
		{
			name:         "create collector without filters",
			baseURL:      "http://prometheus.example.com",
			login:        "user:pass",
			queryFilters: "",
		},
		{
			name:         "create collector with filters",
			baseURL:      "http://prometheus.example.com",
			login:        "user:pass",
			queryFilters: "cluster=~\"prod.*\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collector := NewCollector(tt.baseURL, tt.login, tt.queryFilters)
			if collector == nil {
				t.Error("NewCollector() returned nil")
			}
			if collector.client == nil {
				t.Error("collector.client is nil")
			}
			if collector.queryFilters != tt.queryFilters {
				t.Errorf("collector.queryFilters = %v, want %v", collector.queryFilters, tt.queryFilters)
			}
			if collector.maxConcurrent != 5 {
				t.Errorf("collector.maxConcurrent = %v, want 5", collector.maxConcurrent)
			}
		})
	}
}

func TestSanitizeJobName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "job with slashes",
			input:    "integrations/kubernetes/kube-state-metrics",
			expected: "integrations_kubernetes_kube-state-metrics",
		},
		{
			name:     "job with multiple unsafe chars",
			input:    "my:job/with*unsafe?chars",
			expected: "my_job_with_unsafe_chars",
		},
		{
			name:     "normal job name",
			input:    "api-service",
			expected: "api-service",
		},
		{
			name:     "job with backslash",
			input:    "windows\\service",
			expected: "windows_service",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeJobName(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeJobName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// Helper functions
var testTime = mustParseTime("2024-01-01T12:00:00Z")

func mustParseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) >= len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
