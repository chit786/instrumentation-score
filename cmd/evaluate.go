package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"instrumentation-score-service/internal/engine"
	"instrumentation-score-service/internal/formatters"
	"instrumentation-score-service/internal/loaders"
	"instrumentation-score-service/internal/storage"

	"github.com/spf13/cobra"
)

var (
	// Common flags
	rulesConfig    string
	outputFormats  string // Comma-separated: text,json,html,prometheus
	jsonFile       string
	htmlFile       string
	prometheusFile string

	// Single job flags
	jobFile string

	// All jobs flags
	jobDir       string
	minScore     float64
	showFailures bool
	showCosts    bool
	costPrice    float64

	// S3 flags
	evaluateS3Source bool
	evaluateS3Upload bool
	evaluateS3Bucket string
	evaluateS3Prefix string
	evaluateS3Region string
	evaluateS3RunID  string
)

// JobScoreResult represents the score result for a single job
type JobScoreResult struct {
	JobName          string              `json:"job_name"`
	TotalMetrics     int                 `json:"total_metrics"`
	TotalCardinality int64               `json:"total_cardinality"`
	EstimatedCost    float64             `json:"estimated_cost,omitempty"`
	Score            float64             `json:"instrumentation_score"`
	RuleResults      []engine.RuleResult `json:"rules"`
	FailedMetrics    []string            `json:"failed_metrics,omitempty"`
	MetricsBreakdown map[string]int      `json:"metrics_breakdown"`
}

// AllJobsReport represents the complete report for all jobs
type AllJobsReport struct {
	Timestamp        string           `json:"timestamp"`
	TotalJobs        int              `json:"total_jobs"`
	AverageScore     float64          `json:"average_score"`
	TotalCost        float64          `json:"total_cost,omitempty"`
	TotalCardinality int64            `json:"total_cardinality"`
	Jobs             []JobScoreResult `json:"jobs"`
}

var evaluateCmd = &cobra.Command{
	Use:   "evaluate",
	Short: "Evaluate job metrics against instrumentation score rules",
	Long: `Evaluate Prometheus metrics against instrumentation score rules.

Modes:
  Single Job: Specify --job-file to evaluate one job
  All Jobs:   Specify --job-dir to evaluate all jobs in a directory

Examples:
  # Evaluate single job with HTML output
  instrumentation-score-service evaluate \
    --job-file reports/job_metrics_*/api-service.txt \
    --output html --html-file report.html

  # Evaluate all jobs with multiple outputs
  instrumentation-score-service evaluate \
    --job-dir reports/job_metrics_20251102_160000/ \
    --output json,html \
    --json-file results.json \
    --html-file dashboard.html \
    --show-costs --cost-unit-price 0.00615

  # Text output to console (default)
  instrumentation-score-service evaluate \
    --job-file reports/job_metrics_*/api-service.txt`,
	Run: func(cmd *cobra.Command, args []string) {
		runEvaluate()
	},
}

func init() {
	// Common flags
	evaluateCmd.Flags().StringVarP(&rulesConfig, "rules", "r", "rules_config.yaml", "Rules configuration file")
	evaluateCmd.Flags().StringVarP(&outputFormats, "output", "o", "text", "Output formats (comma-separated): text,json,html,prometheus")
	evaluateCmd.Flags().StringVar(&jsonFile, "json-file", "", "JSON output file path")
	evaluateCmd.Flags().StringVar(&htmlFile, "html-file", "", "HTML output file path")
	evaluateCmd.Flags().StringVar(&prometheusFile, "prometheus-file", "", "Prometheus metrics output file path")

	// Single job mode
	evaluateCmd.Flags().StringVarP(&jobFile, "job-file", "j", "", "Evaluate single job file")

	// All jobs mode
	evaluateCmd.Flags().StringVarP(&jobDir, "job-dir", "d", "", "Evaluate all jobs in directory")
	evaluateCmd.Flags().Float64Var(&minScore, "min-score", 0.0, "Minimum score threshold (highlight jobs below this)")
	evaluateCmd.Flags().BoolVar(&showFailures, "show-failures", false, "Show detailed failure information")
	evaluateCmd.Flags().BoolVar(&showCosts, "show-costs", false, "Display estimated monthly costs")
	evaluateCmd.Flags().Float64Var(&costPrice, "cost-unit-price", 0.0, "Cost per active series per month (required with --show-costs)")

	// S3 mode
	evaluateCmd.Flags().BoolVar(&evaluateS3Source, "s3-source", false, "Download job metrics from S3")
	evaluateCmd.Flags().BoolVar(&evaluateS3Upload, "s3-upload", false, "Upload evaluation results to S3")
	evaluateCmd.Flags().StringVar(&evaluateS3Bucket, "s3-bucket", "", "S3 bucket name (or use S3_BUCKET env var)")
	evaluateCmd.Flags().StringVar(&evaluateS3Prefix, "s3-prefix", "", "S3 key prefix/path (or use S3_PREFIX env var)")
	evaluateCmd.Flags().StringVar(&evaluateS3Region, "s3-region", "eu-west-1", "AWS region (or use AWS_REGION env var)")
	evaluateCmd.Flags().StringVar(&evaluateS3RunID, "s3-run-id", "", "Run ID for S3 organization (default: auto-generated timestamp)")
}

func runEvaluate() {
	// Handle S3 source if specified
	if evaluateS3Source {
		bucket := evaluateS3Bucket
		if bucket == "" {
			bucket = os.Getenv("S3_BUCKET")
		}

		prefix := evaluateS3Prefix
		if prefix == "" {
			prefix = os.Getenv("S3_PREFIX")
		}

		region := evaluateS3Region
		if region == "" {
			region = os.Getenv("AWS_REGION")
			if region == "" {
				region = "eu-west-1"
			}
		}

		config := storage.EvaluationDownloadConfig{
			Bucket: bucket,
			Prefix: prefix,
			Region: region,
		}

		downloadedDir, err := storage.DownloadEvaluationSource(config)
		if err != nil {
			log.Fatalf("Error: Failed to download from S3: %v", err)
		}
		jobDir = downloadedDir
		fmt.Printf("Downloaded job metrics from S3 to: %s\n\n", jobDir)
	}

	// Determine mode
	if jobFile != "" && jobDir != "" {
		log.Fatal("Error: Cannot specify both --job-file and --job-dir. Choose one mode.")
	}

	if jobFile == "" && jobDir == "" {
		log.Fatal("Error: Must specify either --job-file (single job), --job-dir (all jobs), or --s3-source")
	}

	// Parse and validate output formats
	formats := parseOutputFormats(outputFormats)
	if len(formats) == 0 {
		log.Fatal("Error: At least one output format must be specified")
	}

	// Validate output file requirements
	for _, format := range formats {
		switch format {
		case "json":
			if jsonFile == "" && !contains(formats, "text") {
				log.Fatal("Error: --json-file is required when using --output json (or include 'text' for console output)")
			}
		case "html":
			if htmlFile == "" {
				log.Fatal("Error: --html-file is required when using --output html")
			}
		case "prometheus":
			if prometheusFile == "" && !contains(formats, "text") {
				log.Fatal("Error: --prometheus-file is required when using --output prometheus (or include 'text' for console output)")
			}
		case "text":
			// Text can always go to stdout
		default:
			log.Fatalf("Error: Unknown output format: %s. Valid formats: text, json, html, prometheus", format)
		}
	}

	// Validate cost flags
	if showCosts && costPrice <= 0 {
		log.Fatal("Error: --cost-unit-price must be specified and greater than 0 when --show-costs is enabled")
	}

	// Route to appropriate handler
	if jobFile != "" {
		runSingleJobEvaluation(formats)
	} else {
		runAllJobsEvaluation(formats)
	}
}

// parseOutputFormats parses comma-separated output formats
func parseOutputFormats(formats string) []string {
	if formats == "" {
		return []string{"text"}
	}

	parts := strings.Split(formats, ",")
	var result []string
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// runSingleJobEvaluation evaluates a single job
func runSingleJobEvaluation(formats []string) {
	// Load job metrics
	jobData, err := loaders.LoadJobMetricReport(jobFile)
	if err != nil {
		log.Fatalf("Error loading job metrics from %s: %v", jobFile, err)
	}

	if len(jobData) == 0 {
		log.Fatalf("No metrics found in %s", jobFile)
	}

	// Get job name from first entry
	jobName := jobData[0].Job

	// Initialize rule engine
	ruleEngine, err := engine.NewRuleEngine(rulesConfig)
	if err != nil {
		log.Fatalf("Error initializing rule engine: %v\n\nPlease ensure rules_config.yaml exists", err)
	}

	// Convert to evaluation format
	cardinalityData := loaders.ConvertJobMetricToCardinality(jobData)
	labelsData := loaders.ConvertJobMetricToLabels(jobData)

	// Evaluate
	results, err := ruleEngine.EvaluateWithData(cardinalityData, labelsData)
	if err != nil {
		log.Fatalf("Error evaluating rules: %v", err)
	}

	// Calculate score
	score := engine.CalculateInstrumentationScore(results)

	// Calculate cost if requested
	var totalCardinality int64
	var estimatedCost float64
	if showCosts && costPrice > 0 {
		for _, metric := range cardinalityData {
			totalCardinality += metric.Count
		}
		estimatedCost = float64(totalCardinality) * costPrice
	}

	// Generate outputs for each requested format
	for _, format := range formats {
		switch format {
		case "text":
			fmt.Printf("\n=== Instrumentation Score Report for Job: %s ===\n\n", jobName)
			fmt.Printf("Total Metrics: %d\n", len(jobData))
			if showCosts {
				fmt.Printf("Total Cardinality: %d series\n", totalCardinality)
				fmt.Printf("Estimated Cost: $%.2f/month\n", estimatedCost)
			}
			fmt.Printf("Instrumentation Score: %.2f%%\n\n", score)
			formatters.Text(jobName, score, results)

		case "json":
			result := JobScoreResult{
				JobName:          jobName,
				TotalMetrics:     len(jobData),
				TotalCardinality: totalCardinality,
				EstimatedCost:    estimatedCost,
				Score:            score,
				RuleResults:      results,
			}
			data, _ := json.MarshalIndent(result, "", "  ")

			if jsonFile != "" {
				if err := os.WriteFile(jsonFile, data, 0600); err != nil {
					log.Fatalf("Error writing JSON file: %v", err)
				}
				fmt.Printf("JSON report saved to %s\n", jsonFile)
			} else {
				fmt.Println(string(data))
			}

		case "html":
			formatters.HTML(jobName, score, results, htmlFile)
			fmt.Printf("HTML report saved to %s\n", htmlFile)

		case "prometheus":
			if prometheusFile != "" {
				// Write to file
				file, err := os.OpenFile(prometheusFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
				if err != nil {
					log.Fatalf("Error creating prometheus file: %v", err)
				}
				defer file.Close()

				// Redirect stdout temporarily
				oldStdout := os.Stdout
				os.Stdout = file
				formatters.PrometheusMetrics(jobName, score, results)
				os.Stdout = oldStdout

				fmt.Printf("Prometheus metrics saved to %s\n", prometheusFile)
			} else {
				formatters.PrometheusMetrics(jobName, score, results)
			}
		}
	}
}

// runAllJobsEvaluation evaluates all jobs in a directory
func runAllJobsEvaluation(formats []string) {
	// Find all job files
	files, err := filepath.Glob(filepath.Join(jobDir, "*.txt"))
	if err != nil {
		log.Fatalf("Error reading directory %s: %v", jobDir, err)
	}

	if len(files) == 0 {
		log.Fatalf("No job metric files found in %s", jobDir)
	}

	fmt.Printf("Found %d job files to evaluate...\n", len(files))

	// Initialize rule engine
	ruleEngine, err := engine.NewRuleEngine(rulesConfig)
	if err != nil {
		log.Fatalf("Error initializing rule engine: %v\n\nPlease ensure rules_config.yaml exists", err)
	}

	// Evaluate each job
	var allResults []JobScoreResult
	var totalScore float64
	var totalCost float64
	var totalCardinality int64
	var excludedCount int

	for i, file := range files {
		fmt.Printf("\rEvaluating jobs: %d/%d", i+1, len(files))

		result, err := evaluateSingleJobFile(file, ruleEngine)
		if err != nil {
			// Check if it's an exclusion error
			if strings.Contains(err.Error(), "is excluded from evaluation") || strings.Contains(err.Error(), "no metrics remaining after exclusion filtering") {
				excludedCount++
			} else {
				log.Printf("\nWarning: Failed to evaluate %s: %v", filepath.Base(file), err)
			}
			continue
		}

		allResults = append(allResults, result)
		totalScore += result.Score
		totalCost += result.EstimatedCost
		totalCardinality += result.TotalCardinality
	}

	fmt.Printf("\n\n")

	if excludedCount > 0 {
		fmt.Printf("ℹ️  Excluded %d job(s) based on exclusion_list in rules_config.yaml\n\n", excludedCount)
	}

	if len(allResults) == 0 {
		log.Fatal("No jobs were successfully evaluated")
	}

	// Calculate average score
	avgScore := totalScore / float64(len(allResults))

	// Create report
	report := AllJobsReport{
		Timestamp:        time.Now().Format(time.RFC3339),
		TotalJobs:        len(allResults),
		AverageScore:     avgScore,
		TotalCost:        totalCost,
		TotalCardinality: totalCardinality,
		Jobs:             allResults,
	}

	// Generate outputs for each requested format
	for _, format := range formats {
		switch format {
		case "text":
			printSummary(report)

		case "json":
			data, err := json.MarshalIndent(report, "", "  ")
			if err != nil {
				log.Fatalf("Error marshaling JSON: %v", err)
			}

			if jsonFile != "" {
				if err := os.WriteFile(jsonFile, data, 0600); err != nil {
					log.Fatalf("Error writing JSON file: %v", err)
				}
				fmt.Printf("JSON report saved to %s\n", jsonFile)
			} else {
				fmt.Println(string(data))
			}

		case "html":
			generateHTMLReport(report, files)

		case "prometheus":
			// Convert JobScoreResult to formatters.JobScoreData
			var jobsData []formatters.JobScoreData
			for _, job := range allResults {
				jobsData = append(jobsData, formatters.JobScoreData{
					JobName:          job.JobName,
					TotalMetrics:     job.TotalMetrics,
					TotalCardinality: job.TotalCardinality,
					EstimatedCost:    job.EstimatedCost,
					Score:            job.Score,
					RuleResults:      job.RuleResults,
				})
			}

			// Generate SLI metrics for Cortex.io SLO tracking
			promMetrics := formatters.PrometheusMetricsWithSLO(jobsData)

			if prometheusFile != "" {
				if err := os.WriteFile(prometheusFile, []byte(promMetrics), 0600); err != nil {
					log.Fatalf("Error writing Prometheus file: %v", err)
				}
				fmt.Printf("Prometheus metrics saved to %s\n", prometheusFile)
			} else {
				fmt.Print(promMetrics)
			}
		}
	}

	// Upload to S3 if requested
	if evaluateS3Upload {
		fmt.Println("\nUploading evaluation results to S3...")

		bucket := evaluateS3Bucket
		if bucket == "" {
			bucket = os.Getenv("S3_BUCKET")
		}

		prefix := evaluateS3Prefix
		if prefix == "" {
			prefix = os.Getenv("S3_PREFIX")
		}

		region := evaluateS3Region
		if region == "" {
			region = os.Getenv("AWS_REGION")
			if region == "" {
				region = "eu-west-1"
			}
		}

		// Create manifest
		manifest := &storage.EvaluationManifest{
			Timestamp:        report.Timestamp,
			TotalJobs:        report.TotalJobs,
			AverageScore:     report.AverageScore,
			TotalCardinality: report.TotalCardinality,
			TotalCost:        report.TotalCost,
			RulesConfig:      rulesConfig,
			OutputFormats:    strings.Join(formats, ","),
		}

		// Determine source type
		if evaluateS3Source {
			manifest.SourceType = "s3"
			manifest.SourcePath = fmt.Sprintf("s3://%s/%s", bucket, evaluateS3Prefix)
		} else if jobDir != "" {
			manifest.SourceType = "local_directory"
			manifest.SourcePath = jobDir
		} else if jobFile != "" {
			manifest.SourceType = "local_file"
			manifest.SourcePath = jobFile
		}

		config := storage.EvaluationUploadConfig{
			Bucket:         bucket,
			Prefix:         prefix,
			Region:         region,
			RunID:          evaluateS3RunID,
			JSONFile:       jsonFile,
			HTMLFile:       htmlFile,
			PrometheusFile: prometheusFile,
			OutputFormats:  formats,
			Manifest:       manifest,
		}

		if err := storage.UploadEvaluationResults(config); err != nil {
			log.Fatalf("Error: Failed to upload to S3: %v", err)
		}
	}
}

func evaluateSingleJobFile(filePath string, ruleEngine *engine.RuleEngine) (JobScoreResult, error) {
	// Load job metrics
	jobData, err := loaders.LoadJobMetricReport(filePath)
	if err != nil {
		return JobScoreResult{}, err
	}

	if len(jobData) == 0 {
		return JobScoreResult{}, fmt.Errorf("no metrics found")
	}

	jobName := jobData[0].Job

	// Check if job is completely excluded
	if ruleEngine.IsJobExcluded(jobName) {
		return JobScoreResult{}, fmt.Errorf("job %s is excluded from evaluation", jobName)
	}

	// Convert formats
	cardinalityData := loaders.ConvertJobMetricToCardinality(jobData)
	labelsData := loaders.ConvertJobMetricToLabels(jobData)

	// Filter out excluded metrics
	cardinalityData, labelsData = ruleEngine.FilterExcludedMetrics(jobName, cardinalityData, labelsData)

	// Check if any metrics remain after filtering
	if len(cardinalityData) == 0 && len(labelsData) == 0 {
		return JobScoreResult{}, fmt.Errorf("no metrics remaining after exclusion filtering for job %s", jobName)
	}

	// Calculate total cardinality
	var totalCardinality int64
	for _, metric := range cardinalityData {
		totalCardinality += metric.Count
	}

	// Calculate cost if enabled
	var estimatedCost float64
	if showCosts && costPrice > 0 {
		estimatedCost = float64(totalCardinality) * costPrice
	}

	// Evaluate
	results, err := ruleEngine.EvaluateWithData(cardinalityData, labelsData)
	if err != nil {
		return JobScoreResult{}, err
	}

	// Calculate score
	score := engine.CalculateInstrumentationScore(results)

	// Collect failed metrics
	var failedMetrics []string
	failedMetricsMap := make(map[string]bool)
	for _, result := range results {
		for metricName := range result.FailedMetrics {
			if !failedMetricsMap[metricName] {
				failedMetrics = append(failedMetrics, metricName)
				failedMetricsMap[metricName] = true
			}
		}
	}

	// Create breakdown
	breakdown := make(map[string]int)
	for _, result := range results {
		breakdown[result.RuleID] = result.PassedChecks
	}

	return JobScoreResult{
		JobName:          jobName,
		TotalMetrics:     len(jobData),
		TotalCardinality: totalCardinality,
		EstimatedCost:    estimatedCost,
		Score:            score,
		RuleResults:      results,
		FailedMetrics:    failedMetrics,
		MetricsBreakdown: breakdown,
	}, nil
}

func generateHTMLReport(report AllJobsReport, files []string) {
	// Prepare HTML data
	var jobsHTMLData []formatters.JobHTMLData

	// Create a map for quick lookup using actual job names from file content
	jobFileMap := make(map[string]string)
	for _, file := range files {
		jobData, err := loaders.LoadJobMetricReport(file)
		if err != nil || len(jobData) == 0 {
			continue
		}
		actualJobName := jobData[0].Job
		jobFileMap[actualJobName] = file
	}

	for _, jobResult := range report.Jobs {
		// Find the corresponding file
		jobFilePath := jobFileMap[jobResult.JobName]
		if jobFilePath == "" {
			continue
		}

		// Load job data for detailed metrics
		jobData, err := loaders.LoadJobMetricReport(jobFilePath)
		if err != nil {
			continue
		}

		// Convert to cardinality and labels data
		cardinalityData := loaders.ConvertJobMetricToCardinality(jobData)
		labelsDataList := loaders.ConvertJobMetricToLabels(jobData)

		// Create metric details
		var metrics []formatters.JobMetricDetail
		for _, metric := range jobData {
			// Find cardinality
			var cardinality string
			for _, cardData := range cardinalityData {
				if cardData.MetricName == metric.MetricName {
					cardinality = strconv.FormatInt(cardData.Count, 10)
					break
				}
			}

			// Find labels
			var labels string
			for _, labelData := range labelsDataList {
				if labelData.MetricName == metric.MetricName {
					labels = strings.Join(labelData.Labels, ", ")
					break
				}
			}

			// Check if metric failed
			failedValidators := jobResult.RuleResults
			var failures []string
			status := "pass"
			for _, result := range failedValidators {
				if validators, exists := result.FailedMetrics[metric.MetricName]; exists {
					failures = append(failures, validators...)
					status = "fail"
				}
			}

			metrics = append(metrics, formatters.JobMetricDetail{
				MetricName:  metric.MetricName,
				Cardinality: cardinality,
				Labels:      labels,
				Status:      status,
				FailedRules: failures,
			})
		}

		// Determine score category
		scoreInt := int(math.Round(jobResult.Score))
		var category, statusClass string
		if scoreInt >= 90 {
			category = "Excellent"
			statusClass = "excellent"
		} else if scoreInt >= 75 {
			category = "Good"
			statusClass = "good"
		} else if scoreInt >= 50 {
			category = "Needs Improvement"
			statusClass = "warning"
		} else {
			category = "Poor"
			statusClass = "poor"
		}

		jobsHTMLData = append(jobsHTMLData, formatters.JobHTMLData{
			JobName:          jobResult.JobName,
			Score:            jobResult.Score,
			ScoreInt:         scoreInt,
			Category:         category,
			StatusClass:      statusClass,
			Results:          jobResult.RuleResults,
			Metrics:          metrics,
			TotalMetrics:     jobResult.TotalMetrics,
			TotalCardinality: jobResult.TotalCardinality,
			EstimatedCost:    jobResult.EstimatedCost,
			ShowCost:         showCosts,
		})
	}

	// Sort by score (worst first)
	sort.Slice(jobsHTMLData, func(i, j int) bool {
		return jobsHTMLData[i].Score < jobsHTMLData[j].Score
	})

	// Generate HTML
	formatters.HTMLMultiJobWithCost(jobsHTMLData, report.AverageScore, report.TotalCost, report.TotalCardinality, showCosts, htmlFile, rulesConfig)
	fmt.Printf("✅ HTML report saved to %s\n", htmlFile)
}

func printSummary(report AllJobsReport) {
	fmt.Printf("\n=== Summary ===\n")
	fmt.Printf("Total Jobs: %d\n", report.TotalJobs)
	fmt.Printf("Average Score: %.2f%%\n", report.AverageScore)
	fmt.Printf("Total Active Series: %d\n", report.TotalCardinality)
	if showCosts {
		fmt.Printf("Total Cost: $%.2f/month\n", report.TotalCost)
	}

	// Count by category
	excellent, good, needsImprovement, poor := 0, 0, 0, 0
	for _, job := range report.Jobs {
		switch {
		case job.Score >= 90:
			excellent++
		case job.Score >= 75:
			good++
		case job.Score >= 50:
			needsImprovement++
		default:
			poor++
		}
	}

	fmt.Printf("\nScore Distribution:\n")
	fmt.Printf("  Excellent (90-100): %d jobs\n", excellent)
	fmt.Printf("  Good (75-89): %d jobs\n", good)
	fmt.Printf("  Needs Improvement (50-74): %d jobs\n", needsImprovement)
	fmt.Printf("  Poor (0-49): %d jobs\n", poor)

	if minScore > 0 {
		fmt.Printf("\nJobs Below Threshold (%.2f%%):\n", minScore)
		count := 0
		for _, job := range report.Jobs {
			if job.Score < minScore {
				count++
				fmt.Printf("  - %s: %.2f%%\n", job.JobName, job.Score)
			}
		}
		if count == 0 {
			fmt.Printf("  (none)\n")
		}
	}
}
