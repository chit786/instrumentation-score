**Rule ID:** PROM-MET-02

**Description:** Prometheus metrics must maintain bounded cardinality to control costs and performance.

**Rationale:** High-cardinality metrics directly impact infrastructure costs and system performance. Each unique combination of label values creates a new time series, consuming storage, memory, and CPU resources. Unbounded cardinality leads to:
- Exponential cost increases (storage, ingestion, querying)
- Query timeouts and dashboard failures
- Out-of-memory errors in Prometheus
- Increased network bandwidth for remote write
- Degraded alerting performance

Controlling cardinality at the metric level is the most critical factor in maintaining a healthy, cost-effective observability system.

**Target:** Metric

**Criteria:** Each Prometheus metric MUST have fewer than 10,000 unique time series (cardinality) within a 1-hour window. This limit ensures predictable costs and stable query performance.

**Impact:** Critical
