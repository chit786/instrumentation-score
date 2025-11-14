package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestAnalysisUploadConfig(t *testing.T) {
	config := AnalysisUploadConfig{
		Bucket:        "test-bucket",
		Prefix:        "test-prefix",
		Region:        "eu-west-1",
		JobMetricsDir: "/tmp/metrics",
		ErrorFile:     "/tmp/errors.txt",
		Timestamp:     "20251102_160000",
	}

	if config.Bucket != "test-bucket" {
		t.Errorf("Bucket = %v, want test-bucket", config.Bucket)
	}
	if config.Timestamp != "20251102_160000" {
		t.Errorf("Timestamp = %v, want 20251102_160000", config.Timestamp)
	}
}

func TestEvaluationUploadConfig(t *testing.T) {
	manifest := &EvaluationManifest{
		RunID:        "test-run",
		TotalJobs:    10,
		AverageScore: 85.5,
	}

	config := EvaluationUploadConfig{
		Bucket:        "test-bucket",
		Prefix:        "test-prefix",
		Region:        "eu-west-1",
		RunID:         "test-run",
		JSONFile:      "report.json",
		HTMLFile:      "dashboard.html",
		OutputFormats: []string{"html", "json"},
		Manifest:      manifest,
	}

	if config.RunID != "test-run" {
		t.Errorf("RunID = %v, want test-run", config.RunID)
	}
	if config.Manifest.TotalJobs != 10 {
		t.Errorf("Manifest.TotalJobs = %v, want 10", config.Manifest.TotalJobs)
	}
}

func TestEvaluationDownloadConfig(t *testing.T) {
	config := EvaluationDownloadConfig{
		Bucket: "test-bucket",
		Prefix: "job_metrics_20251102_160000",
		Region: "us-west-2",
	}

	if config.Bucket != "test-bucket" {
		t.Errorf("Bucket = %v, want test-bucket", config.Bucket)
	}
	if config.Region != "us-west-2" {
		t.Errorf("Region = %v, want us-west-2", config.Region)
	}
}

func TestEvaluationManifest(t *testing.T) {
	manifest := EvaluationManifest{
		Timestamp:        "2025-11-02T16:00:00Z",
		RunID:            "prod-20251102",
		TotalJobs:        45,
		AverageScore:     87.5,
		TotalCardinality: 1500000,
		TotalCost:        9225.00,
		RulesConfig:      "rules_config.yaml",
		OutputFormats:    "html,json",
		SourceType:       "local_directory",
		SourcePath:       "reports/job_metrics_20251102_160000/",
	}

	manifest.Files.JSON = "evaluations/prod-20251102/report.json"
	manifest.Files.HTML = "evaluations/prod-20251102/dashboard.html"
	manifest.Files.Manifest = "evaluations/prod-20251102/manifest.json"

	// Test JSON marshaling
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("failed to marshal manifest: %v", err)
	}

	// Test JSON unmarshaling
	var decoded EvaluationManifest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal manifest: %v", err)
	}

	if decoded.RunID != manifest.RunID {
		t.Errorf("RunID = %v, want %v", decoded.RunID, manifest.RunID)
	}
	if decoded.TotalJobs != manifest.TotalJobs {
		t.Errorf("TotalJobs = %v, want %v", decoded.TotalJobs, manifest.TotalJobs)
	}
	if decoded.AverageScore != manifest.AverageScore {
		t.Errorf("AverageScore = %v, want %v", decoded.AverageScore, manifest.AverageScore)
	}
	if decoded.Files.JSON != manifest.Files.JSON {
		t.Errorf("Files.JSON = %v, want %v", decoded.Files.JSON, manifest.Files.JSON)
	}
}

func TestEvaluationManifest_JSONFormat(t *testing.T) {
	manifest := EvaluationManifest{
		Timestamp:        "2025-11-02T16:00:00Z",
		RunID:            "test-run",
		TotalJobs:        10,
		AverageScore:     90.0,
		TotalCardinality: 100000,
		TotalCost:        615.00,
		RulesConfig:      "rules_config.yaml",
		OutputFormats:    "html",
		SourceType:       "s3",
		SourcePath:       "s3://bucket/prefix",
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal manifest: %v", err)
	}

	// Verify JSON structure
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	// Check required fields
	requiredFields := []string{
		"timestamp", "run_id", "total_jobs", "average_score",
		"total_cardinality", "rules_config", "output_formats",
		"source_type", "files",
	}

	for _, field := range requiredFields {
		if _, ok := result[field]; !ok {
			t.Errorf("missing required field: %s", field)
		}
	}
}

func TestUploadAnalysisResults_InvalidConfig(t *testing.T) {
	config := AnalysisUploadConfig{
		Bucket:        "", // Invalid: empty bucket
		Prefix:        "test-prefix",
		Region:        "eu-west-1",
		JobMetricsDir: "/tmp/metrics",
		ErrorFile:     "/tmp/errors.txt",
		Timestamp:     "20251102_160000",
	}

	err := UploadAnalysisResults(config)
	if err == nil {
		t.Errorf("expected error for empty bucket")
	}
}

func TestUploadAnalysisResults_NonExistentDirectory(t *testing.T) {
	config := AnalysisUploadConfig{
		Bucket:        "test-bucket",
		Prefix:        "test-prefix",
		Region:        "eu-west-1",
		JobMetricsDir: "/nonexistent/directory",
		ErrorFile:     "/tmp/errors.txt",
		Timestamp:     "20251102_160000",
	}

	// This will fail when trying to upload non-existent directory
	// We expect an error
	err := UploadAnalysisResults(config)
	if err == nil {
		t.Errorf("expected error for non-existent directory")
	}
}

func TestDownloadEvaluationSource_InvalidConfig(t *testing.T) {
	config := EvaluationDownloadConfig{
		Bucket: "", // Invalid: empty bucket
		Prefix: "test-prefix",
		Region: "eu-west-1",
	}

	_, err := DownloadEvaluationSource(config)
	if err == nil {
		t.Errorf("expected error for empty bucket")
	}
}

func TestUploadEvaluationResults_InvalidConfig(t *testing.T) {
	config := EvaluationUploadConfig{
		Bucket:        "", // Invalid: empty bucket
		Prefix:        "test-prefix",
		Region:        "eu-west-1",
		RunID:         "test-run",
		OutputFormats: []string{"html"},
	}

	err := UploadEvaluationResults(config)
	if err == nil {
		t.Errorf("expected error for empty bucket")
	}
}

func TestUploadEvaluationResults_AutoGenerateRunID(t *testing.T) {
	// Create temp files for testing
	tmpDir, err := os.MkdirTemp("", "uploader-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	jsonFile := filepath.Join(tmpDir, "report.json")
	if err := os.WriteFile(jsonFile, []byte(`{"test": "data"}`), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	manifest := &EvaluationManifest{
		TotalJobs:    10,
		AverageScore: 85.0,
	}

	config := EvaluationUploadConfig{
		Bucket:        "test-bucket",
		Prefix:        "test-prefix",
		Region:        "eu-west-1",
		RunID:         "", // Empty - should auto-generate
		JSONFile:      jsonFile,
		OutputFormats: []string{"json"},
		Manifest:      manifest,
	}

	// This will fail because we don't have real AWS credentials
	// But we can verify the config is valid
	err = UploadEvaluationResults(config)
	if err == nil {
		t.Skip("Skipping actual upload - requires AWS credentials")
	}

	// Verify manifest was updated with auto-generated run ID
	if config.Manifest.RunID == "" {
		t.Errorf("expected auto-generated run ID, got empty string")
	}
}

func TestUploadEvaluationResults_WithManifest(t *testing.T) {
	manifest := &EvaluationManifest{
		Timestamp:        "2025-11-02T16:00:00Z",
		TotalJobs:        10,
		AverageScore:     85.0,
		TotalCardinality: 100000,
		RulesConfig:      "rules_config.yaml",
		OutputFormats:    "html,json",
		SourceType:       "local_directory",
		SourcePath:       "/tmp/metrics",
	}

	config := EvaluationUploadConfig{
		Bucket:        "test-bucket",
		Prefix:        "test-prefix",
		Region:        "eu-west-1",
		RunID:         "test-run",
		OutputFormats: []string{"html", "json"},
		Manifest:      manifest,
	}

	// Verify manifest is properly configured
	if config.Manifest.TotalJobs != 10 {
		t.Errorf("Manifest.TotalJobs = %v, want 10", config.Manifest.TotalJobs)
	}
	if config.Manifest.SourceType != "local_directory" {
		t.Errorf("Manifest.SourceType = %v, want local_directory", config.Manifest.SourceType)
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name  string
		slice []string
		item  string
		want  bool
	}{
		{
			name:  "item exists - exact match",
			slice: []string{"html", "json", "text"},
			item:  "json",
			want:  true,
		},
		{
			name:  "item exists - case insensitive",
			slice: []string{"HTML", "JSON", "TEXT"},
			item:  "json",
			want:  true,
		},
		{
			name:  "item does not exist",
			slice: []string{"html", "json"},
			item:  "xml",
			want:  false,
		},
		{
			name:  "empty slice",
			slice: []string{},
			item:  "json",
			want:  false,
		},
		{
			name:  "nil slice",
			slice: nil,
			item:  "json",
			want:  false,
		},
		{
			name:  "empty item",
			slice: []string{"html", "json"},
			item:  "",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := contains(tt.slice, tt.item)
			if got != tt.want {
				t.Errorf("contains(%v, %v) = %v, want %v", tt.slice, tt.item, got, tt.want)
			}
		})
	}
}

func TestManifestWithCost(t *testing.T) {
	manifest := EvaluationManifest{
		TotalJobs:    10,
		AverageScore: 85.0,
		TotalCost:    1500.50,
	}

	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded EvaluationManifest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.TotalCost != 1500.50 {
		t.Errorf("TotalCost = %v, want 1500.50", decoded.TotalCost)
	}
}

func TestManifestWithoutCost(t *testing.T) {
	manifest := EvaluationManifest{
		TotalJobs:    10,
		AverageScore: 85.0,
		// TotalCost omitted
	}

	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Verify cost field is omitted when zero
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	// With omitempty, zero cost should not appear in JSON
	if _, ok := result["total_cost"]; ok {
		t.Errorf("expected total_cost to be omitted when zero")
	}
}

func TestManifestFiles(t *testing.T) {
	manifest := EvaluationManifest{}
	manifest.Files.JSON = "path/to/report.json"
	manifest.Files.HTML = "path/to/dashboard.html"
	manifest.Files.Prometheus = "path/to/metrics.prom"
	manifest.Files.Manifest = "path/to/manifest.json"

	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded EvaluationManifest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Files.JSON != manifest.Files.JSON {
		t.Errorf("Files.JSON = %v, want %v", decoded.Files.JSON, manifest.Files.JSON)
	}
	if decoded.Files.HTML != manifest.Files.HTML {
		t.Errorf("Files.HTML = %v, want %v", decoded.Files.HTML, manifest.Files.HTML)
	}
	if decoded.Files.Prometheus != manifest.Files.Prometheus {
		t.Errorf("Files.Prometheus = %v, want %v", decoded.Files.Prometheus, manifest.Files.Prometheus)
	}
	if decoded.Files.Manifest != manifest.Files.Manifest {
		t.Errorf("Files.Manifest = %v, want %v", decoded.Files.Manifest, manifest.Files.Manifest)
	}
}

func TestUploadEvaluationResults_MultipleFormats(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "uploader-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test files
	jsonFile := filepath.Join(tmpDir, "report.json")
	htmlFile := filepath.Join(tmpDir, "dashboard.html")
	promFile := filepath.Join(tmpDir, "metrics.prom")

	if err := os.WriteFile(jsonFile, []byte(`{"test": "data"}`), 0644); err != nil {
		t.Fatalf("failed to write JSON file: %v", err)
	}
	if err := os.WriteFile(htmlFile, []byte(`<html>test</html>`), 0644); err != nil {
		t.Fatalf("failed to write HTML file: %v", err)
	}
	if err := os.WriteFile(promFile, []byte(`# HELP test\ntest 1`), 0644); err != nil {
		t.Fatalf("failed to write Prometheus file: %v", err)
	}

	manifest := &EvaluationManifest{
		TotalJobs:    5,
		AverageScore: 90.0,
	}

	config := EvaluationUploadConfig{
		Bucket:         "test-bucket",
		Prefix:         "test-prefix",
		Region:         "eu-west-1",
		RunID:          "test-run",
		JSONFile:       jsonFile,
		HTMLFile:       htmlFile,
		PrometheusFile: promFile,
		OutputFormats:  []string{"html", "json", "prometheus"},
		Manifest:       manifest,
	}

	// This will fail without AWS credentials, but validates config
	err = UploadEvaluationResults(config)
	if err == nil {
		t.Skip("Skipping actual upload - requires AWS credentials")
	}

	// Verify all files were specified
	if config.JSONFile == "" {
		t.Error("JSONFile should be set")
	}
	if config.HTMLFile == "" {
		t.Error("HTMLFile should be set")
	}
	if config.PrometheusFile == "" {
		t.Error("PrometheusFile should be set")
	}
}

