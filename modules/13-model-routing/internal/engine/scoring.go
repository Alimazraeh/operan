package engine

import "sort"

// Candidate represents a scored model candidate (used by tests and scoring).
type Candidate struct {
	ModelID        string
	Capability     float64
	CostWeight     float64
	LatencyWeight  float64
	Reliability    float64
	QualityScore   float64
	CompositeScore float64
}

// ScoringEngine computes composite scores for model candidates.
type ScoringEngine struct {
	// Weights define the relative importance of each factor.
	// Default weights follow the spec:
	//   capability   * 0.40
	//   cost         * 0.20
	//   latency      * 0.15
	//   reliability  * 0.25
	//   quality      * 0.15
	WeightCapability   float64
	WeightCost         float64
	WeightLatency      float64
	WeightReliability  float64
	WeightQuality      float64
}

// NewScoringEngine returns a ScoringEngine with spec-default weights.
func NewScoringEngine() *ScoringEngine {
	return &ScoringEngine{
		WeightCapability:  0.40,
		WeightCost:        0.20,
		WeightLatency:     0.15,
		WeightReliability: 0.25,
		WeightQuality:     0.15,
	}
}

// CompositeScore computes the weighted score for a model candidate.
// capability_score * 0.40 +
// (100 - cost_weight) * 0.20 +
// (100 - latency_weight) * 0.15 +
// reliability_weight * 0.25 +
// quality_score * 0.15
func (s *ScoringEngine) CompositeScore(
	capability, costWeight, latencyWeight, reliability, quality float64,
) float64 {
	return capability*s.WeightCapability +
		(100-costWeight)*s.WeightCost +
		(100-latencyWeight)*s.WeightLatency +
		reliability*s.WeightReliability +
		quality*s.WeightQuality
}

// Rank sorts candidates by composite score descending (in-place).
// Deprecated: ranking is now done inline in the router.
func (s *ScoringEngine) Rank(candidates []Candidate) {
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].CompositeScore > candidates[j].CompositeScore
	})
}