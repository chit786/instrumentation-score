package collectors

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestPrometheusClient_GetAllMetricNames(t *testing.T) {
	tests := []struct {
		name         string
		queryFilters string
		response     interface{}
		wantCount    int
		wantErr      bool
	}{
		{
			name:         "successful fetch without filters",
			queryFilters: "",
			response: map[string]interface{}{
				"data": []string{"metric1", "metric2", "metric3"},
			},
			wantCount: 3,
			wantErr:   false,
		},
		{
			name:         "successful fetch with filters",
			queryFilters: "cluster=~\"prod.*\"",
			response: map[string]interface{}{
				"data": []string{"metric1", "metric2"},
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:         "empty result",
			queryFilters: "",
			response: map[string]interface{}{
				"data": []string{},
			},
			wantCount: 0,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/v1/label/__name__/values" {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				_ = json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			client := NewPrometheusClient(server.URL, "user:pass")
			metrics, err := client.GetAllMetricNames(tt.queryFilters)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetAllMetricNames() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(metrics) != tt.wantCount {
				t.Errorf("GetAllMetricNames() got %d metrics, want %d", len(metrics), tt.wantCount)
			}
		})
	}
}

func TestPrometheusClient_GetJobsForMetric(t *testing.T) {
	tests := []struct {
		name         string
		metricName   string
		queryFilters string
		response     interface{}
		wantJobs     []string
		wantErr      bool
	}{
		{
			name:         "successful fetch with jobs",
			metricName:   "http_requests_total",
			queryFilters: "",
			response: map[string]interface{}{
				"data": map[string]interface{}{
					"result": []map[string]interface{}{
						{"metric": map[string]string{"job": "api-service"}},
						{"metric": map[string]string{"job": "web-service"}},
					},
				},
			},
			wantJobs: []string{"api-service", "web-service"},
			wantErr:  false,
		},
		{
			name:         "no jobs found",
			metricName:   "nonexistent_metric",
			queryFilters: "",
			response: map[string]interface{}{
				"data": map[string]interface{}{
					"result": []map[string]interface{}{},
				},
			},
			wantJobs: []string{},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/v1/query" {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				_ = json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			client := NewPrometheusClient(server.URL, "user:pass")
			jobs, err := client.GetJobsForMetric(tt.metricName, tt.queryFilters, 1234567890)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetJobsForMetric() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(jobs) != len(tt.wantJobs) {
				t.Errorf("GetJobsForMetric() got %d jobs, want %d", len(jobs), len(tt.wantJobs))
			}
		})
	}
}

func TestPrometheusClient_GetCardinality(t *testing.T) {
	tests := []struct {
		name         string
		metricName   string
		job          string
		queryFilters string
		response     interface{}
		wantCard     string
		wantErr      bool
	}{
		{
			name:         "successful cardinality fetch",
			metricName:   "http_requests_total",
			job:          "api-service",
			queryFilters: "",
			response: map[string]interface{}{
				"data": map[string]interface{}{
					"result": []map[string]interface{}{
						{"value": []interface{}{1234567890, "42"}},
					},
				},
			},
			wantCard: "42",
			wantErr:  false,
		},
		{
			name:         "no result",
			metricName:   "http_requests_total",
			job:          "api-service",
			queryFilters: "",
			response: map[string]interface{}{
				"data": map[string]interface{}{
					"result": []map[string]interface{}{},
				},
			},
			wantCard: "0",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/v1/query" {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				_ = json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			client := NewPrometheusClient(server.URL, "user:pass")
			card, err := client.GetCardinality(tt.metricName, tt.job, tt.queryFilters, 1234567890)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetCardinality() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if card != tt.wantCard {
				t.Errorf("GetCardinality() = %v, want %v", card, tt.wantCard)
			}
		})
	}
}

func TestPrometheusClient_GetLabels(t *testing.T) {
	tests := []struct {
		name         string
		metricName   string
		job          string
		queryFilters string
		response     interface{}
		wantLabels   int
		wantErr      bool
	}{
		{
			name:         "successful labels fetch via query",
			metricName:   "http_requests_total",
			job:          "api-service",
			queryFilters: "",
			response: map[string]interface{}{
				"data": map[string]interface{}{
					"result": []map[string]interface{}{
						{
							"metric": map[string]interface{}{
								"__name__": "http_requests_total",
								"method":   "GET",
								"status":   "200",
								"job":      "api-service",
							},
						},
					},
				},
			},
			wantLabels: 3, // method, status, job (excluding __name__)
			wantErr:    false,
		},
		{
			name:         "fallback to labels API",
			metricName:   "http_requests_total",
			job:          "api-service",
			queryFilters: "",
			response: map[string]interface{}{
				"data": map[string]interface{}{
					"result": []map[string]interface{}{},
				},
			},
			wantLabels: 3, // Falls back to labels API which returns 3 labels
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Respond to query endpoint
				if r.URL.Path == "/api/v1/query" {
					_ = json.NewEncoder(w).Encode(tt.response)
					return
				}
				// Fallback to labels API
				if r.URL.Path == "/api/v1/labels" {
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"data": []string{"method", "status", "job"},
					})
					return
				}
				t.Errorf("unexpected path: %s", r.URL.Path)
			}))
			defer server.Close()

			client := NewPrometheusClient(server.URL, "user:pass")
			labels, err := client.GetLabels(tt.metricName, tt.job, tt.queryFilters)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetLabels() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(labels) != tt.wantLabels {
				t.Errorf("GetLabels() got %d labels, want %d", len(labels), tt.wantLabels)
			}
		})
	}
}

func TestPrometheusClient_ErrorHandling(t *testing.T) {
	t.Run("handles 429 rate limit", func(t *testing.T) {
		callCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "rate limit exceeded",
			})
		}))
		defer server.Close()

		client := NewPrometheusClient(server.URL, "user:pass")
		_, err := client.GetCardinality("test_metric", "test_job", "", 1234567890)

		if err == nil {
			t.Error("expected error for 429 response")
		}
	})

	t.Run("handles HTTP errors", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "internal server error",
			})
		}))
		defer server.Close()

		client := NewPrometheusClient(server.URL, "user:pass")
		_, err := client.GetJobsForMetric("test_metric", "", 1234567890)

		if err == nil {
			t.Error("expected error for 500 response")
		}
	})
}

func TestPrometheusClient_RetryLogic(t *testing.T) {
	t.Run("retries on network errors", func(t *testing.T) {
		var attemptCount int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			count := atomic.AddInt32(&attemptCount, 1)
			if count < 3 {
				w.WriteHeader(http.StatusBadGateway)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"error": "connection refused",
				})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []string{"metric1", "metric2"},
			})
		}))
		defer server.Close()

		client := NewPrometheusClient(server.URL, "user:pass")
		client.SetRetryCount(2)
		
		metrics, err := client.GetAllMetricNames("")
		
		if err != nil {
			t.Errorf("expected success after retries, got error: %v", err)
		}
		if len(metrics) != 2 {
			t.Errorf("expected 2 metrics, got %d", len(metrics))
		}
		if atomic.LoadInt32(&attemptCount) != 3 {
			t.Errorf("expected 3 attempts (1 initial + 2 retries), got %d", atomic.LoadInt32(&attemptCount))
		}
	})

	t.Run("fails after max retries", func(t *testing.T) {
		var attemptCount int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&attemptCount, 1)
			w.WriteHeader(http.StatusBadGateway)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "connection refused",
			})
		}))
		defer server.Close()

		client := NewPrometheusClient(server.URL, "user:pass")
		client.SetRetryCount(2)
		
		_, err := client.GetAllMetricNames("")
		
		if err == nil {
			t.Error("expected error after max retries")
		}
		if atomic.LoadInt32(&attemptCount) != 3 {
			t.Errorf("expected 3 attempts (1 initial + 2 retries), got %d", atomic.LoadInt32(&attemptCount))
		}
	})

	t.Run("succeeds on first attempt", func(t *testing.T) {
		var attemptCount int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&attemptCount, 1)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []string{"metric1"},
			})
		}))
		defer server.Close()

		client := NewPrometheusClient(server.URL, "user:pass")
		client.SetRetryCount(2)
		
		metrics, err := client.GetAllMetricNames("")
		
		if err != nil {
			t.Errorf("expected success, got error: %v", err)
		}
		if len(metrics) != 1 {
			t.Errorf("expected 1 metric, got %d", len(metrics))
		}
		if atomic.LoadInt32(&attemptCount) != 1 {
			t.Errorf("expected 1 attempt, got %d", atomic.LoadInt32(&attemptCount))
		}
	})
}
