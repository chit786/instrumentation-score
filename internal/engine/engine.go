package engine

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"instrumentation-score-service/internal/loaders"

	"gopkg.in/yaml.v3"
)

// RuleResult represents the result of evaluating a rule
type RuleResult struct {
	RuleID            string
	Impact            string
	PassedChecks      int                 // Number of validators that contributed to the score
	TotalChecks       int                 // Total number of validators
	FailedChecks      []string            // Names of validators that had failures
	FailedMetrics     map[string][]string // metric_name -> []validator_names that failed
	PassedMetrics     int                 // Total metrics that passed across all validators
	TotalMetrics      int                 // Total metrics evaluated across all validators
	PassedCardinality int64               // Total cardinality of passed metrics (for weighted scoring)
	TotalCardinality  int64               // Total cardinality of all metrics (for weighted scoring)
	ValidatorStats    []ValidatorStat     // Detailed stats per validator
}

// ValidatorStat tracks pass/fail statistics for a single validator
type ValidatorStat struct {
	Name          string
	PassedMetrics int
	TotalMetrics  int
	PassRate      float64
	UITitle       string // Display title for UI
	UIDescription string // Description for UI
}

// RuleEngine evaluates rules based on declarative definitions
type RuleEngine struct {
	rules             []RuleDefinition
	exclusionList     []ExclusionEntry
	exclusionPatterns []*regexp.Regexp
}

// NewRuleEngine creates a new rule engine from a YAML rules file
func NewRuleEngine(rulesFile string) (*RuleEngine, error) {
	data, err := os.ReadFile(rulesFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read rules file: %w", err)
	}

	var config RulesConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal rules: %w", err)
	}

	// Compile regex patterns for job name matching
	var patterns []*regexp.Regexp
	for i, exclusion := range config.ExclusionList {
		if exclusion.JobNamePattern != "" {
			pattern, err := regexp.Compile(exclusion.JobNamePattern)
			if err != nil {
				return nil, fmt.Errorf("invalid regex pattern in exclusion_list[%d]: %w", i, err)
			}
			patterns = append(patterns, pattern)
		} else {
			patterns = append(patterns, nil)
		}
	}

	return &RuleEngine{
		rules:             config.Rules,
		exclusionList:     config.ExclusionList,
		exclusionPatterns: patterns,
	}, nil
}

// IsJobExcluded checks if a job is completely excluded
func (e *RuleEngine) IsJobExcluded(jobName string) bool {
	for i, exclusion := range e.exclusionList {
		// Check exact job name match
		if exclusion.Job != "" && exclusion.Job == jobName && len(exclusion.Metrics) == 0 {
			return true
		}
		// Check regex pattern match
		if exclusion.JobNamePattern != "" && e.exclusionPatterns[i] != nil {
			if e.exclusionPatterns[i].MatchString(jobName) && len(exclusion.Metrics) == 0 {
				return true
			}
		}
	}
	return false
}

// IsMetricExcluded checks if a specific metric is excluded for a job
func (e *RuleEngine) IsMetricExcluded(jobName, metricName string) bool {
	for i, exclusion := range e.exclusionList {
		matchesJob := false

		// Check if job matches by exact name
		if exclusion.Job != "" && exclusion.Job == jobName {
			matchesJob = true
		}

		// Check if job matches by pattern
		if exclusion.JobNamePattern != "" && e.exclusionPatterns[i] != nil {
			if e.exclusionPatterns[i].MatchString(jobName) {
				matchesJob = true
			}
		}

		if matchesJob {
			// If no metrics specified, entire job is excluded
			if len(exclusion.Metrics) == 0 {
				return true
			}
			// Check if this specific metric is excluded
			for _, excludedMetric := range exclusion.Metrics {
				if excludedMetric == metricName {
					return true
				}
			}
		}
	}
	return false
}

// FilterExcludedMetrics filters out excluded metrics from data sources
func (e *RuleEngine) FilterExcludedMetrics(jobName string, cardinalityData []loaders.CardinalityData, labelsData []loaders.LabelsData) ([]loaders.CardinalityData, []loaders.LabelsData) {
	var filteredCardinality []loaders.CardinalityData
	var filteredLabels []loaders.LabelsData

	// Filter cardinality data
	for _, data := range cardinalityData {
		if !e.IsMetricExcluded(jobName, data.MetricName) {
			filteredCardinality = append(filteredCardinality, data)
		}
	}

	// Filter labels data
	for _, data := range labelsData {
		if !e.IsMetricExcluded(jobName, data.MetricName) {
			filteredLabels = append(filteredLabels, data)
		}
	}

	return filteredCardinality, filteredLabels
}

// EvaluateRules evaluates all rules against the provided data
func (e *RuleEngine) EvaluateRules(dataFiles map[string]string) ([]RuleResult, error) {
	dataSources := make(map[string]interface{})
	for key, file := range dataFiles {
		switch key {
		case "cardinality":
			data, err := loaders.LoadCardinalityReport(file)
			if err != nil {
				return nil, fmt.Errorf("failed to load cardinality data: %w", err)
			}
			dataSources["cardinality"] = data
		case "labels":
			data, err := loaders.LoadLabelsReport(file)
			if err != nil {
				return nil, fmt.Errorf("failed to load labels data: %w", err)
			}
			dataSources["labels"] = data
		}
	}

	return e.evaluateWithDataSources(dataSources)
}

// EvaluateWithData evaluates rules using in-memory data instead of files
func (e *RuleEngine) EvaluateWithData(cardinalityData []loaders.CardinalityData, labelsData []loaders.LabelsData) ([]RuleResult, error) {
	dataSources := make(map[string]interface{})
	dataSources["cardinality"] = cardinalityData
	dataSources["labels"] = labelsData

	return e.evaluateWithDataSources(dataSources)
}

func (e *RuleEngine) evaluateWithDataSources(dataSources map[string]interface{}) ([]RuleResult, error) {
	var results []RuleResult

	for _, rule := range e.rules {
		result, err := e.evaluateRule(rule, dataSources)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate rule %s: %w", rule.RuleID, err)
		}
		results = append(results, result)
	}

	return results, nil
}

// evaluateRule evaluates a single rule
func (e *RuleEngine) evaluateRule(rule RuleDefinition, dataSources map[string]interface{}) (RuleResult, error) {
	result := RuleResult{
		RuleID:            rule.RuleID,
		Impact:            rule.Impact,
		PassedChecks:      0,
		TotalChecks:       len(rule.Validators),
		FailedChecks:      []string{},
		FailedMetrics:     make(map[string][]string),
		PassedMetrics:     0,
		TotalMetrics:      0,
		PassedCardinality: 0,
		TotalCardinality:  0,
		ValidatorStats:    []ValidatorStat{},
	}

	for _, validator := range rule.Validators {
		passedCount, totalCount, failedMetrics, passedCard, totalCard, err := e.evaluateValidatorWithStats(validator, dataSources)
		if err != nil {
			return result, fmt.Errorf("validator %s failed: %w", validator.Name, err)
		}

		passRate := 0.0
		if totalCount > 0 {
			passRate = float64(passedCount) / float64(totalCount)
		}

		result.ValidatorStats = append(result.ValidatorStats, ValidatorStat{
			Name:          validator.Name,
			PassedMetrics: passedCount,
			TotalMetrics:  totalCount,
			PassRate:      passRate,
			UITitle:       validator.UITitle,
			UIDescription: validator.UIDescription,
		})

		result.PassedMetrics += passedCount
		result.TotalMetrics += totalCount
		result.PassedCardinality += passedCard
		result.TotalCardinality += totalCard
		result.PassedChecks++

		if len(failedMetrics) > 0 {
			result.FailedChecks = append(result.FailedChecks, validator.Name)
			for _, metricName := range failedMetrics {
				result.FailedMetrics[metricName] = append(result.FailedMetrics[metricName], validator.Name)
			}
		}
	}

	return result, nil
}

// ValidatorResult contains the results of evaluating a validator
type ValidatorResult struct {
	PassedCount       int
	TotalCount        int
	FailedMetrics     []string
	PassedCardinality int64
	TotalCardinality  int64
}

// evaluateValidatorWithStats evaluates a validator and returns pass/fail statistics
func (e *RuleEngine) evaluateValidatorWithStats(validator ValidatorConfig, dataSources map[string]interface{}) (int, int, []string, int64, int64, error) {
	data := dataSources[validator.DataSource]
	if data == nil {
		return 0, 0, nil, 0, 0, fmt.Errorf("data source %s not found", validator.DataSource)
	}

	switch validator.Type {
	case "cardinality":
		cardinalityData, ok := data.([]loaders.CardinalityData)
		if !ok {
			return 0, 0, nil, 0, 0, fmt.Errorf("invalid data type for %s validator", validator.Type)
		}
		return evaluateMetricsWithCardinality(cardinalityData, validator, e.evaluateCardinalityMetric)
	case "format":
		// Format validator only checks naming patterns, uses labels data source
		labelsData, ok := data.([]loaders.LabelsData)
		if !ok {
			return 0, 0, nil, 0, 0, fmt.Errorf("format validator requires labels data source")
		}
		passed, total, failed, err := evaluateMetrics(labelsData, validator, e.evaluateLabelsMetric)
		return passed, total, failed, 0, 0, err
	case "labels", "label_count":
		labelsData, ok := data.([]loaders.LabelsData)
		if !ok {
			return 0, 0, nil, 0, 0, fmt.Errorf("invalid data type for %s validator", validator.Type)
		}
		passed, total, failed, err := evaluateMetrics(labelsData, validator, e.evaluateLabelsMetric)
		return passed, total, failed, 0, 0, err
	default:
		return 0, 0, nil, 0, 0, fmt.Errorf("unknown validator type: %s", validator.Type)
	}
}

// MetricEvaluator is a function that evaluates a single metric against conditions
type MetricEvaluator[T any] func(metric T, conditions []ConditionConfig, validatorType string) bool

// evaluateMetrics is a generic function that evaluates any metric type
func evaluateMetrics[T any](data []T, validator ValidatorConfig, evaluator MetricEvaluator[T]) (int, int, []string, error) {
	passed := 0
	total := len(data)
	var failedMetrics []string

	for _, metric := range data {
		if evaluator(metric, validator.Conditions, validator.Type) {
			passed++
		} else {
			var metricName string
			switch m := any(metric).(type) {
			case loaders.CardinalityData:
				metricName = m.MetricName
			case loaders.LabelsData:
				metricName = m.MetricName
			}
			failedMetrics = append(failedMetrics, metricName)
		}
	}

	return passed, total, failedMetrics, nil
}

// evaluateMetricsWithCardinality evaluates cardinality metrics and tracks cardinality sums
func evaluateMetricsWithCardinality(data []loaders.CardinalityData, validator ValidatorConfig, evaluator MetricEvaluator[loaders.CardinalityData]) (int, int, []string, int64, int64, error) {
	passed := 0
	total := len(data)
	var failedMetrics []string
	var passedCardinality int64
	var totalCardinality int64

	for _, metric := range data {
		totalCardinality += metric.Count
		if evaluator(metric, validator.Conditions, validator.Type) {
			passed++
			passedCardinality += metric.Count
		} else {
			failedMetrics = append(failedMetrics, metric.MetricName)
		}
	}

	return passed, total, failedMetrics, passedCardinality, totalCardinality, nil
}

// evaluateCardinalityMetric evaluates a cardinality or format metric
func (e *RuleEngine) evaluateCardinalityMetric(metric loaders.CardinalityData, conditions []ConditionConfig, validatorType string) bool {
	for _, condition := range conditions {
		var conditionMet bool
		switch condition.Field {
		case "count":
			conditionMet = e.compareValues(float64(metric.Count), condition.Operator, condition.Value)
		case "metric_name":
			conditionMet = e.compareStrings(metric.MetricName, condition.Operator, condition.Value)
		default:
			return false
		}
		if !conditionMet {
			return false
		}
	}
	return true
}

// evaluateLabelsMetric evaluates a labels or label_count metric
func (e *RuleEngine) evaluateLabelsMetric(metric loaders.LabelsData, conditions []ConditionConfig, validatorType string) bool {
	for _, condition := range conditions {
		var conditionMet bool
		switch condition.Field {
		case "metric_name":
			conditionMet = e.compareStrings(metric.MetricName, condition.Operator, condition.Value)
		case "labels":
			conditionMet = e.evaluateLabelsField(metric.Labels, condition)
		case "label_count":
			conditionMet = e.compareLabelCount(len(metric.Labels), condition)
		default:
			return false
		}
		if !conditionMet {
			return false
		}
	}
	return true
}

// evaluateLabelsField evaluates label field conditions
func (e *RuleEngine) evaluateLabelsField(labels []string, condition ConditionConfig) bool {
	expectedStr, ok := condition.Value.(string)
	if !ok {
		return false
	}

	switch condition.Operator {
	case "not_contains":
		for _, label := range labels {
			if strings.Contains(strings.ToLower(label), strings.ToLower(expectedStr)) {
				return false
			}
		}
		return true
	case "contains":
		for _, label := range labels {
			if strings.Contains(strings.ToLower(label), strings.ToLower(expectedStr)) {
				return true
			}
		}
		return false
	case "matches":
		// For matches operator, ALL labels must match the pattern
		for _, label := range labels {
			if !e.compareStrings(label, condition.Operator, condition.Value) {
				return false
			}
		}
		return true
	default:
		for _, label := range labels {
			if e.compareStrings(label, condition.Operator, condition.Value) {
				return true
			}
		}
		return false
	}
}

// compareLabelCount compares label count against a condition
func (e *RuleEngine) compareLabelCount(labelCount int, condition ConditionConfig) bool {
	intVal, ok := condition.Value.(int)
	if !ok {
		return false
	}

	switch condition.Operator {
	case "lt":
		return labelCount < intVal
	case "lte":
		return labelCount <= intVal
	case "gt":
		return labelCount > intVal
	case "gte":
		return labelCount >= intVal
	case "eq":
		return labelCount == intVal
	default:
		return false
	}
}

// compareValues compares numeric values
func (e *RuleEngine) compareValues(actual float64, operator string, expected interface{}) bool {
	expectedVal, ok := expected.(float64)
	if !ok {
		// Try to convert from int
		if intVal, ok := expected.(int); ok {
			expectedVal = float64(intVal)
		} else {
			return false
		}
	}

	switch operator {
	case "gt":
		return actual > expectedVal
	case "lt":
		return actual < expectedVal
	case "gte":
		return actual >= expectedVal
	case "lte":
		return actual <= expectedVal
	case "eq":
		return actual == expectedVal
	default:
		return false
	}
}

// compareStrings compares string values
func (e *RuleEngine) compareStrings(actual string, operator string, expected interface{}) bool {
	expectedStr, ok := expected.(string)
	if !ok {
		return false
	}

	switch operator {
	case "matches":
		regex, err := regexp.Compile(expectedStr)
		if err != nil {
			return false
		}
		return regex.MatchString(actual)
	case "contains":
		return strings.Contains(strings.ToLower(actual), strings.ToLower(expectedStr))
	case "not_contains":
		return !strings.Contains(strings.ToLower(actual), strings.ToLower(expectedStr))
	case "eq":
		return actual == expectedStr
	default:
		return false
	}
}

// CalculateInstrumentationScore implements the formula from the spec:
// Score = (Σ(Pi × Wi)) / (Σ(Ti × Wi)) × 100
// Rules with cardinality data use cardinality-weighted scoring, others use metric-count scoring
func CalculateInstrumentationScore(results []RuleResult) float64 {
	impactWeights := map[string]float64{
		"Critical":  40.0, // Increased from 40.0 to emphasize cardinality impact
		"Important": 30.0, // Decreased from 30.0
		"Normal":    20.0,
		"Low":       10.0,
	}

	var numerator float64   // Σ(P_i × W_i)
	var denominator float64 // Σ(T_i × W_i)

	for _, result := range results {
		weight := impactWeights[result.Impact]

		// Use cardinality-weighted scoring if the rule has cardinality data
		// Rules using "cardinality" data source will have TotalCardinality > 0
		// Rules using "labels" data source will have TotalCardinality = 0
		if result.TotalCardinality > 0 {
			numerator += float64(result.PassedCardinality) * weight
			denominator += float64(result.TotalCardinality) * weight
		} else {
			numerator += float64(result.PassedMetrics) * weight
			denominator += float64(result.TotalMetrics) * weight
		}
	}

	if denominator == 0 {
		return 0.0
	}

	// Score = (Σ(P_i × W_i) / Σ(T_i × W_i)) × 100
	return (numerator / denominator) * 100
}
