package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// AnalysisUploadConfig contains configuration for uploading analysis results
type AnalysisUploadConfig struct {
	Bucket       string
	Prefix       string
	Region       string
	JobMetricsDir string
	ErrorFile    string
	Timestamp    string
}

// EvaluationUploadConfig contains configuration for uploading evaluation results
type EvaluationUploadConfig struct {
	Bucket         string
	Prefix         string
	Region         string
	RunID          string
	JSONFile       string
	HTMLFile       string
	PrometheusFile string
	OutputFormats  []string
	Manifest       *EvaluationManifest
}

// EvaluationDownloadConfig contains configuration for downloading from S3
type EvaluationDownloadConfig struct {
	Bucket string
	Prefix string
	Region string
}

// EvaluationManifest contains metadata about an evaluation run
type EvaluationManifest struct {
	Timestamp        string  `json:"timestamp"`
	RunID            string  `json:"run_id"`
	TotalJobs        int     `json:"total_jobs"`
	AverageScore     float64 `json:"average_score"`
	TotalCardinality int64   `json:"total_cardinality"`
	TotalCost        float64 `json:"total_cost,omitempty"`
	RulesConfig      string  `json:"rules_config"`
	OutputFormats    string  `json:"output_formats"`
	SourceType       string  `json:"source_type"`
	SourcePath       string  `json:"source_path,omitempty"`
	Files            struct {
		JSON       string `json:"json,omitempty"`
		HTML       string `json:"html,omitempty"`
		Prometheus string `json:"prometheus,omitempty"`
		Manifest   string `json:"manifest"`
	} `json:"files"`
}

// UploadAnalysisResults uploads analysis results to S3
func UploadAnalysisResults(config AnalysisUploadConfig) error {
	s3Client, err := NewS3Client(config.Bucket, config.Prefix, config.Region)
	if err != nil {
		return fmt.Errorf("failed to create S3 client: %w", err)
	}

	s3Prefix := fmt.Sprintf("job_metrics_%s", config.Timestamp)
	uploadedFiles, err := s3Client.UploadDirectory(config.JobMetricsDir, s3Prefix)
	if err != nil {
		return fmt.Errorf("failed to upload job metrics directory: %w", err)
	}

	fmt.Printf("Uploaded %d job metric files to %s\n", len(uploadedFiles), s3Client.GetS3URI(s3Prefix))

	if _, err := os.Stat(config.ErrorFile); err == nil {
		errorS3Key := fmt.Sprintf("metrics_errors_%s.txt", config.Timestamp)
		if err := s3Client.UploadFile(config.ErrorFile, errorS3Key); err != nil {
			fmt.Printf("WARNING: Failed to upload error file: %v\n", err)
		} else {
			fmt.Printf("Uploaded error file to %s\n", s3Client.GetS3URI(errorS3Key))
		}
	}

	fmt.Printf("\nS3 Location: s3://%s/%s/job_metrics_%s/\n", config.Bucket, config.Prefix, config.Timestamp)
	return nil
}

// DownloadEvaluationSource downloads job metrics from S3 for evaluation
func DownloadEvaluationSource(config EvaluationDownloadConfig) (string, error) {
	s3Client, err := NewS3Client(config.Bucket, config.Prefix, config.Region)
	if err != nil {
		return "", fmt.Errorf("failed to create S3 client: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "instrumentation-score-s3-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	fmt.Printf("Downloading job metrics from S3...\n")
	fmt.Printf("S3 Location: s3://%s/%s\n", config.Bucket, config.Prefix)

	downloadedFiles, err := s3Client.DownloadDirectory(config.Prefix, tmpDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("failed to download from S3: %w", err)
	}

	fmt.Printf("Downloaded %d files\n", len(downloadedFiles))
	return tmpDir, nil
}

// UploadEvaluationResults uploads evaluation results to S3 with manifest
func UploadEvaluationResults(config EvaluationUploadConfig) error {
	s3Client, err := NewS3Client(config.Bucket, config.Prefix, config.Region)
	if err != nil {
		return fmt.Errorf("failed to create S3 client: %w", err)
	}

	// Generate run ID if not provided
	runID := config.RunID
	if runID == "" {
		timestamp := time.Now().Format("20060102_150405")
		runID = fmt.Sprintf("evaluation_%s", timestamp)
	}

	s3Prefix := fmt.Sprintf("evaluations/%s", runID)

	// Update manifest
	if config.Manifest == nil {
		config.Manifest = &EvaluationManifest{}
	}
	config.Manifest.RunID = runID
	if config.Manifest.Timestamp == "" {
		config.Manifest.Timestamp = time.Now().Format(time.RFC3339)
	}

	// Upload JSON if provided
	if config.JSONFile != "" && contains(config.OutputFormats, "json") {
		s3Key := fmt.Sprintf("%s/report.json", s3Prefix)
		if err := s3Client.UploadFile(config.JSONFile, s3Key); err != nil {
			return fmt.Errorf("failed to upload JSON: %w", err)
		}
		config.Manifest.Files.JSON = s3Key
		fmt.Printf("✅ Uploaded JSON report to %s\n", s3Client.GetS3URI(s3Key))
	}

	// Upload HTML if provided
	if config.HTMLFile != "" && contains(config.OutputFormats, "html") {
		s3Key := fmt.Sprintf("%s/dashboard.html", s3Prefix)
		if err := s3Client.UploadFile(config.HTMLFile, s3Key); err != nil {
			return fmt.Errorf("failed to upload HTML: %w", err)
		}
		config.Manifest.Files.HTML = s3Key
		fmt.Printf("✅ Uploaded HTML dashboard to %s\n", s3Client.GetS3URI(s3Key))
	}

	// Upload Prometheus metrics if provided
	if config.PrometheusFile != "" && contains(config.OutputFormats, "prometheus") {
		s3Key := fmt.Sprintf("%s/metrics.prom", s3Prefix)
		if err := s3Client.UploadFile(config.PrometheusFile, s3Key); err != nil {
			return fmt.Errorf("failed to upload Prometheus metrics: %w", err)
		}
		config.Manifest.Files.Prometheus = s3Key
		fmt.Printf("✅ Uploaded Prometheus metrics to %s\n", s3Client.GetS3URI(s3Key))
	}

	// Upload manifest
	manifestS3Key := fmt.Sprintf("%s/manifest.json", s3Prefix)
	config.Manifest.Files.Manifest = manifestS3Key
	manifestData, err := json.MarshalIndent(config.Manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	if err := s3Client.UploadContent(manifestData, manifestS3Key); err != nil {
		return fmt.Errorf("failed to upload manifest: %w", err)
	}
	fmt.Printf("✅ Uploaded manifest to %s\n", s3Client.GetS3URI(manifestS3Key))

	fmt.Printf("\n📦 Evaluation Package: s3://%s/%s/\n", config.Bucket, s3Prefix)
	fmt.Printf("   Run ID: %s\n", runID)
	fmt.Printf("   Timestamp: %s\n", config.Manifest.Timestamp)
	fmt.Printf("   Total Jobs: %d\n", config.Manifest.TotalJobs)
	fmt.Printf("   Average Score: %.2f%%\n", config.Manifest.AverageScore)
	if config.Manifest.TotalCost > 0 {
		fmt.Printf("   Total Cost: $%.2f/month\n", config.Manifest.TotalCost)
	}

	return nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, item) {
			return true
		}
	}
	return false
}

