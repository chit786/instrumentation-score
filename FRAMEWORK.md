# Framework Guide: Building Custom Instrumentation Rules

> **A declarative, no-code framework for defining Prometheus metrics quality rules**

This guide explains how to use the Instrumentation Score Service as a flexible framework for implementing your organization's specific metrics governance policies.

## Table of Contents

- [Framework Overview](#framework-overview)
- [Quick Start: Your First Custom Rule](#quick-start-your-first-custom-rule)
- [Framework Architecture](#framework-architecture)
- [Rule Definition Reference](#rule-definition-reference)
- [Real-World Examples](#real-world-examples)
- [Testing Your Rules](#testing-your-rules)
- [Extending the Engine](#extending-the-engine)
- [Best Practices](#best-practices)

---

## Framework Overview

### Built on the Instrumentation Score Specification

This framework implements the [Instrumentation Score specification](https://github.com/instrumentation-score/spec) - an open, community-driven standard for measuring instrumentation quality. All rules and scoring follow the spec's guidelines to ensure:

- ✅ **Standardized Scoring:** Uses spec-compliant weights (Critical: 40, Important: 30, Normal: 20, Low: 10)
- ✅ **Consistent Format:** Rule documentation follows the [spec's rule format](https://github.com/instrumentation-score/spec/tree/main/rules)
- ✅ **Community Alignment:** Rules can be shared and compared across organizations
- ✅ **Vendor Neutral:** Not tied to any specific observability platform

### Declarative Rule Definition

Define instrumentation quality rules using YAML configuration - no code changes required:

```yaml
- rule_id: "PROM-MET-04"
  description: "Counter metrics must use _total suffix"
  impact: "Important"
  validators:
    - name: "counter_suffix_check"
      type: "format"
      conditions:
        - field: "metric_name"
          operator: "matches"
          value: ".*_total$"
```

### Key Capabilities

| Capability | Description |
|------------|-------------|
| **Declarative Rules** | Define rules in YAML without code changes |
| **Hot Reload** | Rules loaded at runtime, no restart required |
| **Flexible Validation** | 4 validator types, 9 operators, unlimited combinations |
| **Spec-Compliant Scoring** | Uses Instrumentation Score standard weights and formula |
| **Multi-Format Output** | HTML, JSON, Text, Prometheus metrics |
| **Extensible** | Clear extension points for custom validators |

### Framework Components

```yaml
rules_config.yaml          # Rule definitions
  ↓
Rule Engine         # Evaluates rules against metrics
  ↓
Score Calculator           # Applies spec-compliant formula
  ↓
Output Formatters          # Generates reports
```

---

## Quick Start: Your First Custom Rule

### Step 1: Define the Rule

Edit `rules_config.yaml`:

```yaml
- rule_id: "PROM-MET-04"
  description: "Metrics must have help text and type annotations"
  impact: "Normal"
  validators:
    - name: "prom_metrics_documentation_check"
      type: "format"
      data_source: "metadata"
      conditions:
        - field: "help_text"
          operator: "not_empty"
```

### Step 2: Document the Rule

Create `rules/PROM-MET-04.md` following the [Instrumentation Score specification format](https://github.com/instrumentation-score/spec/tree/main/rules):

```markdown
# PROM-MET-04: Metric Documentation

**Description:** Prometheus metrics must include help text and type annotations to ensure discoverability and proper usage.

**Rationale:** Undocumented metrics are difficult to understand and use correctly, leading to misinterpretation and incorrect alerting. This follows Prometheus best practices and improves observability quality.

**Target:** Metric metadata

**Criteria:**
- Each metric MUST have a HELP annotation describing its purpose
- Each metric MUST have a TYPE annotation (counter, gauge, histogram, summary)

**Impact:** Normal



✅ **Compliant:**
```prometheus
# HELP http_requests_total Total number of HTTP requests received
# TYPE http_requests_total counter
http_requests_total{method="GET",status="200"} 1234
```

❌ **Non-Compliant:**
```prometheus
http_requests_total{method="GET",status="200"} 1234
```

**References:**
- [Prometheus Metric and Label Naming](https://prometheus.io/docs/practices/naming/)
- [Instrumentation Score Specification](https://github.com/instrumentation-score/spec)
```

> **Note:** Rule documentation should follow the [Instrumentation Score specification format](https://github.com/instrumentation-score/spec/tree/main/rules) with clear Description, Rationale, Target, Criteria, Impact, and Examples sections.

### Step 3: Test Your Rule

```bash
./instrumentation-score evaluate \
  --job-file reports/job_metrics_*/api-service.txt \
  --rules rules_config.yaml \
  --output text
```

**Output:**
```
=== Instrumentation Score Report for Job: api-service ===

Total Metrics: 45
Instrumentation Score: 92.50%

Rule Evaluation Results:
------------------------
Rule PROM-MET-04 (Normal): 40/45 metrics passed (88.9%)
  Failed metrics:
    - http_request_duration_seconds (missing HELP text)
    - cache_hits (missing TYPE annotation)
    - db_connections (missing both)
```

---

## Framework Architecture

### High-Level Overview

```
┌─────────────────────────────────────────────────────────────┐
│                    rules_config.yaml                         │
│  (Declarative Rule Definitions - No Code Required)          │
│                                                              │
│  - rule_id: "PROM-MET-01"                                 │
│    validators:                                               │
│      - type: "format"                                        │
│        conditions: [...]                                     │
└────────────────────┬────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────┐
│              Rule Engine                              │
│  internal/engine/engine.go                       │
│                                                              │
│  • Loads rules from YAML at runtime                          │
│  • Evaluates conditions against metric data                  │
│  • Calculates scores using spec formula                      │
│  • Tracks per-metric pass/fail status                        │
└────────────────────┬────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────┐
│                 Output Formatters                            │
│  internal/formatters/output.go                                   │
│                                                              │
│  • HTML: Interactive reports with drill-down                 │
│  • JSON: Machine-readable for automation                     │
│  • Text: Human-readable terminal output                      │
│  • Prometheus: Metrics for observability platforms           │
└─────────────────────────────────────────────────────────────┘
```

### Data Flow

```
1. Collect Metrics (analyze command)
   ↓
   Prometheus API → Per-Job Files
   (job_metrics_TIMESTAMP/api-service.txt)

2. Load Rules (evaluate command)
   ↓
   rules_config.yaml → Rule Definitions

3. Evaluate Rules
   ↓
   For each job:
     For each rule:
       For each validator:
         For each metric:
           Check conditions → Pass/Fail

4. Calculate Score
   ↓
   Score = (Σ(P_i × W_i) / Σ(T_i × W_i)) × 100

5. Generate Output
   ↓
   HTML | JSON | Text | Prometheus
```

---

## Rule Definition Reference

### Rule Structure

```yaml
- rule_id: "UNIQUE-ID"           # Unique identifier (e.g., PROM-MET-01)
  description: "Human readable"   # What this rule checks
  impact: "Critical"              # Critical | Important | Normal | Low
  validators:                     # List of validators (OR logic)
    - name: "validator_name"      # Unique validator name
      type: "validator_type"      # cardinality | labels | label_count | format
      data_source: "data_source"  # cardinality | labels | metadata
      conditions:                 # List of conditions (AND logic)
        - field: "field_name"     # Field to check
          operator: "operator"    # Comparison operator
          value: "expected_value" # Expected value
```

### Impact Levels (Spec-Compliant Weights)

| Impact | Weight | Use Case | Example |
|--------|--------|----------|---------|
| **Critical** | 40 | Security, compliance, system stability | Cardinality limits, PII in labels |
| **Important** | 30 | Significant quality impact | Naming conventions, label best practices |
| **Normal** | 20 | Standard best practices | Documentation, unit suffixes |
| **Low** | 10 | Nice-to-have improvements | Metric descriptions, help text |

**Impact on Score:**
```
Job with 100 metrics:
- Critical rule fails 20 metrics: Score drops by 8 points (20 × 40 / 100)
- Important rule fails 20 metrics: Score drops by 6 points (20 × 30 / 100)
- Normal rule fails 20 metrics: Score drops by 4 points (20 × 20 / 100)
```

### Validator Types

#### 1. `cardinality` - Check Time Series Count

**Purpose:** Prevent high-cardinality explosions that cause cost and performance issues.

**Data Source:** `cardinality`

**Available Fields:**
- `metric_name`: The metric name (string)
- `count`: Number of unique time series (int64)
- `label_count`: Number of labels on the metric (int)

**Example:**
```yaml
- name: "cardinality_limit"
  type: "cardinality"
  data_source: "cardinality"
  conditions:
    - field: "count"
      operator: "lt"
      value: 10000
```

#### 2. `labels` - Validate Label Names/Values

**Purpose:** Block forbidden labels that cause unbounded cardinality or contain PII.

**Data Source:** `labels`

**Available Fields:**
- `metric_name`: The metric name (string)
- `label_name`: Individual label name (string)
- `label_value`: Individual label value (string)

**Example:**
```yaml
- name: "no_user_id_labels"
  type: "labels"
  data_source: "labels"
  conditions:
    - field: "label_name"
      operator: "not_contains"
      value: "user_id"
```

#### 3. `label_count` - Enforce Label Limits

**Purpose:** Limit the number of labels per metric to prevent exponential cardinality growth.

**Data Source:** `cardinality`

**Available Fields:**
- `metric_name`: The metric name (string)
- `label_count`: Number of labels (int)

**Example:**
```yaml
- name: "max_10_labels"
  type: "label_count"
  data_source: "cardinality"
  conditions:
    - field: "label_count"
      operator: "lte"
      value: 10
```

#### 4. `format` - Validate Naming Patterns

**Purpose:** Enforce naming conventions for consistency and discoverability.

**Data Source:** `cardinality` or `metadata`

**Available Fields:**
- `metric_name`: The metric name (string)
- Any custom fields from metadata

**Example:**
```yaml
- name: "snake_case_check"
  type: "format"
  data_source: "cardinality"
  conditions:
    - field: "metric_name"
      operator: "matches"
      value: "^[a-z][a-z0-9_]*[a-z0-9]$"
```

### Operators

| Operator | Type | Description | Example |
|----------|------|-------------|---------|
| `lt` | Numeric | Less than | `count < 10000` |
| `lte` | Numeric | Less than or equal | `label_count <= 10` |
| `gt` | Numeric | Greater than | `count > 0` |
| `gte` | Numeric | Greater than or equal | `label_count >= 1` |
| `eq` | Numeric/String | Equals | `label_name == "job"` |
| `ne` | Numeric/String | Not equals | `label_name != "instance"` |
| `contains` | String | Contains substring | `metric_name contains "_total"` |
| `not_contains` | String | Does not contain | `label_name not_contains "user_id"` |
| `matches` | String | Regex match | `metric_name matches "^http_.*"` |

### Condition Logic

**Within a Validator (AND Logic):**
```yaml
validators:
  - name: "histogram_check"
    conditions:
      - field: "metric_name"
        operator: "contains"
        value: "_bucket"
      - field: "label_count"
        operator: "gte"
        value: 1  # Must have 'le' label
    # Both conditions must pass
```

**Across Validators (OR Logic):**
```yaml
validators:
  - name: "check_seconds"
    conditions:
      - field: "metric_name"
        operator: "matches"
        value: ".*_seconds$"
  
  - name: "check_bytes"
    conditions:
      - field: "metric_name"
        operator: "matches"
        value: ".*_bytes$"
  # Either validator can pass
```

---

## Real-World Examples

### Example 1: Security - Block PII in Labels

**Problem:** Metrics accidentally include personally identifiable information in labels.

**Solution:**
```yaml
- rule_id: "ORG-SEC-01"
  description: "Metrics must not contain PII in labels"
  impact: "Critical"
  validators:
    - name: "no_email_labels"
      type: "labels"
      data_source: "labels"
      conditions:
        - field: "label_name"
          operator: "not_contains"
          value: "email"
    
    - name: "no_phone_labels"
      type: "labels"
      data_source: "labels"
      conditions:
        - field: "label_name"
          operator: "not_contains"
          value: "phone"
    
    - name: "no_ip_labels"
      type: "labels"
      data_source: "labels"
      conditions:
        - field: "label_name"
          operator: "not_contains"
          value: "ip_address"
    
    - name: "no_ssn_labels"
      type: "labels"
      data_source: "labels"
      conditions:
        - field: "label_name"
          operator: "not_contains"
          value: "ssn"
```

**Impact:** Critical (Weight: 40) - Security and compliance violation

### Example 2: Cost Control - Team-Specific Limits

**Problem:** Different teams have different cardinality budgets.

**Solution:** Create separate rule configs per team:

**`rules_frontend.yaml`:**
```yaml
- rule_id: "TEAM-FRONTEND-COST"
  description: "Frontend metrics limited to 5k series per metric"
  impact: "Critical"
  validators:
    - name: "frontend_cardinality_limit"
      type: "cardinality"
      data_source: "cardinality"
      conditions:
        - field: "count"
          operator: "lt"
          value: 5000
```

**`rules_backend.yaml`:**
```yaml
- rule_id: "TEAM-BACKEND-COST"
  description: "Backend metrics limited to 20k series per metric"
  impact: "Critical"
  validators:
    - name: "backend_cardinality_limit"
      type: "cardinality"
      data_source: "cardinality"
      conditions:
        - field: "count"
          operator: "lt"
          value: 20000
```

**Usage:**
```bash
# Frontend team
./instrumentation-score evaluate \
  --job-dir reports/frontend/ \
  --rules rules_frontend.yaml

# Backend team
./instrumentation-score evaluate \
  --job-dir reports/backend/ \
  --rules rules_backend.yaml
```

### Example 3: Naming - Enforce Company Namespace

**Problem:** Metrics from different teams have inconsistent naming.

**Solution:**
```yaml
- rule_id: "ORG-NAMING-01"
  description: "Metrics must use company namespace prefix"
  impact: "Important"
  validators:
    - name: "company_prefix_check"
      type: "format"
      data_source: "cardinality"
      conditions:
        - field: "metric_name"
          operator: "matches"
          value: "^(acme|http|process|go)_.*"
    # Allows: acme_*, http_*, process_*, go_*
    # Blocks: random_metric_name
```

### Example 4: Best Practices - Counter Suffix

**Problem:** Counter metrics don't follow Prometheus naming conventions.

**Solution:**
```yaml
- rule_id: "PROM-NAMING-01"
  description: "Counter metrics must use _total suffix"
  impact: "Important"
  validators:
    - name: "counter_suffix_check"
      type: "format"
      data_source: "cardinality"
      conditions:
        - field: "metric_name"
          operator: "matches"
          value: ".*(total|count|sum)$"
```

### Example 5: Performance - Histogram Label Requirements

**Problem:** Histograms without proper labels cause query issues.

**Solution:**
```yaml
- rule_id: "PROM-HISTOGRAM-01"
  description: "Histogram metrics must have 'le' label"
  impact: "Important"
  validators:
    - name: "histogram_bucket_check"
      type: "format"
      data_source: "cardinality"
      conditions:
        - field: "metric_name"
          operator: "contains"
          value: "_bucket"
        - field: "label_count"
          operator: "gte"
          value: 1
```

### Example 6: Multi-Condition - Rate Metrics

**Problem:** Rate metrics should end with specific suffixes and have time-based labels.

**Solution:**
```yaml
- rule_id: "PROM-RATE-01"
  description: "Rate metrics must follow conventions"
  impact: "Normal"
  validators:
    - name: "rate_suffix_check"
      type: "format"
      data_source: "cardinality"
      conditions:
        - field: "metric_name"
          operator: "matches"
          value: ".*_(rate|per_second|per_minute)$"
```

---

## Testing Your Rules

### 1. Unit Test with Sample Data

**Create Test Data:**
```bash
# Good metric
echo "test-job|http_requests_total|method,status|150" > test_good.txt

# Bad metric (no _total suffix)
echo "test-job|http_requests|method,status|150" > test_bad.txt
```

**Test Rule:**
```bash
./instrumentation-score evaluate \
  --job-file test_good.txt \
  --rules rules_config.yaml \
  --output text

# Expected: 100% score

./instrumentation-score evaluate \
  --job-file test_bad.txt \
  --rules rules_config.yaml \
  --output text

# Expected: Lower score with failure details
```

### 2. Validate YAML Syntax

```bash
# Install yamllint
pip install yamllint

# Check syntax
yamllint rules_config.yaml

# Expected output:
# ✓ rules_config.yaml
```

### 3. Test Rule Loading

```bash
# Verify rules are loaded correctly
./instrumentation-score evaluate \
  --job-file test.txt \
  --rules rules_config.yaml \
  --output json | jq '.rules[] | {rule_id, impact, validators: .validators | length}'

# Output:
# {
#   "rule_id": "PROM-MET-01",
#   "impact": "Important",
#   "validators": 1
# }
```

### 4. Compare Before/After Scores

```bash
# Baseline score
./instrumentation-score evaluate \
  --job-dir reports/ \
  --rules rules_config.yaml \
  --output baseline.json

# Add new rule to rules_config.yaml

# Re-evaluate
./instrumentation-score evaluate \
  --job-dir reports/ \
  --rules rules_config.yaml \
  --output updated.json

# Compare average scores
echo "Baseline: $(jq '.average_score' baseline.json)"
echo "Updated:  $(jq '.average_score' updated.json)"

# Compare per-job impact
jq -r '.jobs[] | "\(.job_name): \(.instrumentation_score)"' baseline.json > baseline_scores.txt
jq -r '.jobs[] | "\(.job_name): \(.instrumentation_score)"' updated.json > updated_scores.txt
diff baseline_scores.txt updated_scores.txt
```

### 5. Regression Testing

**Create Test Suite:**
```bash
#!/bin/bash
# test_rules.sh

set -e

echo "Testing rules configuration..."

# Test 1: Valid metrics should pass
./instrumentation-score evaluate \
  --job-file testdata/valid_metrics.txt \
  --rules rules_config.yaml \
  --output json > result.json

score=$(jq '.instrumentation_score' result.json)
if (( $(echo "$score >= 90" | bc -l) )); then
  echo "✓ Test 1 passed: Valid metrics scored $score%"
else
  echo "✗ Test 1 failed: Expected >= 90%, got $score%"
  exit 1
fi

# Test 2: Invalid metrics should fail
./instrumentation-score evaluate \
  --job-file testdata/invalid_metrics.txt \
  --rules rules_config.yaml \
  --output json > result.json

score=$(jq '.instrumentation_score' result.json)
if (( $(echo "$score < 50" | bc -l) )); then
  echo "✓ Test 2 passed: Invalid metrics scored $score%"
else
  echo "✗ Test 2 failed: Expected < 50%, got $score%"
  exit 1
fi

echo "All tests passed!"
```

---

## Extending the Engine

Most use cases can be handled with YAML configuration. Consider code changes only if you need:

### 1. Adding New Data Sources

**Current Data Sources:**
- `cardinality`: Metric name, cardinality count, label count
- `labels`: Metric name, label names, label values
- `metadata`: Metric name, help text, type annotations

**When to Add:**
- Need to check metric values (not just metadata)
- Need to query external systems (service registry, etc.)
- Need to validate against historical data

**How to Add:**

**Step 1:** Update `cmd/analyze.go` to collect new data:
```go
// Fetch metric values
func getMetricValues(metricName string) ([]float64, error) {
    query := fmt.Sprintf("%s[5m]", metricName)
    result, err := queryPrometheus(query)
    // ... parse and return values
}
```

**Step 2:** Update file format in `internal/loaders/reports.go`:
```go
// New data structure
type MetricValueData struct {
    MetricName string
    Values     []float64
    Min        float64
    Max        float64
    Avg        float64
}

// Parser
func LoadMetricValues(filePath string) ([]MetricValueData, error) {
    // ... parse new file format
}
```

**Step 3:** Update `internal/engine/rule_definition.go`:
```go
// Add new data source option
const (
    DataSourceCardinality = "cardinality"
    DataSourceLabels      = "labels"
    DataSourceMetadata    = "metadata"
    DataSourceValues      = "values"  // New
)
```

### 2. Adding New Validator Types

**Current Validator Types:**
- `cardinality`: Check time series count
- `labels`: Validate label names/values
- `label_count`: Enforce label limits
- `format`: Validate naming patterns

**When to Add:**
- Need complex validation logic not expressible in conditions
- Need to validate relationships between multiple metrics
- Need to perform calculations on metric data

**How to Add:**

**Step 1:** Add validator type in `internal/engine/engine.go`:
```go
func (e *DeclarativeEngine) evaluateValidatorWithStats(
    validator ValidatorConfig,
    cardData []CardinalityData,
    labelsData []LabelsData,
) (passedCount, totalCount int, failedMetrics []string, err error) {
    
    switch validator.Type {
    case "cardinality":
        return e.evaluateCardinalityValidatorWithStats(validator, cardData)
    case "labels":
        return e.evaluateLabelsValidatorWithStats(validator, labelsData)
    case "label_count":
        return e.evaluateLabelCountValidatorWithStats(validator, cardData)
    case "format":
        return e.evaluateFormatValidatorWithStats(validator, cardData)
    case "value_range":  // New validator type
        return e.evaluateValueRangeValidatorWithStats(validator, cardData)
    default:
        return 0, 0, nil, fmt.Errorf("unknown validator type: %s", validator.Type)
    }
}

// Implement new validator
func (e *DeclarativeEngine) evaluateValueRangeValidatorWithStats(
    validator ValidatorConfig,
    cardData []CardinalityData,
) (passedCount, totalCount int, failedMetrics []string, err error) {
    
    for _, metric := range cardData {
        totalCount++
        
        // Your custom validation logic here
        if validateValueRange(metric) {
            passedCount++
        } else {
            failedMetrics = append(failedMetrics, metric.MetricName)
        }
    }
    
    return passedCount, totalCount, failedMetrics, nil
}
```

**Step 2:** Use in `rules_config.yaml`:
```yaml
- rule_id: "PROM-MET-05"
  description: "Metric values must be within expected range"
  impact: "Normal"
  validators:
    - name: "value_range_check"
      type: "value_range"  # New type
      data_source: "values"
      conditions:
        - field: "max_value"
          operator: "lt"
          value: 1000000
```

### 3. Adding New Operators

**Current Operators:**
- Numeric: `lt`, `lte`, `gt`, `gte`, `eq`, `ne`
- String: `contains`, `not_contains`, `matches`

**When to Add:**
- Need specialized string operations (starts_with, ends_with, etc.)
- Need mathematical operations (modulo, power, etc.)
- Need list operations (in, not_in, etc.)

**How to Add:**

Update `evaluateCondition()` in `internal/engine/engine.go`:
```go
func evaluateCondition(condition ConditionConfig, fieldValue interface{}) (bool, error) {
    switch condition.Operator {
    case "lt", "lte", "gt", "gte", "eq", "ne":
        // ... existing numeric operators
    
    case "contains", "not_contains", "matches":
        // ... existing string operators
    
    case "starts_with":  // New operator
        strValue, ok := fieldValue.(string)
        if !ok {
            return false, fmt.Errorf("starts_with requires string field")
        }
        expectedPrefix := fmt.Sprintf("%v", condition.Value)
        return strings.HasPrefix(strValue, expectedPrefix), nil
    
    case "in":  // New operator for list membership
        strValue := fmt.Sprintf("%v", fieldValue)
        allowedValues := strings.Split(fmt.Sprintf("%v", condition.Value), ",")
        for _, allowed := range allowedValues {
            if strings.TrimSpace(allowed) == strValue {
                return true, nil
            }
        }
        return false, nil
    
    default:
        return false, fmt.Errorf("unknown operator: %s", condition.Operator)
    }
}
```

**Usage:**
```yaml
validators:
  - name: "namespace_check"
    type: "format"
    conditions:
      - field: "metric_name"
        operator: "starts_with"  # New operator
        value: "acme_"
  
  - name: "allowed_types"
    type: "format"
    conditions:
      - field: "metric_type"
        operator: "in"  # New operator
        value: "counter,gauge,histogram"
```

---

## Best Practices

### 1. Start Simple, Iterate

**Phase 1: Baseline (Week 1)**
```yaml
# Start with lenient thresholds
- rule_id: "PROM-MET-02"
  validators:
    - name: "cardinality_check"
      conditions:
        - field: "count"
          operator: "lt"
          value: 50000  # Lenient
```

**Phase 2: Tighten (Month 1)**
```yaml
# Gradually reduce threshold
- rule_id: "PROM-MET-02"
  validators:
    - name: "cardinality_check"
      conditions:
        - field: "count"
          operator: "lt"
          value: 20000  # Tighter
```

**Phase 3: Target (Month 3)**
```yaml
# Reach target threshold
- rule_id: "PROM-MET-02"
  validators:
    - name: "cardinality_check"
      conditions:
        - field: "count"
          operator: "lt"
          value: 10000  # Target
```

### 2. Use Impact Levels Strategically

**Critical (40):** Reserve for issues that:
- Directly impact costs (cardinality)
- Cause system failures (unbounded growth)
- Violate security/compliance (PII in labels)

**Important (30):** Use for issues that:
- Significantly impact quality (naming conventions)
- Prevent future problems (label best practices)
- Affect team productivity (discoverability)

**Normal (20):** Use for issues that:
- Improve maintainability (documentation)
- Follow best practices (unit suffixes)
- Enhance usability (help text)

**Low (10):** Use for issues that:
- Nice-to-have improvements (descriptions)
- Cosmetic issues (formatting)
- Optional enhancements (examples)

### 3. Document Your Rules

**Follow the [Instrumentation Score Specification Format](https://github.com/instrumentation-score/spec/tree/main/rules):**

```markdown
# RULE-ID: Rule Title

**Description:** Brief description of what this rule checks.

**Rationale:** Why this rule exists and what problems it prevents. Include impact on observability quality, cost, or performance.

**Target:** What aspect is being validated (e.g., Metric, Label, Resource)

**Criteria:**
- Specific condition 1 that MUST be met
- Specific condition 2 that MUST be met

**Impact:** Critical | Important | Normal | Low

**Examples:**

✅ **Compliant:**
```
[example of passing metric with explanation]
```

❌ **Non-Compliant:**
```
[example of failing metric with explanation]
```

**References:**
- [Link to relevant Prometheus documentation]
- [Link to Instrumentation Score spec]
```

> **Spec Compliance:** This format aligns with the [community standard](https://github.com/instrumentation-score/spec/tree/main/rules), making your rules shareable and comparable across organizations.

### 4. Version Control Your Rules

```bash
# Track rule changes
git add rules_config.yaml rules/PROM-MET-*.md
git commit -m "Add PROM-MET-04: Metric documentation rule"

# Tag releases
git tag -a v1.2.0 -m "Add documentation rules"
git push origin v1.2.0
```

### 5. Test in Staging First

```bash
# Test new rules against staging data
./instrumentation-score evaluate \
  --job-dir reports/staging/ \
  --rules rules_config_new.yaml \
  --output staging-test.json

# Review impact
jq '.jobs[] | select(.instrumentation_score < 80) | {job_name, score: .instrumentation_score}' staging-test.json

# If acceptable, promote to production
cp rules_config_new.yaml rules_config.yaml
```

### 6. Monitor Rule Impact

```bash
# Track score trends over time
./instrumentation-score evaluate \
  --job-dir reports/$(date +%Y%m%d)/ \
  --rules rules_config.yaml \
  --output daily-scores.json

# Append to historical data
jq '{date: .timestamp, avg_score: .average_score}' daily-scores.json >> score_history.jsonl

# Visualize trends
cat score_history.jsonl | jq -r '[.date, .avg_score] | @csv' > scores.csv
```

### 7. Create Rule Templates

**Template for Security Rules:**
```yaml
- rule_id: "ORG-SEC-XX"
  description: "Security: [What is being protected]"
  impact: "Critical"
  validators:
    - name: "[validator_name]"
      type: "labels"
      data_source: "labels"
      conditions:
        - field: "label_name"
          operator: "not_contains"
          value: "[forbidden_label]"
```

**Template for Cost Rules:**
```yaml
- rule_id: "ORG-COST-XX"
  description: "Cost Control: [What is being limited]"
  impact: "Critical"
  validators:
    - name: "[validator_name]"
      type: "cardinality"
      data_source: "cardinality"
      conditions:
        - field: "count"
          operator: "lt"
          value: [threshold]
```

---

## Summary

### Framework Capabilities

✅ **Declarative:** Define rules in YAML, no code changes required  
✅ **Flexible:** 4 validator types, 9 operators, unlimited combinations  
✅ **Spec-Compliant:** Uses Instrumentation Score standard weights and formula  
✅ **Extensible:** Clear extension points for custom validators  
✅ **Testable:** Independent rule testing with sample data  
✅ **Production-Ready:** Used to score 14,000+ metrics across 1,149 jobs  

### Getting Started Checklist

- [ ] Read the [main README](README.md) for installation and basic usage
- [ ] Review existing rules in `rules_config.yaml`
- [ ] Define your first custom rule
- [ ] Test with sample data
- [ ] Document your rule in `rules/` directory
- [ ] Deploy to staging environment
- [ ] Monitor impact and iterate
- [ ] Share rules with your team

### Resources

- **Main Documentation:** [README.md](README.md)
- **Rule Specification:** [Instrumentation Score Spec](https://github.com/instrumentation-score/spec)
- **Example Rules:** `rules_config.yaml`
- **Rule Documentation:** `rules/` directory

---

**Questions or need help?** Open an issue or discussion in the repository!

