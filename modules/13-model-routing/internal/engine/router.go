package engine

import (
	"context"
	"fmt"
	"sort"

	"github.com/operan/model-routing/internal/store"
)

// RuleStore interface for testability.
type RuleStore interface {
	ListActiveRulesByTask(tenantID, taskType string) ([]store.RuleWithModels, error)
}

// PerfStore interface for testability.
type PerfStore interface {
	GetByModelAndTask(tenantID, modelID, taskType string) (*store.RoutingPerformance, error)
}

// Constraints defines routing request constraints.
type Constraints struct {
	MaxLatencyMs int     `json:"max_latency_ms"`
	MinQuality   float64 `json:"min_quality"`
	MaxTokens    int     `json:"max_tokens"`
	CostTarget   float64 `json:"cost_target"`
}

// RoutingDecision holds the outcome of the routing algorithm.
type RoutingDecision struct {
	ModelID         string  `json:"model_id"`
	Score           float64 `json:"score"`
	Rationale       string  `json:"rationale"`
	FallbackModel   string  `json:"fallback_model,omitempty"`
	CostEstimate    float64 `json:"cost_estimate"`
	LatencyEstimate int     `json:"latency_estimate"`
}

// Router implements the routing engine.
type Router struct {
	ruleStore RuleStore
	perfStore PerfStore
	scoring   *ScoringEngine
}

// NewRouter creates a new routing engine.
func NewRouter(rs RuleStore, ps PerfStore) *Router {
	return &Router{
		ruleStore: rs,
		perfStore: ps,
		scoring:   NewScoringEngine(),
	}
}

// Resolve selects the best model for the given task and constraints.
func (r *Router) Resolve(ctx context.Context, tenantID, taskType string, constraints Constraints) (*RoutingDecision, error) {
	rules, err := r.ruleStore.ListActiveRulesByTask(tenantID, taskType)
	if err != nil {
		return nil, fmt.Errorf("list rules: %w", err)
	}

	var candidates []candidate

	for _, rule := range rules {
		if constraints.MaxLatencyMs > 0 && rule.MaxLatencyMs < constraints.MaxLatencyMs {
			continue
		}
		if constraints.MaxTokens > 0 && rule.MaxTokens < constraints.MaxTokens {
			continue
		}

		for _, mc := range rule.Models {
			perf, _ := r.perfStore.GetByModelAndTask(tenantID, mc.ModelID, taskType)
			qualityScore := 50.0
			if perf != nil {
				qualityScore = perf.QualityScore
			}

			score := r.scoring.CompositeScore(
				mc.CapabilityScore,
				mc.CostWeight,
				mc.LatencyWeight,
				mc.ReliabilityWeight,
				qualityScore,
			)

			candidates = append(candidates, candidate{
				modelID:        mc.ModelID,
				capability:     mc.CapabilityScore,
				costWeight:     mc.CostWeight,
				latencyWeight:  mc.LatencyWeight,
				reliability:    mc.ReliabilityWeight,
				qualityScore:   qualityScore,
				compositeScore: score,
			})
		}
	}

	if len(candidates) == 0 {
		return defaultDecision(taskType), nil
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].compositeScore > candidates[j].compositeScore
	})

	best := candidates[0]
	rationale := fmt.Sprintf("Selected %s for %s task (capability=%.1f, score=%.2f)",
		best.modelID, taskType, best.capability, best.compositeScore)

	decision := &RoutingDecision{
		ModelID:         best.modelID,
		Score:           best.compositeScore,
		Rationale:       rationale,
		CostEstimate:    0,
		LatencyEstimate: 0,
	}

	if len(candidates) > 1 {
		decision.FallbackModel = candidates[1].modelID
	}

	// Apply cost filter if cost_target specified
	if constraints.CostTarget > 0 && decision.CostEstimate > constraints.CostTarget {
		if len(candidates) > 1 && candidates[1].costWeight < best.costWeight {
			decision.ModelID = candidates[1].modelID
			decision.Score = candidates[1].compositeScore
			decision.FallbackModel = best.modelID
		}
	}

	return decision, nil
}

type candidate struct {
	modelID        string
	capability     float64
	costWeight     float64
	latencyWeight  float64
	reliability    float64
	qualityScore   float64
	compositeScore float64
}

func defaultDecision(taskType string) *RoutingDecision {
	defaults := map[string]string{
		"summarize": "qwen-turbo",
		"classify":  "qwen-turbo",
		"generate":  "qwen-max",
		"extract":   "qwen-plus",
		"chat":      "qwen-plus",
		"embed":     "text-embedding-ada-002",
		"general":   "qwen-plus",
	}
	model := defaults[taskType]
	if model == "" {
		model = "qwen-plus"
	}
	return &RoutingDecision{
		ModelID:         model,
		Score:           0,
		Rationale:       fmt.Sprintf("No active routing rule for task_type=%s; using default", taskType),
		CostEstimate:    0,
		LatencyEstimate: 0,
	}
}