package engine

// RulesConfig represents the complete rules configuration from YAML
type RulesConfig struct {
	ExclusionList []ExclusionEntry `yaml:"exclusion_list"`
	Rules         []RuleDefinition `yaml:"rules"`
}

// ExclusionEntry defines a job or job+metrics to exclude from evaluation
type ExclusionEntry struct {
	Job            string   `yaml:"job,omitempty"`              // Exact job name to exclude
	JobNamePattern string   `yaml:"job_name_pattern,omitempty"` // Regex pattern to match job names
	Metrics        []string `yaml:"metrics,omitempty"`          // Specific metrics to exclude
}

// RuleDefinition represents a declarative rule loaded from YAML
type RuleDefinition struct {
	RuleID      string            `yaml:"rule_id"`
	Description string            `yaml:"description"`
	Impact      string            `yaml:"impact"`
	Validators  []ValidatorConfig `yaml:"validators"`
}

// ValidatorConfig defines a validation check
type ValidatorConfig struct {
	Name          string                 `yaml:"name"`
	Type          string                 `yaml:"type"` // "cardinality", "labels", "label_count", "format"
	DataSource    string                 `yaml:"data_source"`
	UITitle       string                 `yaml:"ui_title,omitempty"`
	UIDescription string                 `yaml:"ui_description,omitempty"`
	Conditions    []ConditionConfig      `yaml:"conditions"`
	Parameters    map[string]interface{} `yaml:"parameters,omitempty"`
}

// ConditionConfig defines a validation condition
type ConditionConfig struct {
	Field    string      `yaml:"field"`
	Operator string      `yaml:"operator"` // "matches", "contains", "gt", "lt", "gte", "lte", "eq", "not_contains"
	Value    interface{} `yaml:"value"`
}
