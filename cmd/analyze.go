package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"instrumentation-score/internal/collectors"
	"instrumentation-score/internal/storage"

	"github.com/spf13/cobra"
)

var (
	analyzeOutputDir                   string
	analyzeQueryFilters                string
	analyzeRetryCount                  int
	analyzeS3Upload                    bool
	analyzeS3Bucket                    string
	analyzeS3Prefix                    string
	analyzeS3Region                    string
	analyzeCollectLabelCardinality     bool
	analyzeLabelCardinalityConcurrency int
	analyzeMetricsConcurrency          int
	analyzeJobsConcurrency             int

	// Batch query flags
	analyzeBatchMode          string
	analyzeBatchCharsPerGroup int
	analyzeBatchPatterns      []string
	analyzeBatchMaxResults    int
	analyzeBatchConcurrency   int
	analyzeBatchDebugMode     bool
	analyzeBatchIntervalMs    int
	analyzeBatchRPSLimit      int // Max metrics per batch for RPS control
	analyzeStreamingMode      bool
	analyzeMaxOpenFiles       int
)

var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "Analyze Prometheus metrics and generate per-job reports",
	Long: `Analyze Prometheus metrics and generate comprehensive per-job reports.

This command fetches metrics from Prometheus, analyzes them by job, and generates:
- Per-job metric files with format: JOB|METRIC_NAME|LABELS|CARDINALITY
- Error report in <output-dir>/errors/errors.txt

The reports are written directly to the specified output directory.

Examples:
  # For authenticated Prometheus (e.g., Grafana Cloud)
  export login="user:password"
  export url="https://your-prometheus-instance.com/api/prom"
  
  instrumentation-score analyze \
    --output-dir ./reports

  # For local/unauthenticated Prometheus
  export url="http://localhost:9090"
  
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
	analyzeCmd.Flags().BoolVar(&analyzeCollectLabelCardinality, "collect-label-cardinality", false, "Collect per-label cardinality data using Mimir cardinality API (more accurate but slower)")
	analyzeCmd.Flags().IntVar(&analyzeLabelCardinalityConcurrency, "label-cardinality-concurrency", 0, "Number of concurrent label cardinality API requests (default: 50, or CONCURRENT_LABEL_CARDINALITY env var)")
	analyzeCmd.Flags().IntVar(&analyzeMetricsConcurrency, "metrics-concurrency", 0, "Number of concurrent metrics to process (default: 5, or CONCURRENT_METRICS env var)")
	analyzeCmd.Flags().IntVar(&analyzeJobsConcurrency, "jobs-concurrency", 0, "Number of concurrent job queries per metric (default: 3, or CONCURRENT_JOBS env var)")

	// Batch query flags
	analyzeCmd.Flags().StringVar(&analyzeBatchMode, "batch-mode", "legacy", "Query mode: legacy, alphabetic, custom, adaptive (default: legacy for backward compatibility)")
	analyzeCmd.Flags().IntVar(&analyzeBatchCharsPerGroup, "batch-chars-per-group", 3, "Characters per batch group for alphabetic mode (e.g., 3 for [a-c], [d-f], etc.)")
	analyzeCmd.Flags().StringSliceVar(&analyzeBatchPatterns, "batch-patterns", []string{}, "Custom regex patterns for batch mode (e.g., 'http_.*,grpc_.*,aws_.*')")
	analyzeCmd.Flags().IntVar(&analyzeBatchMaxResults, "batch-max-results", 10000, "Target max results per batch for adaptive mode")
	analyzeCmd.Flags().IntVar(&analyzeBatchConcurrency, "batch-concurrency", 0, "Number of concurrent batch queries (default: 2, or CONCURRENT_BATCHES env var). Lower values reduce head spike.")
	analyzeCmd.Flags().BoolVar(&analyzeBatchDebugMode, "batch-debug-mode", false, "Print batch details without executing queries (useful for optimization)")
	analyzeCmd.Flags().IntVar(&analyzeBatchIntervalMs, "batch-interval-ms", 0, "Minimum milliseconds between batch starts for uniform load (0 = no rate limiting). Example: 500 = max 2 batches/sec")
	analyzeCmd.Flags().IntVar(&analyzeBatchRPSLimit, "batch-rps-limit", 0, "Max metrics per batch to control query size/RPS. Overrides batch-max-results. Example: 1000 = each query returns max ~1000 metrics")
	analyzeCmd.Flags().BoolVar(&analyzeStreamingMode, "streaming-mode", true, "Enable streaming writes to disk (reduces memory usage)")
	analyzeCmd.Flags().IntVar(&analyzeMaxOpenFiles, "max-open-files", 100, "Maximum concurrent open file handles")
}

func runAnalyze() {
	client, err := collectors.NewPrometheusClientFromEnv()
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		os.Exit(1)
	}

	// Use the output directory directly (no timestamped subdirectory)
	jobMetricsDir := analyzeOutputDir
	if err := os.MkdirAll(jobMetricsDir, 0700); err != nil {
		fmt.Printf("ERROR: Failed to create output directory: %v\n", err)
		os.Exit(1)
	}

	// Create errors subdirectory with static filename
	errorsDir := filepath.Join(jobMetricsDir, "errors")
	if err := os.MkdirAll(errorsDir, 0700); err != nil {
		fmt.Printf("ERROR: Failed to create errors directory: %v\n", err)
		os.Exit(1)
	}
	errorFile := filepath.Join(errorsDir, "errors.txt")

	fmt.Printf("Starting Prometheus metrics analysis...\n")
	fmt.Printf("Prometheus URL: %s\n", client.BaseURL)
	if analyzeQueryFilters != "" {
		fmt.Printf("Query filters: %s\n", analyzeQueryFilters)
	}
	fmt.Printf("Batch mode: %s\n", analyzeBatchMode)
	fmt.Printf("Streaming mode: %v\n", analyzeStreamingMode)
	fmt.Printf("Retry count: %d\n", analyzeRetryCount)
	fmt.Printf("Collect label cardinality: %v\n", analyzeCollectLabelCardinality)
	fmt.Printf("Output directory: %s\n", jobMetricsDir)
	fmt.Println()

	collector := collectors.NewCollectorWithClient(client, analyzeQueryFilters)
	collector.SetRetryCount(analyzeRetryCount)
	collector.SetCollectLabelCardinality(analyzeCollectLabelCardinality)

	// Override concurrency settings if flags are provided (flags take precedence over env vars)
	if analyzeLabelCardinalityConcurrency > 0 {
		collector.SetLabelCardinalityConcurrency(analyzeLabelCardinalityConcurrency)
	}
	if analyzeMetricsConcurrency > 0 {
		collector.SetMetricsConcurrency(analyzeMetricsConcurrency)
	}
	if analyzeJobsConcurrency > 0 {
		collector.SetJobsConcurrency(analyzeJobsConcurrency)
	}
	if analyzeBatchConcurrency > 0 {
		collector.SetBatchConcurrency(analyzeBatchConcurrency)
	}

	// Configure batch debug mode
	collector.SetBatchDebugMode(analyzeBatchDebugMode)

	// Configure rate limiting for uniform load distribution
	if analyzeBatchIntervalMs > 0 {
		collector.SetBatchIntervalMs(analyzeBatchIntervalMs)
	}

	// Configure batch mode
	// Auto-switch to adaptive mode when RPS limit is set (most optimized)
	effectiveBatchMode := analyzeBatchMode
	if analyzeBatchRPSLimit > 0 && analyzeBatchMode == "legacy" {
		effectiveBatchMode = "adaptive"
		fmt.Printf("Auto-switching to adaptive batch mode (RPS limit set)\n")
	}

	if effectiveBatchMode != "legacy" {
		collector.SetBatchMode(true)

		var strategy collectors.BatchStrategy
		switch effectiveBatchMode {
		case "alphabetic":
			strategy = &collectors.AlphabeticBatchStrategy{
				CharsPerBatch: analyzeBatchCharsPerGroup,
			}
		case "custom":
			if len(analyzeBatchPatterns) == 0 {
				fmt.Printf("ERROR: --batch-patterns required for custom batch mode\n")
				os.Exit(1)
			}
			strategy = &collectors.CustomBatchStrategy{
				Patterns: analyzeBatchPatterns,
			}
		case "adaptive":
			// Use RPS limit if specified, otherwise fall back to max results
			targetSize := analyzeBatchMaxResults
			if analyzeBatchRPSLimit > 0 {
				targetSize = analyzeBatchRPSLimit
				fmt.Printf("RPS limit: %d metrics per batch\n", targetSize)
			}
			strategy = &collectors.AdaptiveBatchStrategy{
				TargetResultsPerBatch: targetSize,
			}
		default:
			fmt.Printf("ERROR: Invalid batch mode: %s (must be: legacy, alphabetic, custom, or adaptive)\n", effectiveBatchMode)
			os.Exit(1)
		}

		collector.SetBatchStrategy(strategy)
	}

	// Configure streaming mode
	collector.SetStreamingMode(analyzeStreamingMode)
	collector.SetMaxOpenFiles(analyzeMaxOpenFiles)

	// Collect metrics
	var errors []collectors.ErrorRecord
	if analyzeStreamingMode {
		// Use streaming mode (memory efficient)
		errors, err = collector.CollectMetricsStreaming(jobMetricsDir)
		if err != nil {
			fmt.Printf("ERROR: %v\n", err)
			os.Exit(1)
		}
	} else {
		// Use legacy in-memory mode (backward compatible)
		var allData []collectors.JobMetricData
		allData, errors, err = collector.CollectMetrics()
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
	}

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
			RunID:         filepath.Base(jobMetricsDir), // Use directory name as S3 subdirectory
		}

		if err := storage.UploadAnalysisResults(config); err != nil {
			fmt.Printf("ERROR: Failed to upload to S3: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Println("\nAnalysis complete!")
}
