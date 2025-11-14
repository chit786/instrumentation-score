# Instrumentation Score Service


> **Evaluate and improve your Prometheus metrics quality with automated scoring**

A production-ready tool that analyzes Prometheus metrics against [instrumentation best practices](https://github.com/instrumentation-score/spec), providing actionable insights to improve observability quality, reduce costs, and maintain healthy metrics.

[![CI](https://github.com/instrumentation-score-service/instrumentation-score/workflows/CI/badge.svg)](https://github.com/instrumentation-score-service/instrumentation-score/actions)
[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Spec Compliant](https://img.shields.io/badge/Spec-Compliant-green)](https://github.com/instrumentation-score/spec)

**📚 Documentation:** [Framework Guide](FRAMEWORK.md)

## About This Project

### Inspired by the Instrumentation Score Specification

This project implements the [Instrumentation Score specification](https://github.com/instrumentation-score/spec/blob/main/specification.md) - an open standard for measuring instrumentation quality created by [OllyGarden](https://olly.garden).

**Key Difference:**
- **Original Spec:** Designed for [OpenTelemetry (OTLP)](https://opentelemetry.io/) traces, metrics, and logs
- **This Project:** Extends the same scoring methodology to **Prometheus-compatible metrics**

By adapting the spec's proven scoring formula and rule-based approach, this tool brings standardized instrumentation quality measurement to the Prometheus ecosystem, enabling teams using Prometheus/Grafana to:
- Apply the same quality standards as OpenTelemetry users
- Benchmark instrumentation quality across different observability stacks
- Use a vendor-neutral, community-driven scoring methodology

### Why Prometheus Extension?

While OpenTelemetry is the future of observability, **Prometheus remains the dominant metrics solution** in cloud-native environments:
- Native Kubernetes integration
- Massive ecosystem adoption
- Battle-tested in production
- Grafana Cloud compatibility

This project bridges the gap, allowing Prometheus users to benefit from the Instrumentation Score standard without migrating to OTLP.

---

## Why Use This Tool?

**Problems it solves:**
- 🔍 **High cardinality metrics** consuming excessive storage and slowing queries
- 🏷️ **Inconsistent naming** making metrics hard to discover and use
- 💰 **Unpredictable costs** from unbounded metric growth
- 📊 **Poor instrumentation quality** hindering effective observability

**What you get:**
- ✅ Automated quality scoring (0-100) per service/job
- ✅ Per-metric analysis with specific failure reasons
- ✅ Cost estimation for metrics storage
- ✅ Beautiful HTML reports with actionable recommendations
- ✅ CI/CD integration ready

---

## 🚀 Quick Start

### Installation Options

#### Option 1: Download Pre-built Binary (Recommended)
Download the latest release for your platform from the [releases page](https://github.com/instrumentation-score-service/instrumentation-score/releases).

```bash
# Linux
wget https://github.com/instrumentation-score-service/instrumentation-score/releases/latest/download/instrumentation-score-service-linux-amd64.tar.gz
tar -xzf instrumentation-score-service-linux-amd64.tar.gz
chmod +x instrumentation-score-service-linux-amd64
sudo mv instrumentation-score-service-linux-amd64 /usr/local/bin/instrumentation-score-service

# macOS (Intel)
wget https://github.com/instrumentation-score-service/instrumentation-score/releases/latest/download/instrumentation-score-service-darwin-amd64.tar.gz
tar -xzf instrumentation-score-service-darwin-amd64.tar.gz
chmod +x instrumentation-score-service-darwin-amd64
sudo mv instrumentation-score-service-darwin-amd64 /usr/local/bin/instrumentation-score-service

# macOS (Apple Silicon)
wget https://github.com/instrumentation-score-service/instrumentation-score/releases/latest/download/instrumentation-score-service-darwin-arm64.tar.gz
tar -xzf instrumentation-score-service-darwin-arm64.tar.gz
chmod +x instrumentation-score-service-darwin-arm64
sudo mv instrumentation-score-service-darwin-arm64 /usr/local/bin/instrumentation-score-service
```

#### Option 2: Docker
```bash
docker pull ghcr.io/instrumentation-score-service/instrumentation-score:latest
```

#### Option 3: Build from Source
```bash
git clone https://github.com/instrumentation-score-service/instrumentation-score.git
cd instrumentation-score
go build -o instrumentation-score-service .
```

### 1. Verify Installation
```bash
instrumentation-score-service --version
```

### 2. Analyze Your Metrics
```bash
# For authenticated Prometheus (e.g., Grafana Cloud, Grafana Enterprise)
export login="user:api_key"
export url="https://your-prometheus-instance.com/api/prom"

# For local/unauthenticated Prometheus
export url="http://localhost:9090"

# Collect metrics data grouped by job
./instrumentation-score-service analyze \
  --output-dir ./reports
```

This creates per-job files in `reports/job_metrics_TIMESTAMP/`:
```
reports/job_metrics_20251102_160000/
├── api-service.txt
├── web-gateway.txt
├── database-exporter.txt
└── ... (one file per job)
```

### 3. Evaluate & Get Scores

**Single Job:**
```bash
./instrumentation-score-service evaluate-single-job \
  --job-file reports/job_metrics_*/api-service.txt \
  --rules rules_config.yaml \
  --output text
```

**All Jobs with Costs:**
```bash
./instrumentation-score-service evaluate-all-jobs \
  --job-dir reports/job_metrics_20251102_160000/ \
  --rules rules_config.yaml \
  --html-file report.html \
  --show-costs \
  --cost-unit-price 0.00615
```

### 4. View Results

**Terminal Output:**
```
=== Instrumentation Score Report for Job: api-service ===

Total Metrics: 45
Instrumentation Score: 97.63%

Rule Evaluation Results:
------------------------
Rule PROM-MET-01 (Important): 1/1 checks passed (100.0%)
Rule PROM-MET-02 (Critical): 3/3 checks passed (100.0%)
  Failed checks: [prom_metrics_label_count_check]
```

**HTML Report:**
- 📊 Interactive dashboard with searchable job list
- 💰 Cost breakdown per job
- 🎯 Jobs sorted by score (worst first)
- 📈 Per-metric details with failure reasons
- 💡 Actionable recommendations

### 5. Enable Shell Autocomplete (Optional)

Get tab completion for commands and flags:

**Bash:**
```bash
# Load for current session
source <(./instrumentation-score-service completion bash)

# Install permanently (Linux)
./instrumentation-score-service completion bash | sudo tee /etc/bash_completion.d/instrumentation-score-service

# Install permanently (macOS with Homebrew)
./instrumentation-score-service completion bash > $(brew --prefix)/etc/bash_completion.d/instrumentation-score-service
```

**Zsh:**
```bash
# Enable completion system (if not already enabled)
echo "autoload -U compinit; compinit" >> ~/.zshrc

# Install completion
./instrumentation-score-service completion zsh > "${fpath[1]}/_instrumentation-score-service"

# Restart your shell or reload
source ~/.zshrc
```

**Fish:**
```bash
# Load for current session
./instrumentation-score-service completion fish | source

# Install permanently
./instrumentation-score-service completion fish > ~/.config/fish/completions/instrumentation-score-service.fish
```

**PowerShell:**
```powershell
# Load for current session
instrumentation-score-service completion powershell | Out-String | Invoke-Expression

# Install permanently (add to PowerShell profile)
instrumentation-score-service completion powershell > instrumentation-score-service.ps1
```

**What you get:**
- ✅ Command completion (`analyze`, `evaluate`, `completion`)
- ✅ Flag completion (`--output-dir`, `--s3-upload`, etc.)
- ✅ Flag value hints (e.g., `--output` suggests `html,json,prometheus`)
- ✅ Context-aware suggestions

---

## 🗄️ S3 Integration Workflow

Store and retrieve metrics reports in S3 for centralized storage and CI/CD integration.

### AWS Authentication

The service supports the **full AWS credential chain** for authentication. Choose the method that fits your environment:

#### 1. Environment Variables (Recommended for CI/CD)
```bash
export AWS_ACCESS_KEY_ID=your-access-key
export AWS_SECRET_ACCESS_KEY=your-secret-key
export AWS_REGION=eu-west-1
```

#### 2. AWS Profile (Recommended for Local Development)
```bash
# Use a specific profile from ~/.aws/credentials
export AWS_PROFILE=production

# Or set it inline
AWS_PROFILE=production ./instrumentation-score-service analyze --s3-upload
```

**~/.aws/credentials example:**
```ini
[default]
aws_access_key_id = AKIA...
aws_secret_access_key = ...

[production]
aws_access_key_id = AKIA...
aws_secret_access_key = ...
region = eu-west-1

[staging]
aws_access_key_id = AKIA...
aws_secret_access_key = ...
region = eu-west-1
```

#### 3. IAM Role (Recommended for AWS Infrastructure)
No credentials needed when running on AWS infrastructure with IAM roles:
- **EC2**: Instance profile
- **ECS/Fargate**: Task role
- **Lambda**: Execution role
- **EKS**: IRSA (IAM Roles for Service Accounts)

```bash
# No AWS credentials needed - automatic!
./instrumentation-score-service analyze --s3-upload
```

#### 4. AWS SSO / Web Identity Token
Automatically supported for Kubernetes workloads with IRSA or AWS SSO sessions.

**Authentication Priority Order:**
1. Environment variables (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`)
2. AWS profile (`AWS_PROFILE` or `[default]` in `~/.aws/credentials`)
3. IAM role (EC2, ECS, Lambda, EKS)
4. Web identity token (Kubernetes IRSA)

### Complete Workflow Example

**Step 1: Analyze and Upload to S3**
```bash
# Option A: Using environment variables
export AWS_ACCESS_KEY_ID=your-access-key
export AWS_SECRET_ACCESS_KEY=your-secret-key
export AWS_REGION=eu-west-1

# Option B: Using AWS profile
export AWS_PROFILE=production

# Option C: Using IAM role (no credentials needed on AWS infrastructure)

# Configure S3 storage
export S3_BUCKET=my-metrics-bucket
export S3_PREFIX=instrumentation-reports

# Analyze metrics and upload to S3
./instrumentation-score-service analyze \
  --output-dir ./reports \
  --s3-upload
```

Output:
```
Starting Prometheus metrics analysis...
Generated per-job files in reports/job_metrics_20251102_160000/

Uploading reports to S3...
Uploaded 45 job metric files to s3://my-metrics-bucket/instrumentation-reports/job_metrics_20251102_160000/
Uploaded error file to s3://my-metrics-bucket/instrumentation-reports/metrics_errors_20251102_160000.txt

S3 Location: s3://my-metrics-bucket/instrumentation-reports/job_metrics_20251102_160000/

Analysis complete!
```

**Step 2: Download from S3 and Evaluate (with Upload)**
```bash
# Download, evaluate, and upload results back to S3
./instrumentation-score-service evaluate \
  --s3-source \
  --s3-prefix instrumentation-reports/job_metrics_20251102_160000 \
  --output html,json \
  --html-file dashboard.html \
  --json-file report.json \
  --show-costs \
  --cost-unit-price 0.00615 \
  --s3-upload \
  --s3-run-id prod-20251102
```

**Complete S3 Structure:**
```
s3://my-metrics-bucket/instrumentation-reports/
├── job_metrics_20251102_160000/        # Raw metrics data (from analyze)
│   ├── job1.txt
│   ├── job2.txt
│   └── ...
├── metrics_errors_20251102_160000.txt  # Error log (from analyze)
└── evaluations/                        # Evaluation results
    └── prod-20251102/                  # Run-specific results
        ├── dashboard.html              # Interactive HTML report
        ├── report.json                 # JSON results
        └── manifest.json               # Run metadata & traceability
```
## 📊 How It Works

### 1. Data Collection (`analyze`)
Fetches metrics from Grafana Cloud Prometheus and groups them by `job` label:

```
Job: api-service
├── Metric: http_requests_total
│   ├── Labels: [method, status, endpoint]
│   └── Cardinality: 120
├── Metric: http_request_duration_seconds
│   ├── Labels: [method, endpoint, le]
│   └── Cardinality: 3660
└── ...
```

### 2. Rule Evaluation
Applies configurable rules defined in `rules_config.yaml`:

**Example Rule:**
```yaml
- rule_id: "PROM-MET-02"
  description: "Prometheus metric labels must maintain bounded cardinality"
  impact: "Critical"  # Weight: 40
  validators:
    - name: "prom_metrics_cardinality_check"
  type: "cardinality"
  conditions:
    - field: "count"
      operator: "lt"
      value: 10000
    
    - name: "prom_metrics_label_count_check"
      type: "label_count"
      conditions:
        - field: "label_count"
          operator: "lte"
          value: 10
```

### 3. Score Calculation

Uses the [Instrumentation Score specification](https://github.com/instrumentation-score/spec/blob/main/specification.md) formula:

```
Score = (Σ(P_i × W_i) / Σ(T_i × W_i)) × 100

Where:
  P_i = Metrics passed for impact level i
  T_i = Total metrics for impact level i
  W_i = Weight for impact level i
```

**Impact Weights (Spec-Compliant):**

As defined in the [official specification](https://github.com/instrumentation-score/spec/blob/main/specification.md#score-calculation-formula):
- **Critical:** 40 (highest priority - security, compliance, critical functionality)
- **Important:** 30 (significant impact on quality)
- **Normal:** 20 (standard best practices)
- **Low:** 10 (nice-to-have improvements)

> **Note:** This project uses the exact same weights and formula as the OpenTelemetry-focused specification, ensuring consistency and comparability across different observability implementations.

**Score Categories:**
- 90-100: ✅ Excellent
- 75-89: 🟢 Good
- 50-74: ⚠️ Needs Improvement
- 0-49: 🔴 Poor

**Example Calculation:**
```
Job: api-service (100 metrics)

Rule Results:
- PROM-MET-01 (Important, W=30): 95/100 metrics passed
- PROM-MET-02 (Critical, W=40):  80/100 metrics passed
- PROM-MET-03 (Important, W=30): 90/100 metrics passed

Calculation:
Numerator   = (95×30) + (80×40) + (90×30) = 2,850 + 3,200 + 2,700 = 8,750
Denominator = (100×30) + (100×40) + (100×30) = 3,000 + 4,000 + 3,000 = 10,000

Score = (8,750 / 10,000) × 100 = 87.5% 🟢 Good
```

**Impact of Rule Weights:**

Notice how cardinality failures (Critical, W=40) have bigger impact than naming failures (Important, W=30):

```
If cardinality drops to 50/100 passed:
Numerator = (95×30) + (50×40) + (90×30) = 7,550
Score = 75.5% (dropped 12 points!)

If naming drops to 50/100 passed:
Numerator = (50×30) + (80×40) + (90×30) = 7,550  
Score = 75.5% (dropped 12 points, same as above)
```

This demonstrates why cardinality is marked as **Critical** - it has the highest weight and directly impacts costs and performance.

### 4. Cost Estimation

When `--show-costs` is enabled:

```
Cost = total_cardinality × cost_per_series

Example:
  Total Active Series: 986,370
  Cost per series: $0.00615/month
  
  Estimated Cost = 986,370 × 0.00615 = $6,066.18/month
```

**Cost Breakdown:**
- Typical cloud Prometheus pricing: ~$6-8 per 1,000 active series/month
- Use `--cost-unit-price` to match your provider's pricing (e.g., `0.00615` for $6.15/1k series)
- Costs scale linearly with cardinality
- High-cardinality metrics are the primary cost driver

---

## 📋 Command Reference

### `analyze`
Collect metrics from Grafana Cloud and group by job.

```bash
./instrumentation-score-service analyze \
  --output-dir ./reports \
  --additional-query-filters 'cluster=~"prod-1-27-a1|prod-eu-central"'
```

**Environment Variables:**
- `login`: Prometheus username:password or username:api_key (optional, for authenticated endpoints)
- `url`: Prometheus API URL (required)

**Flags:**
- `--additional-query-filters`: PromQL label filters (e.g., `cluster=~"prod.*"`)
  - Filters both metric name discovery AND per-metric queries
  - Significantly reduces processing time for large Prometheus instances

**Output:**
- `job_metrics_TIMESTAMP/`: Directory with per-job files
- `metrics_errors_TIMESTAMP.txt`: Error log

**S3 Upload (Optional):**
Upload generated reports directly to S3:

```bash
./instrumentation-score-service analyze \
  --output-dir ./reports \
  --s3-upload \
  --s3-bucket my-metrics-bucket \
  --s3-prefix instrumentation-reports \
  --s3-region eu-west-1
```

Or use environment variables:
```bash
export S3_BUCKET=my-metrics-bucket
export S3_PREFIX=instrumentation-reports
export AWS_REGION=eu-west-1

./instrumentation-score-service analyze \
  --output-dir ./reports \
  --s3-upload
```

**S3 Flags:**
- `--s3-upload`: Enable S3 upload
- `--s3-bucket`: S3 bucket name (or `S3_BUCKET` env var)
- `--s3-prefix`: S3 key prefix (or `S3_PREFIX` env var)
- `--s3-region`: AWS region (or `AWS_REGION` env var, default: eu-west-1)

**S3 Output Structure:**
```
s3://my-bucket/instrumentation-reports/
├── job_metrics_20251102_160000/
│   ├── job1.txt
│   ├── job2.txt
│   └── ...
└── metrics_errors_20251102_160000.txt
```

### `evaluate-single-job`
Evaluate one job's metrics.

```bash
./instrumentation-score-service evaluate-single-job \
  --job-file reports/job_metrics_*/api-service.txt \
  --rules rules_config.yaml \
  --output text|json|html \
  --html-file report.html
```

### `evaluate-all-jobs`
Evaluate all jobs in a directory.

```bash
./instrumentation-score-service evaluate-all-jobs \
  --job-dir reports/job_metrics_20251102_160000/ \
  --rules rules_config.yaml \
  --output summary.json \
  --html-file all-jobs.html \
  --show-costs \
  --cost-unit-price 0.00615
```

**Flags:**
- `--show-costs`: Enable cost calculation
- `--cost-unit-price`: Cost per active series per month (e.g., 0.00615 = $6.15/1000 series)
- `--min-score`: Highlight jobs below threshold

**Output:**
- JSON with `total_cost` and per-job `estimated_cost`
- HTML report with:
  - Total cost in sidebar
  - Jobs sorted by score (worst first)
  - Per-metric cost breakdown

**S3 Download (Optional):**
Download job metrics from S3 and evaluate:

```bash
./instrumentation-score-service evaluate \
  --s3-source \
  --s3-bucket my-metrics-bucket \
  --s3-prefix instrumentation-reports/job_metrics_20251102_160000 \
  --s3-region eu-west-1 \
  --output html \
  --html-file dashboard.html \
  --show-costs \
  --cost-unit-price 0.00615
```

Or use environment variables:
```bash
export S3_BUCKET=my-metrics-bucket
export S3_PREFIX=instrumentation-reports/job_metrics_20251102_160000
export AWS_REGION=eu-west-1

./instrumentation-score-service evaluate \
  --s3-source \
  --output html \
  --html-file dashboard.html
```

**S3 Flags:**
- `--s3-source`: Download job metrics from S3
- `--s3-bucket`: S3 bucket name (or `S3_BUCKET` env var)
- `--s3-prefix`: S3 key prefix/path to job_metrics directory (or `S3_PREFIX` env var)
- `--s3-region`: AWS region (or `AWS_REGION` env var, default: eu-west-1)

**Note:** Files are downloaded to a temporary directory and automatically cleaned up after evaluation.

**S3 Upload (Optional):**
Upload evaluation results (reports, dashboards) to S3 for centralized storage:

```bash
./instrumentation-score-service evaluate \
  --job-dir reports/job_metrics_20251102_160000/ \
  --output html,json \
  --html-file dashboard.html \
  --json-file report.json \
  --show-costs \
  --cost-unit-price 0.00615 \
  --s3-upload \
  --s3-bucket my-metrics-bucket \
  --s3-prefix instrumentation-reports \
  --s3-run-id prod-20251102
```

Or use environment variables:
```bash
export S3_BUCKET=my-metrics-bucket
export S3_PREFIX=instrumentation-reports
export AWS_REGION=eu-west-1

./instrumentation-score-service evaluate \
  --job-dir reports/job_metrics_20251102_160000/ \
  --output html,json \
  --html-file dashboard.html \
  --json-file report.json \
  --s3-upload \
  --s3-run-id prod-20251102
```

**S3 Upload Flags:**
- `--s3-upload`: Enable S3 upload of evaluation results
- `--s3-run-id`: Custom run ID for organization (default: auto-generated timestamp)
- Other S3 flags same as download mode

**S3 Output Structure with Metadata:**
```
s3://my-bucket/instrumentation-reports/evaluations/prod-20251102/
├── dashboard.html           # HTML report
├── report.json             # JSON results
├── metrics.prom            # Prometheus metrics (if generated)
└── manifest.json           # Run metadata
```

**Manifest Contents:**
```json
{
  "timestamp": "2025-11-02T16:00:00Z",
  "run_id": "prod-20251102",
  "total_jobs": 45,
  "average_score": 87.5,
  "total_cardinality": 1500000,
  "total_cost": 9225.00,
  "rules_config": "rules_config.yaml",
  "output_formats": "html,json",
  "source_type": "local_directory",
  "source_path": "reports/job_metrics_20251102_160000/",
  "files": {
    "json": "evaluations/prod-20251102/report.json",
    "html": "evaluations/prod-20251102/dashboard.html",
    "manifest": "evaluations/prod-20251102/manifest.json"
  }
}
```

### `evaluate-metrics`
Legacy command for evaluating raw metrics files.

```bash
./instrumentation-score-service evaluate-metrics \
  --data-dir ./testdata \
  --rules rules_config.yaml \
  --output json
```

---

## 📏 Current Rules

### PROM-MET-01: Naming Conventions
- **Impact:** Important (Weight: 30)
- **Purpose:** Consistent naming improves discoverability and maintainability
- **Checks:**
  - Metric names must use snake_case format: `^[a-z][a-z0-9_]*[a-z0-9]$`
  - Should include appropriate suffixes (_total, _seconds, _bytes, _ratio)

### PROM-MET-02: Cardinality Control
- **Impact:** Critical (Weight: 40)
- **Purpose:** Control costs and prevent performance issues
- **Why Critical?**
  - Direct 1:1 relationship with infrastructure costs
  - Primary cause of Prometheus performance issues
  - Can cause complete system failures if unbounded
  - Difficult to fix retroactively once data is ingested
- **Checks:**
  - Each metric must have < 10,000 unique time series (cardinality)

**Cost Impact Example:**
```
Metric with 10,000 series at $0.00615/series = $61.50/month
Metric with 50,000 series = $307.50/month (5x cost!)
```

### PROM-MET-03: Label Best Practices
- **Impact:** Important (Weight: 30)
- **Purpose:** Prevent future cardinality issues and improve maintainability
- **Checks:**
  - Labels must NOT contain unbounded identifiers:
    - `user_id`, `session_id`, `request_id`, `trace_id`
  - Metrics must have ≤ 10 labels per metric
  - Excessive labels increase potential cardinality exponentially

**Why These Matter:**
- Forbidden labels should be in logs/traces, not metrics
- Each additional label multiplies potential cardinality
- Example: 10 labels with 5 values each = 5^10 = 9.7M possible series!

### Using This as a Framework: Adding Custom Rules

> **📖 For comprehensive framework documentation, see [FRAMEWORK.md](FRAMEWORK.md)**
> 
> **🗺️ Confused about field mappings? See [RULES_FIELD_MAPPING.md](RULES_FIELD_MAPPING.md)**

This tool is designed as a **flexible framework** for defining your own instrumentation quality rules without code changes.

#### Quick Start: Add a New Rule

**Step 1: Define the Rule** in `rules_config.yaml`:

```yaml
- rule_id: "PROM-MET-04"
  description: "Metrics must have help text and type annotations"
  impact: "Normal"  # Weight: 20
  validators:
    - name: "prom_metrics_documentation_check"
  type: "format"
      data_source: "metadata"
  conditions:
        - field: "help_text"
          operator: "not_empty"
```

**Step 2: Document the Rule** in `rules/PROM-MET-04.md`:

```markdown
# PROM-MET-04: Metric Documentation

Prometheus metrics must include help text and type annotations to ensure discoverability and proper usage.

**Rationale:** Undocumented metrics are difficult to understand and use correctly, leading to misinterpretation and incorrect alerting.

**Criteria:**
- Each metric must have a HELP annotation
- Each metric must have a TYPE annotation (counter, gauge, histogram, summary)

**Examples:**

Good:
# HELP http_requests_total Total HTTP requests
# TYPE http_requests_total counter
http_requests_total{method="GET"} 1234

Bad:
http_requests_total{method="GET"} 1234
```

**Step 3: Test Your Rule:**

```bash
./instrumentation-score-service evaluate \
  --job-file reports/job_metrics_*/api-service.txt \
  --rules rules_config.yaml \
  --output text
```

#### Available Validator Types

| Type | Purpose | Data Source | Example Use Case |
|------|---------|-------------|------------------|
| `cardinality` | Check time series count | `cardinality` | Prevent high-cardinality explosions |
| `labels` | Validate label names/values | `labels` | Block forbidden labels (user_id, etc.) |
| `label_count` | Enforce label limits | `cardinality` | Max 10 labels per metric |
| `format` | Validate naming patterns | `cardinality` | Enforce snake_case naming |

#### Available Operators

| Operator | Type | Description | Example |
|----------|------|-------------|---------|
| `lt` | Numeric | Less than | `value: 10000` |
| `lte` | Numeric | Less than or equal | `value: 10` |
| `gt` | Numeric | Greater than | `value: 0` |
| `gte` | Numeric | Greater than or equal | `value: 1` |
| `eq` | Numeric/String | Equals | `value: 100` |
| `ne` | Numeric/String | Not equals | `value: 0` |
| `contains` | String | Contains substring | `value: "_total"` |
| `not_contains` | String | Does not contain | `value: "user_id"` |
| `matches` | String | Regex match | `value: "^[a-z][a-z0-9_]*$"` |

#### Complete Rule Example

```yaml
- rule_id: "PROM-MET-05"
  description: "Counter metrics must use _total suffix"
  impact: "Important"  # Weight: 30
  validators:
    - name: "prom_counter_suffix_check"
      type: "format"
      data_source: "cardinality"
      conditions:
        # Check if metric name ends with _total
        - field: "metric_name"
          operator: "matches"
          value: ".*_total$"
```

#### Customization Examples

**1. Adjust Impact Levels:**
```yaml
- rule_id: "PROM-MET-02"
  impact: "Critical"  # Change to: Important, Normal, or Low
```

**2. Modify Thresholds:**
```yaml
      conditions:
        - field: "count"
          operator: "lt"
    value: 50000  # Increase from 10,000 for your needs
```

**3. Add Multiple Conditions (AND logic):**
```yaml
  validators:
  - name: "prom_histogram_check"
    type: "format"
    data_source: "cardinality"
      conditions:
      - field: "metric_name"
        operator: "contains"
        value: "_bucket"
      - field: "label_count"
        operator: "gte"
        value: 1  # Must have at least 'le' label
```

**4. Combine Multiple Validators (OR logic):**
```yaml
- rule_id: "PROM-MET-06"
  description: "Metrics must use standard units"
  impact: "Normal"
  validators:
    - name: "check_seconds_suffix"
      type: "format"
      data_source: "cardinality"
conditions:
  - field: "metric_name"
    operator: "matches"
          value: ".*_seconds$"

    - name: "check_bytes_suffix"
      type: "format"
      data_source: "cardinality"
conditions:
        - field: "metric_name"
          operator: "matches"
          value: ".*_bytes$"
```

#### Real-World Rule Examples

**Block Specific Label Names:**
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
    
    - name: "no_ip_labels"
      type: "labels"
      data_source: "labels"
      conditions:
        - field: "label_name"
          operator: "not_contains"
          value: "ip_address"
```

**Enforce Namespace Prefixes:**
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
          value: "^(mycompany|http|process|go)_.*"
```

**Limit Cardinality by Team:**
```yaml
- rule_id: "ORG-COST-01"
  description: "Frontend metrics limited to 5k series"
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

#### Framework Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    rules_config.yaml                         │
│  (Declarative Rule Definitions - No Code Required)          │
└────────────────────┬────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────┐
│                    Rule Engine                               │
│  internal/engine/engine.go                                   │
│  • Loads rules from YAML                                     │
│  • Evaluates conditions against metric data                  │
│  • Calculates scores using spec formula                      │
└────────────────────┬────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────┐
│                 Output Formatters                            │
│  internal/formatters/output.go                                   │
│  • HTML, JSON, Text, Prometheus formats                      │
│  • Cost calculations                                         │
│  • Per-metric failure details                                │
└─────────────────────────────────────────────────────────────┘
```

**Key Benefits:**
- ✅ **No Code Changes:** Add rules by editing YAML only
- ✅ **Hot Reload:** Rules are loaded at runtime
- ✅ **Type Safe:** Validator types enforce correct data sources
- ✅ **Extensible:** Easy to add new validator types if needed
- ✅ **Testable:** Rules can be tested independently

#### Testing Your Custom Rules

**1. Unit Test with Sample Data:**
```bash
# Create test data file
echo "test-job|my_metric_total|method,status|150" > test_metric.txt

# Evaluate
./instrumentation-score-service evaluate \
  --job-file test_metric.txt \
  --rules rules_config.yaml \
  --output text
```

**2. Validate Rule Syntax:**
```bash
# Check YAML syntax
yamllint rules_config.yaml

# Test rule loading
./instrumentation-score-service evaluate \
  --job-file test_metric.txt \
  --rules rules_config.yaml \
  --output json | jq '.rules'
```

**3. Compare Before/After:**
```bash
# Baseline score
./instrumentation-score-service evaluate \
  --job-dir reports/ \
  --rules rules_config.yaml \
  --output baseline.json

# Add new rule, re-evaluate
./instrumentation-score-service evaluate \
  --job-dir reports/ \
  --rules rules_config.yaml \
  --output updated.json

# Compare scores
diff <(jq '.average_score' baseline.json) <(jq '.average_score' updated.json)
```

#### When to Extend the Engine (Code Changes)

Most use cases can be handled with YAML configuration. Consider code changes only if you need:

1. **New Data Sources:** Currently supports `cardinality`, `labels`, `metadata`
   - Add new data source in `cmd/analyze.go`
   - Update file format in `internal/loaders/reports.go`

2. **New Validator Types:** Currently supports `cardinality`, `labels`, `label_count`, `format`
   - Add new validator in `internal/engine/engine.go`
   - Update `ValidatorConfig` in `internal/engine/rule_definition.go`

3. **New Operators:** Currently supports `lt`, `lte`, `gt`, `gte`, `eq`, `ne`, `contains`, `not_contains`, `matches`
   - Add new operator logic in `evaluateCondition()` function

**Example: Adding a New Validator Type**
```go
// internal/engine/engine.go

func (e *RuleEngine) evaluateValidatorWithStats(
    validator ValidatorConfig,
    cardData []CardinalityData,
    labelsData []LabelsData,
) (passedCount, totalCount int, failedMetrics []string, err error) {
    
    switch validator.Type {
    case "cardinality":
        return e.evaluateCardinalityValidatorWithStats(validator, cardData)
    case "labels":
        return e.evaluateLabelsValidatorWithStats(validator, labelsData)
    case "my_new_type":  // Add your new type here
        return e.evaluateMyNewValidatorWithStats(validator, cardData)
    default:
        return 0, 0, nil, fmt.Errorf("unknown validator type: %s", validator.Type)
    }
}
```

---

## 🔧 Configuration

### Rules Configuration (`rules_config.yaml`)

Define validation rules without code changes:

```yaml
- rule_id: "PROM-MET-01"
  description: "Metrics must follow naming conventions"
  impact: "Important"
  validators:
    - name: "prom_metrics_format_check"
      type: "format"
      data_source: "cardinality"
      conditions:
        - field: "metric_name"
          operator: "matches"
          value: "^[a-z][a-z0-9_]*[a-z0-9]$"

- rule_id: "PROM-MET-02"
  description: "Labels must maintain bounded cardinality"
  impact: "Critical"
  validators:
    - name: "prom_metrics_cardinality_check"
      type: "cardinality"
      data_source: "cardinality"
      conditions:
        - field: "count"
          operator: "lt"
          value: 10000
```

**Validator Types:**
- `cardinality`: Check metric cardinality (time series count)
- `labels`: Validate label names and values
- `label_count`: Enforce label count limits
- `format`: Validate metric naming patterns

**Operators:**
- Comparison: `lt`, `lte`, `gt`, `gte`, `eq`, `ne`
- String: `contains`, `not_contains`, `matches` (regex)

---

## 📈 Output Formats

### Text
Human-readable terminal output with color coding.

### JSON
Machine-readable format for automation:
```json
{
  "timestamp": "2025-11-02T16:00:00Z",
  "total_jobs": 1054,
  "average_score": 87.53,
  "total_cost": 7956.62,
  "jobs": [
    {
      "job_name": "api-service",
      "total_metrics": 45,
      "total_cardinality": 7197,
      "estimated_cost": 1.99,
      "instrumentation_score": 97.63,
      "rules": [...]
    }
  ]
}
```

### HTML
Interactive web report with:
- 🔍 Searchable job list
- 📊 Score visualization (color-coded rings)
- 💰 Cost breakdown
- 📈 Per-metric tables
- 💡 Actionable recommendations
- 🎯 Jobs sorted by score

### Prometheus Metrics
Export scores as Prometheus metrics:
```
instrumentation_score{service="api-service"} 97.63
```

---

## 💼 Use Cases

### 1. CI/CD Quality Gates
```bash
# Fail build if score drops below 80%
score=$(./instrumentation-score-service evaluate-single-job \
  --job-file metrics.txt \
  --rules rules_config.yaml \
  --output json | jq '.instrumentation_score')

if (( $(echo "$score < 80" | bc -l) )); then
  echo "❌ Instrumentation score too low: $score%"
  exit 1
fi
```

### 2. Cost Monitoring
```bash
# Generate weekly cost reports
./instrumentation-score-service evaluate-all-jobs \
  --job-dir reports/job_metrics_latest/ \
  --rules rules_config.yaml \
  --output weekly-report.json \
  --show-costs \
  --cost-unit-price 0.00615

# Alert on cost increases
```

### 3. Team Dashboards
Generate HTML reports for each team:
```bash
# Frontend team
./instrumentation-score-service evaluate-all-jobs \
  --job-dir reports/job_metrics_latest/ \
  --rules rules_config.yaml \
  --html-file frontend-dashboard.html \
  --show-costs \
  --cost-unit-price 0.00615
```

### 4. Prometheus Integration
```bash
# Export scores to Prometheus
./instrumentation-score-service evaluate-all-jobs \
  --job-dir reports/job_metrics_latest/ \
  --rules rules_config.yaml \
  --output prometheus | curl -X POST http://pushgateway:9091/metrics/job/instrumentation_score
```

---

## 🧪 Development

### Build
```bash
make build
```

### Test
```bash
make test
```

### Run Tests with Coverage
```bash
make test-coverage
```

### Available Commands
```bash
make help
```

### Project Structure
```
.
├── cmd/                    # CLI commands
│   ├── analyze.go         # Metrics collection
│   ├── evaluate_single_job.go
│   ├── evaluate_all_jobs.go
│   └── evaluate_metrics.go
├── internal/
│   ├── engine/            # Rule evaluation engine
│   │   ├── engine.go
│   │   └── rule_definition.go
│   ├── output/            # Output formatters
│   │   └── output.go
│   └── reports/           # Data loaders
│       └── reports.go
├── rules/                 # Rule documentation
│   ├── PROM-MET-01.md
│   └── PROM-MET-02.md
├── rules_config.yaml      # Rule definitions
└── testdata/              # Test fixtures
```

---

## 🐳 Docker

### Build Image
```bash
docker build -t instrumentation-score-service .
```

### Run
```bash
docker run -v $(pwd)/reports:/reports \
  -e login="user:api_key" \
  -e url="https://your-prometheus-instance.com/api/prom" \
  instrumentation-score-service analyze \
  --output-dir /reports
```

---

## 🔍 Troubleshooting

### No metrics found
**Problem:** `No metrics found in job file`

**Solution:**
- Verify the job file exists and has data
- Check file format: `JOB|METRIC_NAME|LABELS|CARDINALITY`
- Ensure metrics were collected for that job

### Score is 0%
**Problem:** All rules failing

**Solution:**
- Check `rules_config.yaml` syntax
- Verify data sources match validator types
- Review rule conditions (may be too strict)

### Evaluation is slow
**Problem:** Processing takes too long

**Solution:**
- Reduce metrics count (filter in `analyze`)
- Use `--min-score` to skip high-scoring jobs
- Process jobs in parallel batches

### High costs reported
**Problem:** Unexpected cost estimates

**Solution:**
- Review high-cardinality metrics
- Check for metrics with >10 labels
- Look for unbounded label values (IDs, timestamps)

---

## 📖 Best Practices

### 1. Regular Collection
Run `analyze` daily or weekly to track trends:
```bash
0 2 * * * /path/to/instrumentation-score-service analyze --output-dir /reports/$(date +\%Y\%m\%d)
```

### 2. Set Realistic Thresholds
Start with current baseline, improve incrementally:
```yaml
# Start lenient, tighten over time
- field: "count"
  operator: "lt"
  value: 50000  # Start here
  # value: 10000  # Target
```

### 3. Focus on Critical Rules
Fix Critical (weight 40) issues first for maximum score impact.

### 4. Monitor Costs
Enable `--show-costs` to track metrics storage expenses:
```bash
# Adjust based on your provider's pricing (example: $6.15 per 1k series/month)
--cost-unit-price 0.00615
```

---

## 📖 Relationship to the Instrumentation Score Specification

### Spec Compliance

This project is **fully compliant** with the [Instrumentation Score specification v0.1](https://github.com/instrumentation-score/spec/blob/main/specification.md) created by [OllyGarden](https://olly.garden).

**What we implement from the spec:**
- ✅ **Scoring Formula:** Exact implementation of `Score = (Σ(P_i × W_i) / Σ(T_i × W_i)) × 100`
- ✅ **Impact Weights:** Critical (40), Important (30), Normal (20), Low (10)
- ✅ **Score Categories:** Excellent (90-100), Good (75-89), Needs Improvement (50-74), Poor (0-49)
- ✅ **Rule-Based Evaluation:** Structured rules with ID, Description, Rationale, Criteria, Target, Impact
- ✅ **Transparent Calculation:** Clear, auditable scoring methodology

### Key Adaptation: OTLP → Prometheus

The original specification focuses on **OpenTelemetry Protocol (OTLP)** data:
- **Target:** OTLP traces, metrics, and logs
- **Data Model:** Resource attributes, TraceSpan, Metric, Log
- **Rules:** Based on OpenTelemetry Semantic Conventions

This project **adapts the same principles** to **Prometheus metrics**:
- **Target:** Prometheus exposition format metrics
- **Data Model:** Metric name, labels, cardinality
- **Rules:** Based on Prometheus naming conventions and best practices

### Example: Spec Rule Adaptation

**Original Spec Rule (OpenTelemetry):**
```yaml
ID: RES-001
Description: service.name must be present
Target: Resource
Criteria: Resource has service.name attribute
Impact: Critical
```

**Prometheus Adaptation (This Project):**
```yaml
rule_id: PROM-MET-02
description: Metric labels must maintain bounded cardinality
target: Metric
criteria: Cardinality < 10,000 per metric
impact: Critical
```

### Why This Matters

1. **Unified Standard:** Teams using different observability stacks (OTLP vs Prometheus) can now use the same quality measurement framework
2. **Vendor Neutral:** The spec is open-source and community-driven, not tied to any vendor
3. **Future-Proof:** As the spec evolves, this project can adopt new rules and methodologies
4. **Interoperability:** Scores are comparable across OpenTelemetry and Prometheus implementations

### Contributing to the Spec

The Instrumentation Score specification is an open standard seeking community input. If you have ideas for Prometheus-specific rules or improvements:

1. **Spec Repository:** [github.com/instrumentation-score/spec](https://github.com/instrumentation-score/spec)
2. **Discussions:** Participate in [spec discussions](https://github.com/instrumentation-score/spec/discussions)
3. **This Project:** Implement and test rules here, then propose them to the spec

---

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

This means you are free to use, modify, and distribute this software for any purpose, including commercial use, as long as you include the original license and copyright notice.

---

## 🤝 Contributing

We welcome contributions from the community! Whether you're fixing bugs, adding features, improving documentation, or sharing feedback, your help is appreciated.

### Ways to Contribute

- 🐛 [Report bugs](https://github.com/instrumentation-score-service/instrumentation-score/issues/new?template=bug_report.md)
- 💡 [Request features](https://github.com/instrumentation-score-service/instrumentation-score/issues/new?template=feature_request.md)
- 📖 Improve documentation
- 🔧 Submit pull requests
- ⭐ Star the project if you find it useful!

### Getting Started

1. Read our [Contributing Guide](CONTRIBUTING.md)
2. Check out our [Code of Conduct](CODE_OF_CONDUCT.md)
3. Look for ["good first issue"](https://github.com/instrumentation-score-service/instrumentation-score/labels/good%20first%20issue) labels
4. Join the conversation in [Discussions](https://github.com/instrumentation-score-service/instrumentation-score/discussions)

For detailed contribution guidelines, see [CONTRIBUTING.md](CONTRIBUTING.md).

---

## 🆘 Support

- 📖 [Instrumentation Score Spec](https://github.com/instrumentation-score/spec)
- 🐛 [Report Issues](https://github.com/instrumentation-score-service/instrumentation-score/issues)
- 💬 [Discussions](https://github.com/instrumentation-score-service/instrumentation-score/discussions)

---

## 🎯 Roadmap

- [ ] Support for OpenTelemetry metrics
- [ ] Automated remediation suggestions
- [ ] Grafana dashboard integration
- [ ] Slack/Teams notifications
- [ ] Historical trend analysis
- [ ] Custom rule templates

---

**Made with ❤️ for better observability**
