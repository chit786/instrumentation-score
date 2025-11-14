package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"instrumentation-score/internal/collectors"
	"instrumentation-score/internal/storage"

	"github.com/spf13/cobra"
)

var (
	analyzeOutputDir    string
	analyzeQueryFilters string
	analyzeRetryCount   int
	analyzeS3Upload     bool
	analyzeS3Bucket     string
	analyzeS3Prefix     string
	analyzeS3Region     string
)

var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "Analyze Prometheus metrics and generate per-job reports",
	Long: `Analyze Prometheus metrics and generate comprehensive per-job reports.

This command fetches metrics from Prometheus, analyzes them by job, and generates:
- Per-job metric files with format: JOB|METRIC_NAME|LABELS|CARDINALITY
- Error report for any failures during analysis

The reports are written to a timestamped directory in the output folder.

Examples:
  # For local/unauthenticated Prometheus
  export url="http://localhost:9090"
  
  instrumentation-score analyze \
    --output-dir ./reports

  # For authenticated Prometheus
  export login="user:api_key"
  export url="https://your-prometheus-instance.com/api/prom"
  
  instrumentation-score analyze \
    --output-dir ./reports

  # With query filters
  instrumentation-score analyze \
    --output-dir ./reports \
    --additional-query-filters 'cluster=~"prod.*",environment="production"'

  # Multiple filters
  instrumentation-score analyze \
    --output-dir ./reports \
    --additional-query-filters 'cluster=~"prod-1-27-a1|prod-1-27-a1-eu-central-1",region="us-east-1"'`,
	Run: func(cmd *cobra.Command, args []string) {
		runAnalyze()
	},
}

func init() {
	analyzeCmd.Flags().StringVarP(&analyzeOutputDir, "output-dir", "o", ".", "Output directory for report files")
	analyzeCmd.Flags().StringVar(&analyzeQueryFilters, "additional-query-filters", "", "PromQL label filters (e.g., 'cluster=~\"prod.*\",environment=\"production\"')")
	analyzeCmd.Flags().IntVar(&analyzeRetryCount, "retry-failures-count", 2, "Number of retry attempts for failed requests due to transient network issues (e.g., connection refused, timeouts)")
	analyzeCmd.Flags().BoolVar(&analyzeS3Upload, "s3-upload", false, "Upload generated reports to S3")
	analyzeCmd.Flags().StringVar(&analyzeS3Bucket, "s3-bucket", "", "S3 bucket name (or use S3_BUCKET env var)")
	analyzeCmd.Flags().StringVar(&analyzeS3Prefix, "s3-prefix", "", "S3 key prefix (or use S3_PREFIX env var)")
	analyzeCmd.Flags().StringVar(&analyzeS3Region, "s3-region", "eu-west-1", "AWS region (or use AWS_REGION env var)")
}

func runAnalyze() {
	client, err := collectors.NewPrometheusClientFromEnv()
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		os.Exit(1)
	}

	if err := os.MkdirAll(analyzeOutputDir, 0700); err != nil {
		fmt.Printf("ERROR: Failed to create output directory: %v\n", err)
		os.Exit(1)
	}

	timestamp := time.Now().Format("20060102_150405")
	jobMetricsDir := filepath.Join(analyzeOutputDir, fmt.Sprintf("job_metrics_%s", timestamp))
	if err := os.MkdirAll(jobMetricsDir, 0700); err != nil {
		fmt.Printf("ERROR: Failed to create job metrics directory: %v\n", err)
		os.Exit(1)
	}

	errorFile := filepath.Join(analyzeOutputDir, fmt.Sprintf("metrics_errors_%s.txt", timestamp))

	fmt.Printf("Starting Prometheus metrics analysis...\n")
	fmt.Printf("Prometheus URL: %s\n", client.BaseURL)
	if analyzeQueryFilters != "" {
		fmt.Printf("Query filters: %s\n", analyzeQueryFilters)
	}
	fmt.Printf("Retry count: %d\n", analyzeRetryCount)
	fmt.Printf("Output directory: %s\n", jobMetricsDir)
	fmt.Println()

	collector := collectors.NewCollectorWithClient(client, analyzeQueryFilters)
	collector.SetRetryCount(analyzeRetryCount)
	allData, errors, err := collector.CollectMetrics()
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Writing per-job reports...")
	if err := collectors.WritePerJobFiles(jobMetricsDir, allData); err != nil {
		fmt.Printf("ERROR: Failed to write job files: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Generated per-job files in %s/\n\n", jobMetricsDir)

	if len(errors) > 0 {
		fmt.Printf("WARNING: Encountered %d errors during processing\n", len(errors))
		if err := collectors.WriteErrorsToFile(errorFile, errors); err != nil {
			fmt.Printf("WARNING: Failed to write error file: %v\n", err)
		} else {
			fmt.Printf("Error report saved to %s\n", errorFile)
		}
	} else {
		fmt.Println("No errors encountered!")
	}

	if analyzeS3Upload {
		fmt.Println("\nUploading reports to S3...")

		bucket := analyzeS3Bucket
		if bucket == "" {
			bucket = os.Getenv("S3_BUCKET")
		}

		prefix := analyzeS3Prefix
		if prefix == "" {
			prefix = os.Getenv("S3_PREFIX")
		}

		region := analyzeS3Region
		if region == "" {
			region = os.Getenv("AWS_REGION")
			if region == "" {
				region = "eu-west-1"
			}
		}

		config := storage.AnalysisUploadConfig{
			Bucket:        bucket,
			Prefix:        prefix,
			Region:        region,
			JobMetricsDir: jobMetricsDir,
			ErrorFile:     errorFile,
			Timestamp:     timestamp,
		}

		if err := storage.UploadAnalysisResults(config); err != nil {
			fmt.Printf("ERROR: Failed to upload to S3: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Println("\nAnalysis complete!")
}
