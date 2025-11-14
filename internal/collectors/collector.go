package collectors

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// JobMetricData represents metric data for a specific job
type JobMetricData struct {
	Job         string
	MetricName  string
	Labels      []string
	Cardinality string
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
	client        *PrometheusClient
	queryFilters  string
	maxConcurrent int
}

// NewCollector creates a new metrics collector
func NewCollector(baseURL, login, queryFilters string) *Collector {
	return &Collector{
		client:        NewPrometheusClient(baseURL, login),
		queryFilters:  queryFilters,
		maxConcurrent: 5,
	}
}

// NewCollectorWithClient creates a new metrics collector with an existing Prometheus client
func NewCollectorWithClient(client *PrometheusClient, queryFilters string) *Collector {
	return &Collector{
		client:        client,
		queryFilters:  queryFilters,
		maxConcurrent: 5,
	}
}

// SetRetryCount sets the number of retry attempts for failed requests
func (c *Collector) SetRetryCount(count int) {
	c.client.SetRetryCount(count)
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

	sem := make(chan struct{}, c.maxConcurrent)
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
				fmt.Printf("\rProcessing metrics: %d/%d (%.1f%%)", current, total, float64(current)/float64(total)*100)
			}
		}(metricName)
	}

	wg.Wait()
	fmt.Println()
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

	var results []JobMetricData
	var wg sync.WaitGroup
	var mu sync.Mutex
	sem := make(chan struct{}, 3)

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
			results = append(results, JobMetricData{
				Job:         job,
				MetricName:  metricName,
				Labels:      labels,
				Cardinality: cardinality,
			})
			mu.Unlock()
		}(jobName)
	}
	wg.Wait()
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
		if _, err := writer.WriteString("JOB|METRIC_NAME|LABELS|CARDINALITY\n"); err != nil {
			errMsg := fmt.Sprintf("failed to write header for job %s: %v", data.Job, err)
			writeErrors = append(writeErrors, errMsg)
			skippedJobs[data.Job] = true
			fmt.Printf("WARNING: %s\n", errMsg)
			continue
		}
		}

	writer := jobWriters[data.Job]
	labelsStr := strings.Join(data.Labels, ",")
	line := fmt.Sprintf("%s|%s|%s|%s\n", data.Job, data.MetricName, labelsStr, data.Cardinality)
	if _, err := writer.WriteString(line); err != nil {
		errMsg := fmt.Sprintf("failed to write data for job %s: %v", data.Job, err)
		writeErrors = append(writeErrors, errMsg)
		fmt.Printf("WARNING: %s\n", errMsg)
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
		return fmt.Errorf("failed to write error file header: %w", err)
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
