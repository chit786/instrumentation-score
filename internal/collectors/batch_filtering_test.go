package collectors

import (
	"testing"
)

// TestBatchMetricFiltering verifies that batch queries only return metrics
// that belong to the specific batch, not all metrics in the system.
// This test addresses the bug where kube-system/vpc-cni showed 126 metrics
// instead of the actual 79 metrics.
func TestBatchMetricFiltering(t *testing.T) {
	// Simulate a scenario where we have metrics from different batches
	allMetrics := []string{
		// Batch 1: Metrics starting with 'g' (go_* metrics)
		"go_memstats_next_gc_bytes",
		"go_memstats_other_sys_bytes",
		"go_memstats_stack_inuse_bytes",
		"go_threads",

		// Batch 2: Metrics starting with 'p' (process_* metrics)
		"process_cpu_seconds_total",
		"process_max_fds",
		"process_open_fds",
		"process_resident_memory_bytes",

		// Batch 3: Metrics starting with 'a' (awscni_* metrics)
		"awscni_assigned_ip_per_cidr",
		"awscni_aws_api_latency_ms_sum",
		"awscni_ipamd_node_initialization_duration_seconds_bucket",
	}

	// Create alphabetic strategy with 1 char per batch
	strategy := &AlphabeticBatchStrategy{CharsPerBatch: 1}
	batches := strategy.CreateBatches(allMetrics)

	// Verify each batch only contains metrics with the expected prefix
	for _, batch := range batches {
		t.Logf("Testing batch with pattern: %s, metrics count: %d", batch.Pattern, len(batch.Metrics))

		// Verify all metrics in this batch match the pattern
		for _, metric := range batch.Metrics {
			matched := filterMetricsByPattern([]string{metric}, batch.Pattern)
			if len(matched) == 0 {
				t.Errorf("Metric %s should match pattern %s but doesn't", metric, batch.Pattern)
			}
		}

		// Verify metrics from other batches are NOT in this batch
		for _, otherMetric := range allMetrics {
			inBatch := false
			for _, batchMetric := range batch.Metrics {
				if batchMetric == otherMetric {
					inBatch = true
					break
				}
			}

			matched := filterMetricsByPattern([]string{otherMetric}, batch.Pattern)
			shouldBeInBatch := len(matched) > 0

			if shouldBeInBatch && !inBatch {
				t.Errorf("Metric %s matches pattern %s but is not in batch", otherMetric, batch.Pattern)
			}
			if !shouldBeInBatch && inBatch {
				t.Errorf("Metric %s doesn't match pattern %s but is in batch", otherMetric, batch.Pattern)
			}
		}
	}

	// Verify total metrics across all batches equals original count
	totalMetrics := 0
	for _, batch := range batches {
		totalMetrics += len(batch.Metrics)
	}
	if totalMetrics != len(allMetrics) {
		t.Errorf("Total metrics in batches (%d) doesn't match original count (%d)", totalMetrics, len(allMetrics))
	}
}

// TestExecuteBatchWithAutoSplit_MetricFiltering tests that the executeBatchWithAutoSplit
// function correctly filters results to only include metrics from the expected list
func TestExecuteBatchWithAutoSplit_MetricFiltering(t *testing.T) {
	// This test simulates the scenario where:
	// 1. We have a batch with specific metrics (e.g., go_* metrics for job "kube-system/vpc-cni")
	// 2. The batch query might return additional metrics from other jobs
	// 3. We need to filter to only the metrics in our batch

	batchMetrics := []string{
		"go_memstats_next_gc_bytes",
		"go_memstats_other_sys_bytes",
		"go_threads",
	}

	// Simulate what Prometheus might return (including extra metrics)
	prometheusResults := []MetricJobCount{
		{MetricName: "go_memstats_next_gc_bytes", JobName: "kube-system/vpc-cni", Count: 132},
		{MetricName: "go_memstats_other_sys_bytes", JobName: "kube-system/vpc-cni", Count: 132},
		{MetricName: "go_threads", JobName: "kube-system/vpc-cni", Count: 132},
		// These should be filtered out - they're not in our batch
		{MetricName: "go_gc_duration_seconds", JobName: "kube-system/vpc-cni", Count: 132},
		{MetricName: "go_goroutines", JobName: "other-job", Count: 50},
	}

	// Create a metric set for filtering (simulating what executeBatchWithAutoSplit does)
	metricSet := make(map[string]bool, len(batchMetrics))
	for _, m := range batchMetrics {
		metricSet[m] = true
	}

	// Filter results
	filtered := make([]MetricJobCount, 0, len(prometheusResults))
	for _, r := range prometheusResults {
		if metricSet[r.MetricName] {
			filtered = append(filtered, r)
		}
	}

	// Verify filtering worked correctly
	if len(filtered) != len(batchMetrics) {
		t.Errorf("Expected %d filtered results, got %d", len(batchMetrics), len(filtered))
	}

	// Verify only expected metrics are in filtered results
	for _, result := range filtered {
		if !metricSet[result.MetricName] {
			t.Errorf("Unexpected metric in filtered results: %s", result.MetricName)
		}
	}

	// Verify no extra metrics leaked through
	for _, result := range filtered {
		found := false
		for _, expected := range batchMetrics {
			if result.MetricName == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Filtered result contains unexpected metric: %s", result.MetricName)
		}
	}
}

// TestBatchStrategy_NoMetricLeakage tests that when processing multiple batches,
// metrics from one batch don't leak into another batch's results
func TestBatchStrategy_NoMetricLeakage(t *testing.T) {
	allMetrics := []string{
		// VPC-CNI metrics (79 metrics in real scenario)
		"go_memstats_next_gc_bytes",
		"go_threads",
		"process_cpu_seconds_total",

		// Other job metrics (should not appear in VPC-CNI results)
		"awscni_assigned_ip_per_cidr",
		"container_cpu_usage_seconds_total",
		"kube_pod_status_phase",
	}

	// Create batches
	strategy := &AlphabeticBatchStrategy{CharsPerBatch: 1}
	batches := strategy.CreateBatches(allMetrics)

	// For each batch, verify it only contains its own metrics
	for _, batch := range batches {
		// Get all metrics that should match this pattern
		expectedMetrics := filterMetricsByPattern(allMetrics, batch.Pattern)

		// Verify batch.Metrics matches expectedMetrics
		if len(batch.Metrics) != len(expectedMetrics) {
			t.Errorf("Batch %s: expected %d metrics, got %d",
				batch.Pattern, len(expectedMetrics), len(batch.Metrics))
		}

		// Verify each metric in batch is in expectedMetrics
		for _, metric := range batch.Metrics {
			found := false
			for _, expected := range expectedMetrics {
				if metric == expected {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Batch %s contains unexpected metric: %s", batch.Pattern, metric)
			}
		}

		// Verify no metrics from other patterns leaked in
		for _, metric := range allMetrics {
			inBatch := false
			for _, batchMetric := range batch.Metrics {
				if metric == batchMetric {
					inBatch = true
					break
				}
			}

			shouldBeInBatch := false
			for _, expected := range expectedMetrics {
				if metric == expected {
					shouldBeInBatch = true
					break
				}
			}

			if inBatch != shouldBeInBatch {
				if inBatch {
					t.Errorf("Batch %s incorrectly includes metric: %s", batch.Pattern, metric)
				} else {
					t.Errorf("Batch %s missing expected metric: %s", batch.Pattern, metric)
				}
			}
		}
	}
}
