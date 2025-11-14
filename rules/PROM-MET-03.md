**Rule ID:** PROM-MET-03

**Description:** Prometheus metric labels must follow best practices for maintainability.

**Rationale:** Label design impacts long-term maintainability, query complexity, and operational overhead. Labels containing unbounded identifiers (user_id, session_id, request_id, trace_id) lead to accidental cardinality explosions and should be moved to exemplars, logs, or traces. Excessive label counts increase potential cardinality exponentially, complicate queries and dashboards, and suggest poor metric design.

**Target:** Metric

**Criteria:** Labels on Prometheus metrics MUST NOT contain unbounded identifiers such as user_id, session_id, request_id, or trace_id. Metrics MUST NOT have more than 10 labels to prevent complexity and reduce cardinality explosion risk.

**Impact:** Important

