package formatters

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"os"
	"strings"

	"instrumentation-score/internal/engine"
	"instrumentation-score/web"

	"gopkg.in/yaml.v3"
)

// OutputData represents the complete evaluation output
type OutputData struct {
	ServiceName string              `json:"service_name"`
	Score       float64             `json:"score"`
	Category    string              `json:"category"`
	Results     []engine.RuleResult `json:"rule_results"`
}

// PrometheusMetrics outputs results in Prometheus format
func PrometheusMetrics(serviceName string, score float64, results []engine.RuleResult) {
	fmt.Printf("# HELP instrumentation_score Overall instrumentation quality score (0-100)\n")
	fmt.Printf("# TYPE instrumentation_score gauge\n")
	fmt.Printf("instrumentation_score{service_name=\"%s\"} %.1f\n", serviceName, score)

	fmt.Printf("\n# HELP instrumentation_rule_checks_total Total number of rule checks\n")
	fmt.Printf("# TYPE instrumentation_rule_checks_total counter\n")
	for _, result := range results {
		fmt.Printf("instrumentation_rule_checks_total{service_name=\"%s\",rule_id=\"%s\",impact=\"%s\"} %d\n",
			serviceName, result.RuleID, result.Impact, result.TotalChecks)
	}

	fmt.Printf("\n# HELP instrumentation_rule_failures_total Total number of rule failures\n")
	fmt.Printf("# TYPE instrumentation_rule_failures_total counter\n")
	for _, result := range results {
		failures := result.TotalChecks - result.PassedChecks
		fmt.Printf("instrumentation_rule_failures_total{service_name=\"%s\",rule_id=\"%s\",impact=\"%s\"} %d\n",
			serviceName, result.RuleID, result.Impact, failures)
	}
}

// JobScoreData represents minimal job score data for Prometheus output
type JobScoreData struct {
	JobName          string
	TotalMetrics     int
	TotalCardinality int64
	EstimatedCost    float64
	Score            float64
	RuleResults      []engine.RuleResult
}

// PrometheusMetricsWithSLO outputs per-job instrumentation score metrics for Cortex.io SLO tracking
// These metrics can be used in Cortex.io Scorecards with PromQL queries to define SLOs
// Example Cortex.io SLO configuration:
//
//	errorQuery: 100 - instrumentation_quality_score{job="api-service"}
//	totalQuery: 100
//	slo: 75.0  # Target: Score should be >= 75%
func PrometheusMetricsWithSLO(jobs []JobScoreData) string {
	var output strings.Builder

	// Instrumentation Quality Score (0-100 scale)
	// Primary metric for SLO tracking in Cortex.io
	output.WriteString("# HELP instrumentation_quality_score Instrumentation quality score per job (0-100)\n")
	output.WriteString("# TYPE instrumentation_quality_score gauge\n")
	for _, job := range jobs {
		output.WriteString(fmt.Sprintf("instrumentation_quality_score{job=\"%s\"} %.2f\n", job.JobName, job.Score))
	}
	output.WriteString("\n")

	return output.String()
}

// JSON outputs results in JSON format
func JSON(serviceName string, score float64, results []engine.RuleResult) {
	category := getScoreCategory(score)

	output := OutputData{
		ServiceName: serviceName,
		Score:       score,
		Category:    category,
		Results:     results,
	}

	jsonData, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		log.Fatalf("Error marshaling JSON: %v", err)
	}

	fmt.Println(string(jsonData))
}

// Text outputs results in human-readable text format
func Text(serviceName string, score float64, results []engine.RuleResult) {
	category := getScoreCategory(score)

	fmt.Printf("Instrumentation Score Report for %s\n", serviceName)
	fmt.Printf("=====================================\n\n")
	fmt.Printf("Overall Score: %.1f/100 (%s)\n\n", score, category)

	fmt.Printf("Rule Evaluation Results:\n")
	fmt.Printf("------------------------\n")

	for _, result := range results {
		passRate := float64(result.PassedMetrics) / float64(result.TotalMetrics) * 100
		fmt.Printf("Rule %s (%s): %d/%d metrics passed (%.1f%%)\n",
			result.RuleID, result.Impact, result.PassedMetrics, result.TotalMetrics, passRate)

		if len(result.FailedChecks) > 0 {
			fmt.Printf("  Failed validators: %v\n", result.FailedChecks)
		}
		fmt.Println()
	}
}

// getScoreCategory returns the category based on score according to the spec
func getScoreCategory(score float64) string {
	switch {
	case score >= 90:
		return "Excellent"
	case score >= 75:
		return "Good"
	case score >= 50:
		return "Needs Improvement"
	default:
		return "Poor"
	}
}

// JobMetricDetail represents detailed metric information for HTML output
type JobMetricDetail struct {
	MetricName       string
	Labels           string
	Cardinality      string
	Status           string
	FailedRules      []string
	LabelCardinality string // JSON string of label->cardinality map
}

// MultiJobHTMLData represents data for multi-job HTML reports
type MultiJobHTMLData struct {
	Jobs             []JobHTMLData
	TotalJobs        int
	AverageScore     float64
	TotalCost        float64
	TotalCardinality int64
	ShowCost         bool
	Timestamp        string
	RulesConfigJSON  template.JS
	CSS              template.CSS
	JS               template.JS
}

// JobHTMLData represents a single job's data for HTML output
type JobHTMLData struct {
	JobName          string
	Score            float64
	ScoreInt         int
	Category         string
	StatusClass      string
	Results          []engine.RuleResult
	Metrics          []JobMetricDetail
	TotalMetrics     int
	TotalCardinality int64
	EstimatedCost    float64
	ShowCost         bool
}

// HTMLMultiJob outputs results for multiple jobs in a beautiful HTML report format
func HTMLMultiJob(jobsData []JobHTMLData, avgScore float64, outputFile string) {
	HTMLMultiJobWithCost(jobsData, avgScore, 0, 0, false, outputFile, "")
}

// HTMLMultiJobWithCost outputs results for multiple jobs with cost information
func HTMLMultiJobWithCost(jobsData []JobHTMLData, avgScore float64, totalCost float64, totalCardinality int64, showCost bool, outputFile string, rulesConfigPath string) {
	rulesConfigJSON := template.JS("{}")
	if rulesConfigPath != "" {
		if rulesData, err := os.ReadFile(rulesConfigPath); err == nil {
			var rules interface{}
			if err := yaml.Unmarshal(rulesData, &rules); err == nil {
				if jsonData, err := json.Marshal(rules); err == nil {
					rulesConfigJSON = template.JS(jsonData)
				}
			}
		}
	}

	data := MultiJobHTMLData{
		Jobs:             jobsData,
		TotalJobs:        len(jobsData),
		AverageScore:     avgScore,
		TotalCost:        totalCost,
		TotalCardinality: totalCardinality,
		ShowCost:         showCost,
		Timestamp:        fmt.Sprintf("%v", os.Getenv("TIMESTAMP")),
		RulesConfigJSON:  rulesConfigJSON,
		CSS:              template.CSS(web.CSS),
		JS:               template.JS(web.JS),
	}

	tmpl := template.Must(template.New("multi-job-report.html").Funcs(getTemplateFuncs()).ParseFS(web.Templates, "templates/multi-job-report.html"))

	var output *os.File
	var err error

	if outputFile != "" {
		output, err = os.OpenFile(outputFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
		if err != nil {
			log.Fatalf("Error creating HTML file: %v", err)
		}
		defer output.Close()
	} else {
		output = os.Stdout
	}

	err = tmpl.Execute(output, data)
	if err != nil {
		log.Fatalf("Error executing template: %v", err)
	}

	if outputFile != "" {
		fmt.Printf("HTML report generated: %s\n", outputFile)
	}
}

// HTML outputs results in a beautiful HTML report format
func HTML(serviceName string, score float64, results []engine.RuleResult, outputFile string) {
	category := getScoreCategory(score)

	data := struct {
		ServiceName string
		Score       float64
		ScoreInt    int
		Category    string
		StatusClass string
		Results     []engine.RuleResult
	}{
		ServiceName: serviceName,
		Score:       score,
		ScoreInt:    int(score),
		Category:    category,
		StatusClass: getStatusClass(score),
		Results:     results,
	}

	tmpl := template.Must(template.New("single-job-report.html").Funcs(getTemplateFuncs()).ParseFS(web.Templates, "templates/single-job-report.html"))

	var output *os.File
	var err error

	if outputFile != "" {
		output, err = os.OpenFile(outputFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
		if err != nil {
			log.Fatalf("Error creating HTML file: %v", err)
		}
		defer output.Close()
	} else {
		output = os.Stdout
	}

	err = tmpl.Execute(output, data)
	if err != nil {
		log.Fatalf("Error executing template: %v", err)
	}

	if outputFile != "" {
		fmt.Printf("HTML report generated: %s\n", outputFile)
	}
}

func getStatusClass(score float64) string {
	switch {
	case score >= 90:
		return "status-excellent"
	case score >= 75:
		return "status-good"
	case score >= 50:
		return "status-warning"
	default:
		return "status-poor"
	}
}

func getTemplateFuncs() template.FuncMap {
	return template.FuncMap{
		"passRate": func(passed, total int) float64 {
			if total == 0 {
				return 0
			}
			return float64(passed) / float64(total) * 100
		},
		"lower": func(s string) string {
			return strings.ToLower(s)
		},
		"getImpactClass": func(impact string) string {
			switch impact {
			case "Critical":
				return "impact-critical"
			case "Important":
				return "impact-important"
			case "Moderate":
				return "impact-moderate"
			default:
				return "impact-low"
			}
		},
		"getRuleStatus": func(passed, total int) string {
			if passed == total {
				return "✓ Passed"
			}
			return "⚠ Needs Attention"
		},
		"getRuleStatusClass": func(passed, total int) string {
			if passed == total {
				return "status-passed"
			}
			return "status-failed"
		},
	}
}
