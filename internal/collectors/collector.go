package collectors

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// JobMetricData represents metric data for a specific job
type JobMetricData struct {
	Job              string
	MetricName       string
	Labels           []string
	Cardinality      string
	LabelCardinality map[string]int64 // Per-label cardinality (label_name -> cardinality)
}

// ErrorRecord represents an error that occurred during collection
type ErrorRecord struct {
	MetricName string
	Operation  string
	Error      string
	Timestamp  time.Time
}

// Collector orchestrates the collection of metrics from Prometheus
type Collector struct {
	client                        *PrometheusClient
	queryFilters                  string
	maxConcurrentMetrics          int // Concurrent metric processing
	maxConcurrentJobs             int // Concurrent job queries per metric
	maxConcurrentLabelCardinality int // Concurrent label cardinality API calls
	maxConcurrentBatches          int // Concurrent batch query execution
	collectLabelCardinality       bool
	batchDebugMode                bool // Print batches without executing

	// Batch query configuration
	enableBatching bool
	batchStrategy  BatchStrategy
	streamingMode  bool
	maxOpenFiles   int

	// Rate limiting for uniform load
	batchIntervalMs int // Milliseconds between batch starts (0 = no rate limiting)
}

// NewCollector creates a new metrics collector
func NewCollector(baseURL, login, queryFilters string) *Collector {
	return &Collector{
		client:                        NewPrometheusClient(baseURL, login),
		queryFilters:                  queryFilters,
		maxConcurrentMetrics:          getEnvInt("CONCURRENT_METRICS", 5),
		maxConcurrentJobs:             getEnvInt("CONCURRENT_JOBS", 3),
		maxConcurrentLabelCardinality: getEnvInt("CONCURRENT_LABEL_CARDINALITY", 50),
		maxConcurrentBatches:          getEnvInt("CONCURRENT_BATCHES", 2), // Conservative default for smooth load
		enableBatching:                false,                              // Default to legacy mode for backward compatibility
		streamingMode:                 true,                               // Default to streaming for memory efficiency
		maxOpenFiles:                  100,                                // Default max open files
		batchDebugMode:                false,
	}
}

// NewCollectorWithClient creates a new metrics collector with an existing Prometheus client
func NewCollectorWithClient(client *PrometheusClient, queryFilters string) *Collector {
	return &Collector{
		client:                        client,
		queryFilters:                  queryFilters,
		maxConcurrentMetrics:          getEnvInt("CONCURRENT_METRICS", 5),
		maxConcurrentJobs:             getEnvInt("CONCURRENT_JOBS", 3),
		maxConcurrentLabelCardinality: getEnvInt("CONCURRENT_LABEL_CARDINALITY", 50),
		maxConcurrentBatches:          getEnvInt("CONCURRENT_BATCHES", 2), // Conservative default for smooth load
		enableBatching:                false,                              // Default to legacy mode for backward compatibility
		streamingMode:                 true,                               // Default to streaming for memory efficiency
		maxOpenFiles:                  100,                                // Default max open files
		batchDebugMode:                false,
	}
}

// getEnvInt gets an integer from environment variable or returns default
func getEnvInt(key string, defaultValue int) int {
	if val := os.Getenv(key); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil && intVal > 0 {
			return intVal
		}
	}
	return defaultValue
}

// SetRetryCount sets the number of retry attempts for failed requests
func (c *Collector) SetRetryCount(count int) {
	c.client.SetRetryCount(count)
}

// SetCollectLabelCardinality enables/disables per-label cardinality collection
func (c *Collector) SetCollectLabelCardinality(enabled bool) {
	c.collectLabelCardinality = enabled
}

// SetLabelCardinalityConcurrency sets the number of concurrent label cardinality API requests
func (c *Collector) SetLabelCardinalityConcurrency(concurrency int) {
	if concurrency > 0 {
		c.maxConcurrentLabelCardinality = concurrency
	}
}

// SetMetricsConcurrency sets the number of concurrent metrics to process
func (c *Collector) SetMetricsConcurrency(concurrency int) {
	if concurrency > 0 {
		c.maxConcurrentMetrics = concurrency
	}
}

// SetJobsConcurrency sets the number of concurrent job queries per metric
func (c *Collector) SetJobsConcurrency(concurrency int) {
	if concurrency > 0 {
		c.maxConcurrentJobs = concurrency
	}
}

// SetBatchConcurrency sets the number of concurrent batch queries
func (c *Collector) SetBatchConcurrency(concurrency int) {
	if concurrency > 0 {
		c.maxConcurrentBatches = concurrency
	}
}

// SetBatchDebugMode enables or disables batch debug mode
func (c *Collector) SetBatchDebugMode(enabled bool) {
	c.batchDebugMode = enabled
}

// SetBatchIntervalMs sets the minimum interval between batch starts (in milliseconds)
// This enables rate limiting for uniform load distribution.
// Example: 500ms interval = max 2 batches per second start rate
// Set to 0 to disable rate limiting (default)
func (c *Collector) SetBatchIntervalMs(intervalMs int) {
	if intervalMs >= 0 {
		c.batchIntervalMs = intervalMs
	}
}

// SetBatchMode enables or disables batch query mode
func (c *Collector) SetBatchMode(enabled bool) {
	c.enableBatching = enabled
}

// SetBatchStrategy sets the batch strategy to use
func (c *Collector) SetBatchStrategy(strategy BatchStrategy) {
	c.batchStrategy = strategy
}

// SetStreamingMode enables or disables streaming writes
func (c *Collector) SetStreamingMode(enabled bool) {
	c.streamingMode = enabled
}

// SetMaxOpenFiles sets the maximum number of concurrent open file handles
func (c *Collector) SetMaxOpenFiles(max int) {
	if max > 0 {
		c.maxOpenFiles = max
	}
}

// CollectMetrics collects all metrics from Prometheus and returns job-specific data
func (c *Collector) CollectMetrics() ([]JobMetricData, []ErrorRecord, error) {
	now := time.Now().Unix()
	var errors []ErrorRecord
	var errorsMu sync.Mutex

	fmt.Println("Fetching metric names...")
	metricNames, err := c.client.GetAllMetricNames(c.queryFilters)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch metric names: %w", err)
	}
	fmt.Printf("Found %d metrics\n\n", len(metricNames))

	if c.queryFilters != "" {
		fmt.Printf("Using query filters: %s\n", c.queryFilters)
	}

	fmt.Println("Analyzing metrics by job (this may take a while)...")
	allData := c.fetchJobMetricData(metricNames, now, &errors, &errorsMu)
	fmt.Printf("\nAnalysis complete! Processed %d metric-job combinations\n\n", len(allData))

	return allData, errors, nil
}

func (c *Collector) fetchJobMetricData(metricNames []string, now int64, errors *[]ErrorRecord, errorsMu *sync.Mutex) []JobMetricData {
	var allData []JobMetricData
	var dataMu sync.Mutex
	var wg sync.WaitGroup
	var processed int32

	sem := make(chan struct{}, c.maxConcurrentMetrics)
	total := len(metricNames)

	for _, metricName := range metricNames {
		wg.Add(1)
		sem <- struct{}{}

		go func(metric string) {
			defer wg.Done()
			defer func() { <-sem }()

			jobData, err := c.getJobMetricDataForMetric(metric, now)
			if err != nil {
				errorsMu.Lock()
				*errors = append(*errors, ErrorRecord{
					MetricName: metric,
					Operation:  "fetch_job_data",
					Error:      err.Error(),
					Timestamp:  time.Now(),
				})
				errorsMu.Unlock()
			} else if len(jobData) > 0 {
				dataMu.Lock()
				allData = append(allData, jobData...)
				dataMu.Unlock()
			}

			current := atomic.AddInt32(&processed, 1)
			if current%50 == 0 || current == int32(total) {
				log.Printf("Processing metrics: %d/%d (%.1f%%)", current, total, float64(current)/float64(total)*100)
			}
		}(metricName)
	}

	wg.Wait()
	return allData
}

func (c *Collector) getJobMetricDataForMetric(metricName string, now int64) ([]JobMetricData, error) {
	jobNames, err := c.client.GetJobsForMetric(metricName, c.queryFilters, now)
	if err != nil {
		return nil, err
	}

	if len(jobNames) == 0 {
		return nil, nil
	}

	// Phase 1: Collect basic metric data (cardinality + labels) with limited concurrency
	type basicMetricData struct {
		job         string
		cardinality string
		labels      []string
	}

	var basicData []basicMetricData
	var wg sync.WaitGroup
	var mu sync.Mutex
	sem := make(chan struct{}, c.maxConcurrentJobs) // Concurrent job queries per metric

	for _, jobName := range jobNames {
		wg.Add(1)
		sem <- struct{}{}
		go func(job string) {
			defer wg.Done()
			defer func() { <-sem }()

			cardinality, err := c.client.GetCardinality(metricName, job, c.queryFilters, now)
			if err != nil {
				return
			}

			labels, err := c.client.GetLabels(metricName, job, c.queryFilters)
			if err != nil {
				return
			}

			mu.Lock()
			basicData = append(basicData, basicMetricData{
				job:         job,
				cardinality: cardinality,
				labels:      labels,
			})
			mu.Unlock()
		}(jobName)
	}
	wg.Wait()

	// Phase 2: Collect label cardinality with higher concurrency (if enabled)
	var results []JobMetricData
	if c.collectLabelCardinality {
		var wg2 sync.WaitGroup
		var mu2 sync.Mutex
		// Use separate semaphore with higher concurrency for label cardinality API
		labelCardSem := make(chan struct{}, c.maxConcurrentLabelCardinality)

		for _, data := range basicData {
			wg2.Add(1)
			labelCardSem <- struct{}{}
			go func(d basicMetricData) {
				defer wg2.Done()
				defer func() { <-labelCardSem }()

				var labelCardinality map[string]int64
				if len(d.labels) > 0 {
					var err error
					labelCardinality, err = c.client.GetLabelCardinality(metricName, d.job, d.labels, c.queryFilters)
					if err != nil {
						// Log error but don't fail - fall back to no per-label data
						fmt.Printf("WARNING: Failed to get label cardinality for %s/%s: %v\n", metricName, d.job, err)
						labelCardinality = nil
					}
				}

				mu2.Lock()
				results = append(results, JobMetricData{
					Job:              d.job,
					MetricName:       metricName,
					Labels:           d.labels,
					Cardinality:      d.cardinality,
					LabelCardinality: labelCardinality,
				})
				mu2.Unlock()
			}(data)
		}
		wg2.Wait()
	} else {
		// No label cardinality collection - just convert basic data to results
		for _, data := range basicData {
			results = append(results, JobMetricData{
				Job:              data.job,
				MetricName:       metricName,
				Labels:           data.labels,
				Cardinality:      data.cardinality,
				LabelCardinality: nil,
			})
		}
	}

	return results, nil
}

// sanitizeJobName replaces filesystem-unsafe characters in job names
func sanitizeJobName(jobName string) string {
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
	)
	return replacer.Replace(jobName)
}

// WritePerJobFiles writes collected data to per-job files
func WritePerJobFiles(outputDir string, allData []JobMetricData) error {
	jobFiles := make(map[string]*os.File)
	jobWriters := make(map[string]*bufio.Writer)
	skippedJobs := make(map[string]bool)
	var writeErrors []string

	defer func() {
		for _, writer := range jobWriters {
			writer.Flush()
		}
		for _, file := range jobFiles {
			file.Close()
		}
	}()

	for _, data := range allData {
		if skippedJobs[data.Job] {
			continue
		}

		if _, exists := jobFiles[data.Job]; !exists {
			safeJobName := sanitizeJobName(data.Job)
			filePath := filepath.Join(outputDir, fmt.Sprintf("%s.txt", safeJobName))
			file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
			if err != nil {
				errMsg := fmt.Sprintf("failed to create file for job %s (sanitized: %s): %v", data.Job, safeJobName, err)
				writeErrors = append(writeErrors, errMsg)
				skippedJobs[data.Job] = true
				fmt.Printf("WARNING: %s\n", errMsg)
				continue
			}
			jobFiles[data.Job] = file
			writer := bufio.NewWriter(file)
			jobWriters[data.Job] = writer
			if _, err := writer.WriteString("JOB|METRIC_NAME|LABELS|CARDINALITY|LABEL_CARDINALITY\n"); err != nil {
				return fmt.Errorf("failed to write header: %w", err)
			}
		}

		writer := jobWriters[data.Job]
		labelsStr := strings.Join(data.Labels, ",")

		// Format per-label cardinality as label1:count1,label2:count2,...
		var labelCardinalityStr string
		if len(data.LabelCardinality) > 0 {
			var parts []string
			for _, label := range data.Labels {
				if count, ok := data.LabelCardinality[label]; ok {
					parts = append(parts, fmt.Sprintf("%s:%d", label, count))
				}
			}
			labelCardinalityStr = strings.Join(parts, ",")
		}

		line := fmt.Sprintf("%s|%s|%s|%s|%s\n", data.Job, data.MetricName, labelsStr, data.Cardinality, labelCardinalityStr)
		if _, err := writer.WriteString(line); err != nil {
			return fmt.Errorf("failed to write metric data: %w", err)
		}
	}

	if len(writeErrors) > 0 {
		fmt.Printf("\nWARNING: Skipped %d job(s) due to file creation errors\n", len(skippedJobs))
	}

	return nil
}

// WriteErrorsToFile writes error records to a file
func WriteErrorsToFile(filename string, errors []ErrorRecord) error {
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create error file: %w", err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	if _, err := writer.WriteString("TIMESTAMP|METRIC_NAME|OPERATION|ERROR\n"); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}
	for _, e := range errors {
		line := fmt.Sprintf("%s|%s|%s|%s\n",
			e.Timestamp.Format("2006-01-02 15:04:05"),
			e.MetricName,
			e.Operation,
			e.Error)
		if _, err := writer.WriteString(line); err != nil {
			return fmt.Errorf("failed to write error line: %w", err)
		}
	}

	return nil
}

// CollectMetricsStreaming collects metrics with streaming writes to disk
// This method reduces memory usage by writing data immediately instead of accumulating in memory
func (c *Collector) CollectMetricsStreaming(outputDir string) ([]ErrorRecord, error) {
	now := time.Now().Unix()
	var errors []ErrorRecord
	var errorsMu sync.Mutex

	fmt.Println("Fetching metric names...")
	metricNames, err := c.client.GetAllMetricNames(c.queryFilters)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch metric names: %w", err)
	}
	fmt.Printf("Found %d metrics\n\n", len(metricNames))

	if c.queryFilters != "" {
		fmt.Printf("Using query filters: %s\n", c.queryFilters)
	}

	// Create streaming writer
	writer, err := NewStreamingJobFileWriter(outputDir, c.maxOpenFiles)
	if err != nil {
		return nil, fmt.Errorf("failed to create streaming writer: %w", err)
	}
	defer writer.Close()

	fmt.Println("Analyzing metrics by job (streaming to disk)...")

	if c.enableBatching {
		err = c.fetchAndStreamBatch(metricNames, now, writer, &errors, &errorsMu)
	} else {
		err = c.fetchAndStreamLegacy(metricNames, now, writer, &errors, &errorsMu)
	}

	if err != nil {
		return errors, err
	}

	// Final flush
	if err := writer.Flush(); err != nil {
		return errors, fmt.Errorf("failed to flush writer: %w", err)
	}

	stats := writer.GetStats()
	fmt.Printf("\nAnalysis complete! Processed %d metric-job combinations\n", stats.TotalWrites)
	fmt.Printf("Total files: %d, Bytes written: %d\n\n", stats.TotalFiles, stats.BytesWritten)

	return errors, nil
}

// fetchAndStreamBatch processes metrics using batch queries with streaming writes
// Automatically falls back to legacy mode for any metrics not covered by batches
func (c *Collector) fetchAndStreamBatch(
	metricNames []string,
	now int64,
	writer MetricDataWriter,
	errors *[]ErrorRecord,
	errorsMu *sync.Mutex,
) error {
	if c.batchStrategy == nil {
		// Default to alphabetic strategy
		c.batchStrategy = &AlphabeticBatchStrategy{CharsPerBatch: 3}
	}

	// Create batches
	batches := c.batchStrategy.CreateBatches(metricNames)
	fmt.Printf("Created %d batches for %d metrics\n", len(batches), len(metricNames))

	// Debug mode: Print batches and exit
	if c.batchDebugMode {
		c.printBatchDebugInfo(batches, metricNames)
		return nil
	}

	// Log batch execution plan
	log.Printf("Executing %d batches with concurrency=%d", len(batches), c.maxConcurrentBatches)
	for i, batch := range batches {
		sampleMetrics := batch.Metrics
		if len(sampleMetrics) > 3 {
			sampleMetrics = sampleMetrics[:3]
		}
		if batch.ExplicitMetrics && batch.BatchID != "" {
			log.Printf("  Batch %d: %s (explicit), Metrics=%d, Sample=%v", i+1, batch.BatchID, len(batch.Metrics), sampleMetrics)
		} else {
			log.Printf("  Batch %d: Pattern=%s, Metrics=%d, Sample=%v", i+1, batch.Pattern, len(batch.Metrics), sampleMetrics)
		}
	}

	var processed int32
	total := len(batches)

	// Track which metrics were processed by batch queries
	// This ensures we don't miss any metrics due to batching issues
	var processedMetrics sync.Map

	// Track processed metric-job pairs to avoid duplicates
	// Use sync.Map for better concurrent performance
	var processedPairs sync.Map

	// Cache for query results to avoid repeating identical queries
	// Key: pattern, Value: []MetricJobCount
	// Used when multiple batches share the same pattern (ExplicitMetrics mode)
	var queryCache sync.Map

	// Process batches with controlled concurrency and optional rate limiting
	var wg sync.WaitGroup
	sem := make(chan struct{}, c.maxConcurrentBatches)

	// Rate limiter for uniform load distribution
	var rateLimiter <-chan time.Time
	if c.batchIntervalMs > 0 {
		ticker := time.NewTicker(time.Duration(c.batchIntervalMs) * time.Millisecond)
		defer ticker.Stop()
		rateLimiter = ticker.C
		log.Printf("Rate limiting enabled: %dms between batch starts (%.1f batches/sec max)",
			c.batchIntervalMs, 1000.0/float64(c.batchIntervalMs))
	}

	for batchIdx, batch := range batches {
		// Wait for rate limiter if enabled (ensures uniform batch start rate)
		if rateLimiter != nil {
			<-rateLimiter
		}

		wg.Add(1)
		sem <- struct{}{}

		go func(idx int, b MetricBatch) {
			defer wg.Done()
			defer func() { <-sem }()

			// Build explicit metrics set if this batch uses explicit filtering
			// This is used when metrics share very long prefixes and can't be split by regex
			var explicitSet map[string]struct{}
			if b.ExplicitMetrics {
				explicitSet = make(map[string]struct{}, len(b.Metrics))
				for _, m := range b.Metrics {
					explicitSet[m] = struct{}{}
				}
			}

			// Determine logging name (use BatchID if available, otherwise Pattern)
			batchName := b.Pattern
			if b.BatchID != "" {
				batchName = b.BatchID
			}

			// Get query results (from cache or execute fresh query)
			var metricJobCounts []MetricJobCount
			var err error

			if b.ExplicitMetrics {
				// For explicit metrics batches, use cached results if available
				if cached, ok := queryCache.Load(b.Pattern); ok {
					metricJobCounts = cached.([]MetricJobCount)
				} else {
					// Execute query and cache results
					metricJobCounts, err = c.client.executeBatchWithAutoSplit(b.Pattern, c.queryFilters, now, 0, metricNames)
					if err == nil {
						queryCache.Store(b.Pattern, metricJobCounts)
					}
				}
			} else {
				// Normal batch - execute query directly
				metricJobCounts, err = c.client.executeBatchWithAutoSplit(b.Pattern, c.queryFilters, now, 0, metricNames)
			}

			if err != nil {
				errorsMu.Lock()
				*errors = append(*errors, ErrorRecord{
					MetricName: batchName,
					Operation:  "batch_query",
					Error:      err.Error(),
					Timestamp:  time.Now(),
				})
				errorsMu.Unlock()
				return
			}

			// Deduplicate across batches and track processed metrics
			filtered := make([]MetricJobCount, 0, len(metricJobCounts))
			for _, mjc := range metricJobCounts {
				// If explicit filtering is enabled, skip metrics not in this batch's list
				if b.ExplicitMetrics {
					if _, inBatch := explicitSet[mjc.MetricName]; !inBatch {
						continue
					}
				}

				// Track that this metric was processed
				processedMetrics.Store(mjc.MetricName, struct{}{})

				// Create unique key for metric-job pair (avoid fmt.Sprintf allocation)
				// Use simple string concatenation which is optimized by compiler
				pairKey := mjc.MetricName + "|" + mjc.JobName

				// Check if already processed using LoadOrStore (atomic operation)
				// Returns (value, loaded) - loaded=true means key existed
				if _, alreadyProcessed := processedPairs.LoadOrStore(pairKey, struct{}{}); !alreadyProcessed {
					// First time seeing this pair - add to filtered list
					filtered = append(filtered, mjc)
				}
			}

			// Process each metric-job combination
			c.processMetricJobCounts(filtered, writer, errors, errorsMu)

			current := atomic.AddInt32(&processed, 1)
			if current%10 == 0 || current == int32(total) {
				log.Printf("Processing batches: %d/%d (%.1f%%)", current, total, float64(current)/float64(total)*100)
			}
		}(batchIdx, batch)
	}

	wg.Wait()

	// Safety net: Find metrics that weren't processed by any batch
	// This handles edge cases like metrics starting with special characters
	var missedMetrics []string
	for _, metricName := range metricNames {
		if _, wasProcessed := processedMetrics.Load(metricName); !wasProcessed {
			missedMetrics = append(missedMetrics, metricName)
		}
	}

	// Smart fallback for missed metrics using micro-batching
	if len(missedMetrics) > 0 {
		log.Printf("SAFETY NET: %d metrics were not processed by batch queries, using smart fallback", len(missedMetrics))
		log.Printf("SAFETY NET: Missed metrics sample: %v", missedMetrics[:min(5, len(missedMetrics))])

		// Use micro-batching for better performance than legacy mode
		err := c.fetchMissedMetricsWithMicroBatching(missedMetrics, now, writer, errors, errorsMu, &processedPairs)
		if err != nil {
			return fmt.Errorf("safety net fallback failed: %w", err)
		}

		fmt.Printf("SAFETY NET: Successfully processed %d missed metrics using micro-batching\n", len(missedMetrics))
	} else {
		log.Printf("SAFETY NET: All %d metrics were successfully processed by batch queries", len(metricNames))
	}

	return nil
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// printBatchDebugInfo prints detailed batch information and exits
func (c *Collector) printBatchDebugInfo(batches []MetricBatch, allMetrics []string) {
	fmt.Println("\n========================================")
	fmt.Println("BATCH DEBUG MODE")
	fmt.Println("========================================")

	// Print strategy info
	strategyName := "Unknown"
	switch c.batchStrategy.(type) {
	case *AlphabeticBatchStrategy:
		strategyName = "AlphabeticBatchStrategy"
		if strat, ok := c.batchStrategy.(*AlphabeticBatchStrategy); ok {
			fmt.Printf("Chars Per Group: %d\n", strat.CharsPerBatch)
		}
	case *CustomBatchStrategy:
		strategyName = "CustomBatchStrategy"
	case *AdaptiveBatchStrategy:
		strategyName = "AdaptiveBatchStrategy"
	}
	fmt.Printf("Batch Strategy: %s\n", strategyName)
	fmt.Printf("Total Metrics: %d\n", len(allMetrics))
	fmt.Printf("Batch Concurrency: %d\n", c.maxConcurrentBatches)

	// Count unique patterns and explicit batches
	uniquePatterns := make(map[string]int)
	explicitBatches := 0
	for _, batch := range batches {
		uniquePatterns[batch.Pattern]++
		if batch.ExplicitMetrics {
			explicitBatches++
		}
	}
	fmt.Printf("Unique Patterns: %d (queries to Prometheus)\n", len(uniquePatterns))
	if explicitBatches > 0 {
		fmt.Printf("Explicit Metric Batches: %d (same query, filtered locally)\n", explicitBatches)
	}
	fmt.Println()

	// Print each batch
	totalMetricsInBatches := 0
	for i, batch := range batches {
		if batch.ExplicitMetrics && batch.BatchID != "" {
			fmt.Printf("Batch %d: %s (explicit filter from %s)\n", i+1, batch.BatchID, batch.Pattern)
		} else {
			fmt.Printf("Batch %d: %s\n", i+1, batch.Pattern)
		}
		fmt.Printf("  Metrics: %d\n", len(batch.Metrics))

		// Show sample metrics (first 5)
		sampleSize := 5
		if len(batch.Metrics) < sampleSize {
			sampleSize = len(batch.Metrics)
		}
		if sampleSize > 0 {
			fmt.Printf("  Sample: ")
			for j := 0; j < sampleSize; j++ {
				if j > 0 {
					fmt.Printf(", ")
				}
				fmt.Printf("%s", batch.Metrics[j])
			}
			if len(batch.Metrics) > sampleSize {
				fmt.Printf(", ... (%d more)", len(batch.Metrics)-sampleSize)
			}
			fmt.Println()
		}
		fmt.Println()

		totalMetricsInBatches += len(batch.Metrics)
	}

	// Print summary
	fmt.Println("========================================")
	fmt.Println("SUMMARY")
	fmt.Println("========================================")
	fmt.Printf("Total Batches: %d\n", len(batches))
	fmt.Printf("Total Metrics: %d\n", len(allMetrics))
	fmt.Printf("Metrics in Batches: %d\n", totalMetricsInBatches)

	if len(allMetrics) > 0 {
		improvement := float64(len(allMetrics)) / float64(len(batches))
		fmt.Printf("Legacy Mode Queries: %d\n", len(allMetrics))
		fmt.Printf("Batch Mode Queries: %d\n", len(batches))
		fmt.Printf("Query Reduction: %.1fx\n", improvement)
		fmt.Printf("Estimated RPS Reduction: %.1f%%\n", (1.0-1.0/improvement)*100)
	}

	// Check for missed metrics
	batchedMetrics := make(map[string]bool)
	for _, batch := range batches {
		for _, metric := range batch.Metrics {
			batchedMetrics[metric] = true
		}
	}

	var missedMetrics []string
	for _, metric := range allMetrics {
		if !batchedMetrics[metric] {
			missedMetrics = append(missedMetrics, metric)
		}
	}

	if len(missedMetrics) > 0 {
		fmt.Printf("\n⚠️  WARNING: %d metrics not in any batch (will use safety net)\n", len(missedMetrics))
		sampleSize := 10
		if len(missedMetrics) < sampleSize {
			sampleSize = len(missedMetrics)
		}
		fmt.Printf("Sample: %v\n", missedMetrics[:sampleSize])
	} else {
		fmt.Println("\n✅ All metrics covered by batches")
	}

	fmt.Println("\nExiting (debug mode enabled)")
	fmt.Println("========================================")
}

// fetchMissedMetricsWithMicroBatching processes missed metrics using optimized micro-batches
// This is more efficient than legacy mode (individual queries) while handling edge cases
func (c *Collector) fetchMissedMetricsWithMicroBatching(
	missedMetrics []string,
	now int64,
	writer MetricDataWriter,
	errors *[]ErrorRecord,
	errorsMu *sync.Mutex,
	processedPairs *sync.Map,
) error {
	if len(missedMetrics) == 0 {
		return nil
	}

	// Strategy: Create micro-batches using exact metric name patterns
	// This is much faster than individual queries while being safe for edge cases

	const microBatchSize = 10 // Process 10 metrics per batch query

	log.Printf("SAFETY NET: Creating micro-batches (size=%d) for %d missed metrics", microBatchSize, len(missedMetrics))

	var processed int32
	total := (len(missedMetrics) + microBatchSize - 1) / microBatchSize // Ceiling division

	var wg sync.WaitGroup
	sem := make(chan struct{}, c.maxConcurrentMetrics)

	// Process in micro-batches
	for i := 0; i < len(missedMetrics); i += microBatchSize {
		end := i + microBatchSize
		if end > len(missedMetrics) {
			end = len(missedMetrics)
		}

		batchMetrics := missedMetrics[i:end]

		wg.Add(1)
		sem <- struct{}{}

		go func(metrics []string, batchNum int) {
			defer wg.Done()
			defer func() { <-sem }()

			// Create exact match pattern for this micro-batch
			// Pattern: (__name__="metric1"|__name__="metric2"|...)
			pattern := createExactMatchPattern(metrics)

			// Execute batch query
			metricJobCounts, err := c.client.GetMetricJobsBatch(pattern, c.queryFilters, now)
			if err != nil {
				// If micro-batch fails, log error and record for each metric
				log.Printf("SAFETY NET: Micro-batch %d failed: %v", batchNum, err)
				errorsMu.Lock()
				for _, metric := range metrics {
					*errors = append(*errors, ErrorRecord{
						MetricName: metric,
						Operation:  "safety_net_micro_batch",
						Error:      err.Error(),
						Timestamp:  time.Now(),
					})
				}
				errorsMu.Unlock()
				return
			}

			// Process results from micro-batch
			// Note: We're using the streaming writer which expects full JobMetricData
			// For safety net, we write cardinality info directly since we have the count
			for _, mjc := range metricJobCounts {
				pairKey := mjc.MetricName + "|" + mjc.JobName
				if _, alreadyProcessed := processedPairs.LoadOrStore(pairKey, struct{}{}); !alreadyProcessed {
					// Write cardinality data (count) to the job file
					if err := writer.Write(JobMetricData{
						Job:         mjc.JobName,
						MetricName:  mjc.MetricName,
						Cardinality: strconv.FormatInt(mjc.Count, 10),
						Labels:      []string{}, // Labels not available in micro-batch
					}); err != nil {
						errorsMu.Lock()
						*errors = append(*errors, ErrorRecord{
							MetricName: mjc.MetricName,
							Operation:  "write",
							Error:      err.Error(),
							Timestamp:  time.Now(),
						})
						errorsMu.Unlock()
					}
				}
			}

			current := atomic.AddInt32(&processed, 1)
			if current%5 == 0 || current == int32(total) {
				log.Printf("SAFETY NET: Processing micro-batches: %d/%d (%.1f%%)", current, total, float64(current)/float64(total)*100)
			}
		}(batchMetrics, i/microBatchSize+1)
	}

	wg.Wait()
	return nil
}

// createExactMatchPattern creates a regex pattern for exact metric name matching
// This is used for micro-batching missed metrics
func createExactMatchPattern(metrics []string) string {
	if len(metrics) == 0 {
		return ".*"
	}

	if len(metrics) == 1 {
		// Single metric: exact match
		return "^" + regexp.QuoteMeta(metrics[0]) + "$"
	}

	// Multiple metrics: alternation with exact matches
	// Pattern: ^(metric1|metric2|metric3)$
	var builder strings.Builder
	builder.WriteString("^(")

	for i, metric := range metrics {
		if i > 0 {
			builder.WriteString("|")
		}
		builder.WriteString(regexp.QuoteMeta(metric))
	}

	builder.WriteString(")$")
	return builder.String()
}

// processMetricJobCounts processes metric-job counts from batch query
func (c *Collector) processMetricJobCounts(
	metricJobCounts []MetricJobCount,
	writer MetricDataWriter,
	errors *[]ErrorRecord,
	errorsMu *sync.Mutex,
) {
	var wg sync.WaitGroup

	// Use higher concurrency when collecting label cardinality (API-bound)
	// Otherwise use job concurrency (for simpler processing)
	concurrency := c.maxConcurrentJobs
	if c.collectLabelCardinality {
		concurrency = c.maxConcurrentLabelCardinality
	}
	sem := make(chan struct{}, concurrency)

	for _, mjc := range metricJobCounts {
		wg.Add(1)
		sem <- struct{}{}

		go func(metricName, jobName string, count int64) {
			defer wg.Done()
			defer func() { <-sem }()

			// Fetch labels
			labels, err := c.client.GetLabels(metricName, jobName, c.queryFilters)
			if err != nil {
				// Log error but continue with empty labels
				labels = []string{}
			}

			// Fetch label cardinality if enabled
			var labelCard map[string]int64
			if c.collectLabelCardinality && len(labels) > 0 {
				labelCard, _ = c.client.GetLabelCardinality(metricName, jobName, labels, c.queryFilters)
			}

			// Write immediately to disk (no memory accumulation)
			data := JobMetricData{
				Job:              jobName,
				MetricName:       metricName,
				Labels:           labels,
				Cardinality:      fmt.Sprintf("%d", count),
				LabelCardinality: labelCard,
			}

			if err := writer.Write(data); err != nil {
				errorsMu.Lock()
				*errors = append(*errors, ErrorRecord{
					MetricName: metricName,
					Operation:  "write_data",
					Error:      err.Error(),
					Timestamp:  time.Now(),
				})
				errorsMu.Unlock()
			}
		}(mjc.MetricName, mjc.JobName, mjc.Count)
	}

	wg.Wait()
}

// fetchAndStreamLegacy processes metrics using legacy mode with streaming writes
func (c *Collector) fetchAndStreamLegacy(
	metricNames []string,
	now int64,
	writer MetricDataWriter,
	errors *[]ErrorRecord,
	errorsMu *sync.Mutex,
) error {
	var wg sync.WaitGroup
	var processed int32

	sem := make(chan struct{}, c.maxConcurrentMetrics)
	total := len(metricNames)

	for _, metricName := range metricNames {
		wg.Add(1)
		sem <- struct{}{}

		go func(metric string) {
			defer wg.Done()
			defer func() { <-sem }()

			// Get jobs for this metric (existing method)
			jobNames, err := c.client.GetJobsForMetric(metric, c.queryFilters, now)
			if err != nil {
				errorsMu.Lock()
				*errors = append(*errors, ErrorRecord{
					MetricName: metric,
					Operation:  "fetch_jobs",
					Error:      err.Error(),
					Timestamp:  time.Now(),
				})
				errorsMu.Unlock()
				return
			}

			// Process each job
			for _, jobName := range jobNames {
				cardinality, _ := c.client.GetCardinality(metric, jobName, c.queryFilters, now)
				labels, _ := c.client.GetLabels(metric, jobName, c.queryFilters)

				var labelCard map[string]int64
				if c.collectLabelCardinality && len(labels) > 0 {
					labelCard, _ = c.client.GetLabelCardinality(metric, jobName, labels, c.queryFilters)
				}

				// Write immediately (no memory accumulation)
				data := JobMetricData{
					Job:              jobName,
					MetricName:       metric,
					Labels:           labels,
					Cardinality:      cardinality,
					LabelCardinality: labelCard,
				}

				writer.Write(data)
			}

			current := atomic.AddInt32(&processed, 1)
			if current%50 == 0 || current == int32(total) {
				log.Printf("Processing metrics: %d/%d (%.1f%%)", current, total, float64(current)/float64(total)*100)
			}
		}(metricName)
	}

	wg.Wait()
	return nil
}
