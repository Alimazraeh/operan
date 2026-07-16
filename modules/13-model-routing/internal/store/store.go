package store

// DBPool is the interface for database operations used by stores.
// Both *pgxpool.Pool and pgxmock.PgxPoolIface satisfy this.
type DBPool interface {
	Exec(ctx interface{}, sql string, arguments ...interface{}) (tag interface{}, err error)
	Query(ctx interface{}, sql string, args ...interface{}) (interface{}, error)
	QueryRow(ctx interface{}, sql string, args ...interface{}) interface{}
	Ping(ctx interface{}) error
	Close() error
}

// RuleStore defines the interface for routing rules persistence.
type RuleStore interface {
	CreateRule(rule *RoutingRule) error
	GetRule(id string, tenantID string) (*RoutingRule, error)
	ListRules(tenantID string, taskType *string, isActive *bool, page, pageSize int) ([]RoutingRule, int, error)
	UpdateRule(rule *RoutingRule) error
	DeleteRule(id string, tenantID string) error
	AddModelToRule(model *RoutingRuleModel) error
	GetModelsForRule(ruleID string) ([]RoutingRuleModel, error)
	ListActiveRulesByTask(tenantID, taskType string) ([]RuleWithModels, error)
}

// PerfStore defines the interface for performance metrics persistence.
type PerfStore interface {
	RecordMetric(metric *RoutingPerformance) error
	GetByModel(tenantID, modelID string) ([]RoutingPerformance, error)
	GetByTaskType(tenantID, taskType string) ([]RoutingPerformance, error)
	GetByModelAndTask(tenantID, modelID, taskType string) (*RoutingPerformance, error)
}