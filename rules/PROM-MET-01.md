**Rule ID:** PROM-MET-01

**Description:** Prometheus metrics must follow consistent naming, labeling, and unit conventions.

**Rationale:** Inconsistent metric names and labels lead to poor query usability, duplicated metrics, and unreliable alerting or dashboards. Following Prometheus naming conventions ensures clarity, maintainability, and consistent aggregation across services and teams.

**Target:** Metric

**Criteria:** Metric names MUST match the pattern `^[a-z][a-z0-9_]*[a-z0-9]$` (snake_case, lowercase, starting with a letter, ending with a letter or digit). Label names MUST match the pattern `^[a-z][a-z0-9_]*$` (snake_case, lowercase, starting with a letter). Label keys MUST represent bounded categorical values and avoid unbounded identifiers such as user_id or request_id. All metrics MUST include a meaningful help string describing purpose and units.

**Impact:** Important
