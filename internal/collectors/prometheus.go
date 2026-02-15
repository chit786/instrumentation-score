package collectors

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// PrometheusClient handles communication with Prometheus API
type PrometheusClient struct {
	BaseURL    string
	Login      string
	Client     *http.Client
	RetryCount int
}

// NewPrometheusClient creates a new Prometheus API client
func NewPrometheusClient(baseURL, login string) *PrometheusClient {
	return &PrometheusClient{
		BaseURL:    baseURL,
		Login:      login,
		Client:     &http.Client{Timeout: 30 * time.Second},
		RetryCount: 2,
	}
}

// SetRetryCount sets the number of retry attempts for failed requests
func (c *PrometheusClient) SetRetryCount(count int) {
	c.RetryCount = count
}

// doRequestWithRetry executes an HTTP request with retry logic
func (c *PrometheusClient) doRequestWithRetry(req *http.Request) (*http.Response, error) {
	var lastErr error
	var resp *http.Response

	for attempt := 0; attempt <= c.RetryCount; attempt++ {
		if attempt > 0 {
			waitTime := time.Duration(attempt) * time.Second
			time.Sleep(waitTime)
		}

		resp, lastErr = c.Client.Do(req)
		if lastErr != nil {
			if attempt < c.RetryCount {
				continue
			}
			return nil, lastErr
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return resp, nil
		}

		if resp.StatusCode == 502 || resp.StatusCode == 503 || resp.StatusCode == 504 {
			resp.Body.Close()
			if attempt < c.RetryCount {
				continue
			}
		}

		return resp, nil
	}
	return resp, lastErr
}

// NewPrometheusClientFromEnv creates a Prometheus client from environment variables
// Returns error if required environment variables are not set
// Note: 'login' is optional (for local/unauthenticated Prometheus instances)
func NewPrometheusClientFromEnv() (*PrometheusClient, error) {
	login := os.Getenv("login")
	baseURL := os.Getenv("url")

	if baseURL == "" {
		return nil, fmt.Errorf("missing required environment variable: 'url' must be set\n\n" +
			"Examples:\n" +
			"  # For authenticated Prometheus (e.g., Grafana Cloud)\n" +
			"  export login=\"user:password\"\n" +
			"  export url=\"https://prometheus.example.com\"\n\n" +
			"  # For local/unauthenticated Prometheus\n" +
			"  export url=\"http://localhost:9090\"")
	}

	return NewPrometheusClient(baseURL, login), nil
}

// PrometheusResponse represents a Prometheus query response
type PrometheusResponse struct {
	Data struct {
		Result []struct {
			Value []interface{} `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

// addAuthIfNeeded adds Basic Auth to the request if login credentials are provided
func (c *PrometheusClient) addAuthIfNeeded(req *http.Request) {
	if c.Login != "" {
		parts := strings.Split(c.Login, ":")
		if len(parts) == 2 {
			req.SetBasicAuth(parts[0], parts[1])
		}
	}
}

// GetAllMetricNames fetches all metric names from Prometheus with optional filtering
func (c *PrometheusClient) GetAllMetricNames(queryFilters string) ([]string, error) {
	endpoint := fmt.Sprintf("%s/api/v1/label/__name__/values", c.BaseURL)

	if queryFilters != "" {
		matchSelector := fmt.Sprintf("{%s}", queryFilters)
		params := url.Values{}
		params.Add("match[]", matchSelector)
		endpoint = fmt.Sprintf("%s?%s", endpoint, params.Encode())
	}

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	c.addAuthIfNeeded(req)

	resp, err := c.doRequestWithRetry(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for HTTP errors
	if resp.StatusCode != 200 {
		var errorResp struct {
			Status    string `json:"status"`
			ErrorType string `json:"errorType"`
			Error     string `json:"error"`
		}
		errorMsg := string(body)
		if json.Unmarshal(body, &errorResp) == nil && errorResp.Error != "" {
			errorMsg = errorResp.Error
		}
		return nil, fmt.Errorf("HTTP %d (%s) - failed to fetch metric names: %s", resp.StatusCode, resp.Status, errorMsg)
	}

	var result struct {
		Status string   `json:"status"`
		Data   []string `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w (body: %s)", err, string(body[:min(len(body), 200)]))
	}

	// Check Prometheus API status
	if result.Status != "success" {
		return nil, fmt.Errorf("Prometheus API returned status: %s", result.Status)
	}

	return result.Data, nil
}

// GetJobsForMetric fetches all job names for a specific metric
func (c *PrometheusClient) GetJobsForMetric(metricName, queryFilters string, now int64) ([]string, error) {
	var query string
	if queryFilters != "" {
		query = fmt.Sprintf(`count by (job) ({__name__="%s",%s})`, metricName, queryFilters)
	} else {
		query = fmt.Sprintf(`count by (job) ({__name__="%s"})`, metricName)
	}

	params := url.Values{}
	params.Set("query", query)
	params.Set("time", fmt.Sprintf("%d", now))

	endpoint := fmt.Sprintf("%s/api/v1/query?%s", c.BaseURL, params.Encode())
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("request creation failed: %w", err)
	}
	c.addAuthIfNeeded(req)

	resp, err := c.doRequestWithRetry(req)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != 200 {
		var errorResp struct {
			Status    string `json:"status"`
			ErrorType string `json:"errorType"`
			Error     string `json:"error"`
		}
		errorMsg := string(body)
		if json.Unmarshal(body, &errorResp) == nil && errorResp.Error != "" {
			errorMsg = errorResp.Error
		}
		if resp.StatusCode == 429 {
			time.Sleep(2 * time.Second)
		}
		return nil, fmt.Errorf("HTTP %d (%s) - query: count by (job) - error: %s",
			resp.StatusCode, resp.Status, errorMsg)
	}

	var result struct {
		Data struct {
			Result []struct {
				Metric map[string]string `json:"metric"`
			} `json:"result"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	var jobNames []string
	for _, series := range result.Data.Result {
		if jobName, ok := series.Metric["job"]; ok {
			jobNames = append(jobNames, jobName)
		}
	}

	return jobNames, nil
}

// GetCardinality fetches the cardinality for a specific metric and job
func (c *PrometheusClient) GetCardinality(metricName, job, queryFilters string, now int64) (string, error) {
	var query string
	if queryFilters != "" {
		query = fmt.Sprintf(`count({__name__="%s",%s,job="%s"})`, metricName, queryFilters, job)
	} else {
		query = fmt.Sprintf(`count({__name__="%s",job="%s"})`, metricName, job)
	}

	params := url.Values{}
	params.Set("query", query)
	params.Set("time", fmt.Sprintf("%d", now))

	endpoint := fmt.Sprintf("%s/api/v1/query?%s", c.BaseURL, params.Encode())
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return "0", err
	}
	c.addAuthIfNeeded(req)

	resp, err := c.doRequestWithRetry(req)
	if err != nil {
		return "0", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "0", err
	}

	if resp.StatusCode != 200 {
		var errorResp struct {
			Error string `json:"error"`
		}
		errorMsg := string(body)
		if json.Unmarshal(body, &errorResp) == nil && errorResp.Error != "" {
			errorMsg = errorResp.Error
		}
		if resp.StatusCode == 429 {
			time.Sleep(2 * time.Second)
		}
		return "0", fmt.Errorf("HTTP %d - cardinality query - job: %s - error: %s",
			resp.StatusCode, job, errorMsg)
	}

	var result PrometheusResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "0", err
	}

	if len(result.Data.Result) > 0 && len(result.Data.Result[0].Value) > 1 {
		if countStr, ok := result.Data.Result[0].Value[1].(string); ok {
			return countStr, nil
		}
	}
	return "0", nil
}

// GetLabels fetches all labels for a specific metric and job
func (c *PrometheusClient) GetLabels(metricName, job, queryFilters string) ([]string, error) {
	labels, err := c.getLabelsViaQuery(metricName, job, queryFilters)
	if err == nil && len(labels) > 0 {
		return labels, nil
	}

	return c.getLabelsViaAPI(metricName, job, queryFilters)
}

func (c *PrometheusClient) getLabelsViaQuery(metricName, job, queryFilters string) ([]string, error) {
	var query string
	if queryFilters != "" {
		query = fmt.Sprintf(`{__name__="%s",%s,job="%s"}`, metricName, queryFilters, job)
	} else {
		query = fmt.Sprintf(`{__name__="%s",job="%s"}`, metricName, job)
	}

	params := url.Values{}
	params.Set("query", query)

	endpoint := fmt.Sprintf("%s/api/v1/query?%s", c.BaseURL, params.Encode())
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	c.addAuthIfNeeded(req)

	resp, err := c.doRequestWithRetry(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		if resp.StatusCode == 429 {
			time.Sleep(2 * time.Second)
		}
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var result struct {
		Data struct {
			Result []struct {
				Metric map[string]interface{} `json:"metric"`
			} `json:"result"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	labelSet := make(map[string]bool)
	for _, r := range result.Data.Result {
		for key := range r.Metric {
			if key != "__name__" {
				labelSet[key] = true
			}
		}
	}

	var labels []string
	for label := range labelSet {
		labels = append(labels, label)
	}
	return labels, nil
}

func (c *PrometheusClient) getLabelsViaAPI(metricName, job, queryFilters string) ([]string, error) {
	params := url.Values{}
	var matchQuery string
	if queryFilters != "" {
		matchQuery = fmt.Sprintf(`{__name__="%s",%s,job="%s"}`, metricName, queryFilters, job)
	} else {
		matchQuery = fmt.Sprintf(`{__name__="%s",job="%s"}`, metricName, job)
	}
	params.Set("match[]", matchQuery)

	endpoint := fmt.Sprintf("%s/api/v1/labels?%s", c.BaseURL, params.Encode())
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	c.addAuthIfNeeded(req)

	resp, err := c.doRequestWithRetry(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		var errorResp struct {
			Error string `json:"error"`
		}
		errorMsg := string(body)
		if json.Unmarshal(body, &errorResp) == nil && errorResp.Error != "" {
			errorMsg = errorResp.Error
		}
		if resp.StatusCode == 429 {
			time.Sleep(2 * time.Second)
		}
		return nil, fmt.Errorf("HTTP %d - labels API - job: %s - error: %s",
			resp.StatusCode, job, errorMsg)
	}

	var result struct {
		Data []string `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	var labels []string
	for _, label := range result.Data {
		if label != "__name__" {
			labels = append(labels, label)
		}
	}
	return labels, nil
}

// GetLabelCardinality fetches per-label cardinality using Mimir's cardinality API
// This uses the /api/v1/cardinality/label_values endpoint which is more accurate than estimates
// Reference: https://grafana.com/docs/mimir/latest/query/query-metric-labels/
func (c *PrometheusClient) GetLabelCardinality(metricName, job string, labels []string, queryFilters string) (map[string]int64, error) {
	// Build the selector for this metric and job
	var selector string
	if queryFilters != "" {
		selector = fmt.Sprintf(`{__name__="%s",%s,job="%s"}`, metricName, queryFilters, job)
	} else {
		selector = fmt.Sprintf(`{__name__="%s",job="%s"}`, metricName, job)
	}

	// Build URL with query parameters (Grafana Cloud expects form-encoded params, not JSON body)
	endpoint := fmt.Sprintf("%s/api/v1/cardinality/label_values", c.BaseURL)

	// Build form data with label_names[] array parameter
	params := url.Values{}
	for _, label := range labels {
		params.Add("label_names[]", label)
	}
	params.Set("selector", selector)

	req, err := http.NewRequest("POST", endpoint, strings.NewReader(params.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c.addAuthIfNeeded(req)

	resp, err := c.doRequestWithRetry(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != 200 {
		var errorResp struct {
			Error string `json:"error"`
		}
		errorMsg := string(body)
		if json.Unmarshal(body, &errorResp) == nil && errorResp.Error != "" {
			errorMsg = errorResp.Error
		}
		if resp.StatusCode == 429 {
			time.Sleep(2 * time.Second)
		}
		return nil, fmt.Errorf("HTTP %d - label cardinality API - job: %s - error: %s",
			resp.StatusCode, job, errorMsg)
	}

	// Parse the response (Grafana Cloud format)
	var result struct {
		Labels []struct {
			LabelName        string `json:"label_name"`
			SeriesCount      int64  `json:"series_count"`
			LabelValuesCount int64  `json:"label_values_count"`
		} `json:"labels"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Build the cardinality map using label_values_count (unique values per label)
	cardinalityMap := make(map[string]int64)
	for _, item := range result.Labels {
		cardinalityMap[item.LabelName] = item.LabelValuesCount
	}

	return cardinalityMap, nil
}

// Domain errors for batch queries
var (
	ErrPayloadTooLarge = errors.New("payload too large")
	ErrPatternTooDeep  = errors.New("pattern recursion depth exceeded")
)

// MetricJobCount represents a metric-job combination with count
type MetricJobCount struct {
	MetricName string
	JobName    string
	Count      int64
}

// GetMetricJobsBatch fetches multiple metric-job combinations in a single
// batch query using regex pattern matching.
//
// The query uses the format:
//
//	count by (__name__, job) ({__name__=~"pattern", <queryFilters>})
//
// This method is significantly more efficient than individual queries when
// dealing with large numbers of metrics, reducing API load by up to 99%.
//
// Parameters:
//   - metricNamePattern: Regex pattern for metric names (e.g., "[a-c].*")
//   - queryFilters: Additional PromQL label filters (e.g., "cluster=~\"prod.*\"")
//   - now: Unix timestamp for query evaluation
//
// Returns:
//   - []MetricJobCount: Array of metric-job-count tuples
//   - error: ErrPayloadTooLarge if response exceeds limits, or other errors
func (c *PrometheusClient) GetMetricJobsBatch(
	metricNamePattern string,
	queryFilters string,
	now int64,
) ([]MetricJobCount, error) {
	var query string
	if queryFilters != "" {
		query = fmt.Sprintf(`count by (__name__, job) ({__name__=~"%s",%s})`,
			metricNamePattern, queryFilters)
	} else {
		query = fmt.Sprintf(`count by (__name__, job) ({__name__=~"%s"})`,
			metricNamePattern)
	}

	params := url.Values{}
	params.Set("query", query)
	params.Set("time", fmt.Sprintf("%d", now))

	endpoint := fmt.Sprintf("%s/api/v1/query?%s", c.BaseURL, params.Encode())
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("request creation failed: %w", err)
	}
	c.addAuthIfNeeded(req)

	resp, err := c.doRequestWithRetry(req)
	if err != nil {
		return nil, fmt.Errorf("batch query failed for pattern %s: %w", metricNamePattern, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != 200 {
		var errorResp struct {
			Status    string `json:"status"`
			ErrorType string `json:"errorType"`
			Error     string `json:"error"`
		}
		errorMsg := string(body)
		if json.Unmarshal(body, &errorResp) == nil && errorResp.Error != "" {
			errorMsg = errorResp.Error
		}

		// Check for payload too large error
		if strings.Contains(errorMsg, "response is too large") ||
			strings.Contains(errorMsg, "too many results") {
			return nil, fmt.Errorf("%w: pattern %s - %s",
				ErrPayloadTooLarge, metricNamePattern, errorMsg)
		}

		if resp.StatusCode == 429 {
			time.Sleep(2 * time.Second)
		}
		return nil, fmt.Errorf("HTTP %d (%s) - batch query for pattern %s - error: %s",
			resp.StatusCode, resp.Status, metricNamePattern, errorMsg)
	}

	var result struct {
		Data struct {
			Result []struct {
				Metric map[string]string `json:"metric"`
				Value  []interface{}     `json:"value"`
			} `json:"result"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	var results []MetricJobCount
	for _, series := range result.Data.Result {
		metricName := series.Metric["__name__"]
		jobName := series.Metric["job"]

		if metricName == "" || jobName == "" {
			continue
		}

		// Parse count value
		var count int64
		if len(series.Value) > 1 {
			if countStr, ok := series.Value[1].(string); ok {
				count, _ = strconv.ParseInt(countStr, 10, 64)
			} else if countFloat, ok := series.Value[1].(float64); ok {
				count = int64(countFloat)
			}
		}

		results = append(results, MetricJobCount{
			MetricName: metricName,
			JobName:    jobName,
			Count:      count,
		})
	}

	return results, nil
}

// executeBatchWithAutoSplit handles payload limit errors by splitting batches
// Implements progressive fallback strategy for edge cases
func (c *PrometheusClient) executeBatchWithAutoSplit(
	pattern string,
	queryFilters string,
	now int64,
	depth int,
	metrics []string,
) ([]MetricJobCount, error) {
	if depth > 5 {
		log.Printf("WARNING: Max recursion depth reached for pattern: %s", pattern)
		// Fallback to legacy mode for this pattern
		return c.fallbackToLegacyMode(pattern, queryFilters, now, metrics)
	}

	results, err := c.GetMetricJobsBatch(pattern, queryFilters, now)
	if err == nil {
		// Batch query succeeded - return all results
		// The regex pattern already ensures we only get matching metrics
		return results, nil
	}

	// Check if payload too large
	if !errors.Is(err, ErrPayloadTooLarge) {
		return nil, err
	}

	log.Printf("Payload too large for pattern '%s', attempting to split...", pattern)

	// Split pattern and retry
	subPatterns := splitPattern(pattern)
	if len(subPatterns) == 0 {
		// Cannot split further - fallback to legacy mode
		log.Printf("WARNING: Cannot split pattern '%s' further, falling back to legacy mode", pattern)
		return c.fallbackToLegacyMode(pattern, queryFilters, now, metrics)
	}

	var allResults []MetricJobCount
	for _, subPattern := range subPatterns {
		subResults, err := c.executeBatchWithAutoSplit(subPattern, queryFilters, now, depth+1, metrics)
		if err != nil {
			return nil, err
		}
		allResults = append(allResults, subResults...)
	}

	return allResults, nil
}

// fallbackToLegacyMode processes metrics individually when batch queries fail
// This handles pathological cases where metric names are highly concentrated
func (c *PrometheusClient) fallbackToLegacyMode(
	pattern string,
	queryFilters string,
	now int64,
	allMetrics []string,
) ([]MetricJobCount, error) {
	log.Printf("FALLBACK: Processing pattern '%s' using legacy mode (individual queries)", pattern)

	// Filter metrics that match this pattern
	matchingMetrics := filterMetricsByPattern(allMetrics, pattern)

	if len(matchingMetrics) == 0 {
		return []MetricJobCount{}, nil
	}

	log.Printf("FALLBACK: Found %d metrics matching pattern '%s', querying individually...",
		len(matchingMetrics), pattern)

	var allResults []MetricJobCount
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 3) // Conservative concurrency to avoid head spike

	for _, metricName := range matchingMetrics {
		wg.Add(1)
		sem <- struct{}{}

		go func(metric string) {
			defer wg.Done()
			defer func() { <-sem }()

			// Use legacy GetJobsForMetric method
			jobNames, err := c.GetJobsForMetric(metric, queryFilters, now)
			if err != nil {
				log.Printf("ERROR: Failed to get jobs for metric '%s': %v", metric, err)
				return
			}

			// For each job, get cardinality
			for _, jobName := range jobNames {
				cardinality, err := c.GetCardinality(metric, jobName, queryFilters, now)
				if err != nil {
					continue
				}

				count, _ := strconv.ParseInt(cardinality, 10, 64)

				mu.Lock()
				allResults = append(allResults, MetricJobCount{
					MetricName: metric,
					JobName:    jobName,
					Count:      count,
				})
				mu.Unlock()
			}
		}(metricName)
	}

	wg.Wait()

	log.Printf("FALLBACK: Completed legacy mode processing for pattern '%s', got %d results",
		pattern, len(allResults))

	return allResults, nil
}
