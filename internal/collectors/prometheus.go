package collectors

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
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

	var result struct {
		Data []string `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
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
