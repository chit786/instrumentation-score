package engine

import (
	"os"
	"testing"

	"instrumentation-score/internal/loaders"
)

func TestRuleEngine_EvaluateCardinalityRule(t *testing.T) {
	// Create a temporary rules file
	rulesContent := `
exclusion_list: []
rules:
- rule_id: "TEST-MET-01"
  description: "Test cardinality rule"
  impact: "Critical"
  validators:
    - name: "test_cardinality_check"
      type: "cardinality"
      data_source: "cardinality"
      conditions:
        - field: "count"
          operator: "lt"
          value: 10000
      threshold:
        pass_percentage: 90.0
`
	tmpRulesFile, err := os.CreateTemp("", "test_rules_*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp rules file: %v", err)
	}
	defer os.Remove(tmpRulesFile.Name())

	if _, err := tmpRulesFile.WriteString(rulesContent); err != nil {
		t.Fatalf("Failed to write rules: %v", err)
	}
	tmpRulesFile.Close()

	// Create test data file
	dataContent := `http_requests_total|1500
http_request_duration_seconds|2500
memory_usage_bytes|500
high_cardinality_metric|15000
`
	tmpDataFile, err := os.CreateTemp("", "test_data_*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp data file: %v", err)
	}
	defer os.Remove(tmpDataFile.Name())

	if _, err := tmpDataFile.WriteString(dataContent); err != nil {
		t.Fatalf("Failed to write data: %v", err)
	}
	tmpDataFile.Close()

	// Initialize engine
	engine, err := NewRuleEngine(tmpRulesFile.Name())
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	// Evaluate rules
	dataFiles := map[string]string{
		"cardinality": tmpDataFile.Name(),
	}

	results, err := engine.EvaluateRules(dataFiles)
	if err != nil {
		t.Fatalf("Failed to evaluate rules: %v", err)
	}

	// Verify results
	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	result := results[0]
	if result.RuleID != "TEST-MET-01" {
		t.Errorf("Expected rule ID TEST-MET-01, got %s", result.RuleID)
	}

	// 3 out of 4 metrics pass the cardinality check
	if result.PassedMetrics != 3 {
		t.Errorf("Expected 3 passed metrics, got %d", result.PassedMetrics)
	}
	if result.TotalMetrics != 4 {
		t.Errorf("Expected 4 total metrics, got %d", result.TotalMetrics)
	}
}

func TestRuleEngine_EvaluateFormatRule(t *testing.T) {
	// Create a temporary rules file
	rulesContent := `
exclusion_list: []
rules:
- rule_id: "TEST-MET-02"
  description: "Test format rule"
  impact: "Important"
  validators:
    - name: "test_format_check"
      type: "format"
      data_source: "labels"
      conditions:
        - field: "metric_name"
          operator: "matches"
          value: "^[a-z][a-z0-9_]*[a-z0-9]$"
      threshold:
        pass_percentage: 80.0
`
	tmpRulesFile, err := os.CreateTemp("", "test_rules_*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp rules file: %v", err)
	}
	defer os.Remove(tmpRulesFile.Name())

	if _, err := tmpRulesFile.WriteString(rulesContent); err != nil {
		t.Fatalf("Failed to write rules: %v", err)
	}
	tmpRulesFile.Close()

	// Create test data file (labels format: METRIC_NAME|LABELS)
	dataContent := `http_requests_total|method,status,path
http_request_duration_seconds|method,status
memory_usage_bytes|instance,job
InvalidMetricName|label1,label2
`
	tmpDataFile, err := os.CreateTemp("", "test_data_*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp data file: %v", err)
	}
	defer os.Remove(tmpDataFile.Name())

	if _, err := tmpDataFile.WriteString(dataContent); err != nil {
		t.Fatalf("Failed to write data: %v", err)
	}
	tmpDataFile.Close()

	// Initialize engine
	engine, err := NewRuleEngine(tmpRulesFile.Name())
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	// Evaluate rules
	dataFiles := map[string]string{
		"labels": tmpDataFile.Name(),
	}

	results, err := engine.EvaluateRules(dataFiles)
	if err != nil {
		t.Fatalf("Failed to evaluate rules: %v", err)
	}

	// Verify results
	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	result := results[0]
	// 3 out of 4 metrics pass the format check
	if result.PassedMetrics != 3 {
		t.Errorf("Expected 3 passed metrics, got %d", result.PassedMetrics)
	}
	if result.TotalMetrics != 4 {
		t.Errorf("Expected 4 total metrics, got %d", result.TotalMetrics)
	}
}

func TestRuleEngine_EvaluateLabelsRule(t *testing.T) {
	// Create a temporary rules file
	rulesContent := `
exclusion_list: []
rules:
- rule_id: "TEST-MET-03"
  description: "Test labels rule"
  impact: "Critical"
  validators:
    - name: "test_labels_check"
      type: "labels"
      data_source: "labels"
      conditions:
        - field: "labels"
          operator: "not_contains"
          value: "user_id"
      threshold:
        pass_percentage: 90.0
`
	tmpRulesFile, err := os.CreateTemp("", "test_rules_*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp rules file: %v", err)
	}
	defer os.Remove(tmpRulesFile.Name())

	if _, err := tmpRulesFile.WriteString(rulesContent); err != nil {
		t.Fatalf("Failed to write rules: %v", err)
	}
	tmpRulesFile.Close()

	// Create test data file
	dataContent := `"http_requests_total"|"method,status"
"user_requests_total"|"method,user_id"
"memory_usage_bytes"|"type"
`
	tmpDataFile, err := os.CreateTemp("", "test_labels_*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp data file: %v", err)
	}
	defer os.Remove(tmpDataFile.Name())

	if _, err := tmpDataFile.WriteString(dataContent); err != nil {
		t.Fatalf("Failed to write data: %v", err)
	}
	tmpDataFile.Close()

	// Initialize engine
	engine, err := NewRuleEngine(tmpRulesFile.Name())
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	// Evaluate rules
	dataFiles := map[string]string{
		"labels": tmpDataFile.Name(),
	}

	results, err := engine.EvaluateRules(dataFiles)
	if err != nil {
		t.Fatalf("Failed to evaluate rules: %v", err)
	}

	// Verify results
	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	result := results[0]
	// 2 out of 3 metrics pass the labels check
	if result.PassedMetrics != 2 {
		t.Errorf("Expected 2 passed metrics, got %d", result.PassedMetrics)
	}
	if result.TotalMetrics != 3 {
		t.Errorf("Expected 3 total metrics, got %d", result.TotalMetrics)
	}
}

func TestCompareValues(t *testing.T) {
	engine := &RuleEngine{}

	tests := []struct {
		name     string
		actual   float64
		operator string
		expected interface{}
		want     bool
	}{
		{"gt true", 100.0, "gt", 50.0, true},
		{"gt false", 50.0, "gt", 100.0, false},
		{"lt true", 50.0, "lt", 100.0, true},
		{"lt false", 100.0, "lt", 50.0, false},
		{"gte true equal", 100.0, "gte", 100.0, true},
		{"gte true greater", 100.0, "gte", 50.0, true},
		{"lte true equal", 100.0, "lte", 100.0, true},
		{"lte true less", 50.0, "lte", 100.0, true},
		{"eq true", 100.0, "eq", 100.0, true},
		{"eq false", 100.0, "eq", 50.0, false},
		{"int conversion", 100.0, "gt", 50, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := engine.compareValues(tt.actual, tt.operator, tt.expected)
			if got != tt.want {
				t.Errorf("compareValues() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCompareStrings(t *testing.T) {
	engine := &RuleEngine{}

	tests := []struct {
		name     string
		actual   string
		operator string
		expected interface{}
		want     bool
	}{
		{"matches valid", "http_requests_total", "matches", "^[a-z][a-z0-9_]*$", true},
		{"matches invalid", "HttpRequests", "matches", "^[a-z][a-z0-9_]*$", false},
		{"contains true", "user_id_label", "contains", "user_id", true},
		{"contains false", "method_label", "contains", "user_id", false},
		{"not_contains true", "method_label", "not_contains", "user_id", true},
		{"not_contains false", "user_id_label", "not_contains", "user_id", false},
		{"eq true", "exact_match", "eq", "exact_match", true},
		{"eq false", "not_match", "eq", "exact_match", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := engine.compareStrings(tt.actual, tt.operator, tt.expected)
			if got != tt.want {
				t.Errorf("compareStrings() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluateCondition(t *testing.T) {
	engine := &RuleEngine{}

	metric := loaders.CardinalityData{
		MetricName: "http_requests_total",
		Count:      5000,
	}

	tests := []struct {
		name       string
		conditions []ConditionConfig
		want       bool
	}{
		{
			name: "count less than",
			conditions: []ConditionConfig{
				{
					Field:    "count",
					Operator: "lt",
					Value:    10000.0,
				},
			},
			want: true,
		},
		{
			name: "count greater than",
			conditions: []ConditionConfig{
				{
					Field:    "count",
					Operator: "gt",
					Value:    1000.0,
				},
			},
			want: true,
		},
		{
			name: "metric name matches",
			conditions: []ConditionConfig{
				{
					Field:    "metric_name",
					Operator: "matches",
					Value:    "^http_.*",
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := engine.evaluateCardinalityMetric(metric, tt.conditions, "cardinality")
			if got != tt.want {
				t.Errorf("evaluateCardinalityMetric() = %v, want %v", got, tt.want)
			}
		})
	}
}
