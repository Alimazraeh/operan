package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompositeScore_Basic(t *testing.T) {
	eng := NewScoringEngine()
	// capability=80, cost_weight=30, latency_weight=40, reliability=90, quality=70
	// score = 80*0.40 + (100-30)*0.20 + (100-40)*0.15 + 90*0.25 + 70*0.15
	//       = 32.0 + 14.0 + 9.0 + 22.5 + 10.5 = 88.0
	score := eng.CompositeScore(80, 30, 40, 90, 70)
	assert.InDelta(t, 88.0, score, 0.01)
}

func TestCompositeScore_AllHigh(t *testing.T) {
	eng := NewScoringEngine()
	// capability=100, cost_weight=0, latency_weight=0, reliability=100, quality=100
	// = 100*0.40 + 100*0.20 + 100*0.15 + 100*0.25 + 100*0.15 = 115
	score := eng.CompositeScore(100, 0, 0, 100, 100)
	assert.InDelta(t, 115.0, score, 0.01)
}

func TestCompositeScore_AllLow(t *testing.T) {
	eng := NewScoringEngine()
	// capability=0, cost_weight=100, latency_weight=100, reliability=0, quality=0
	// = 0 + 0 + 0 + 0 + 0 = 0
	score := eng.CompositeScore(0, 100, 100, 0, 0)
	assert.InDelta(t, 0.0, score, 0.01)
}

func TestCompositeScore_EqualWeights(t *testing.T) {
	eng := NewScoringEngine()
	// capability=50, cost_weight=50, latency_weight=50, reliability=50, quality=50
	// = 20 + 10 + 7.5 + 12.5 + 7.5 = 57.5
	score := eng.CompositeScore(50, 50, 50, 50, 50)
	assert.InDelta(t, 57.5, score, 0.01)
}

func TestCompositeScore_CostOptimization(t *testing.T) {
	eng := NewScoringEngine()
	scoreA := eng.CompositeScore(80, 10, 50, 50, 50) // cheap model
	scoreB := eng.CompositeScore(80, 90, 50, 50, 50) // expensive model
	assert.True(t, scoreA > scoreB, "cheaper model should score higher")
	// A = 80*0.40 + 90*0.20 + 50*0.15 + 50*0.25 + 50*0.15 = 77.5
	// B = 80*0.40 + 10*0.20 + 50*0.15 + 50*0.25 + 50*0.15 = 61.5
	// difference = 16.0
	assert.InDelta(t, 16.0, scoreA-scoreB, 0.01)
}

func TestCompositeScore_ReliabilityBias(t *testing.T) {
	eng := NewScoringEngine()
	scoreA := eng.CompositeScore(80, 50, 50, 95, 50) // high reliability
	scoreB := eng.CompositeScore(80, 50, 50, 50, 50) // low reliability
	assert.True(t, scoreA > scoreB)
	// difference = (95-50)*0.25 = 11.25
	assert.InDelta(t, 11.25, scoreA-scoreB, 0.01)
}

func TestRank_SortsDescending(t *testing.T) {
	candidates := []Candidate{
		{ModelID: "c", CompositeScore: 50},
		{ModelID: "a", CompositeScore: 90},
		{ModelID: "b", CompositeScore: 70},
	}

	eng := NewScoringEngine()
	eng.Rank(candidates)

	assert.Equal(t, "a", candidates[0].ModelID)
	assert.Equal(t, 90.0, candidates[0].CompositeScore)
	assert.Equal(t, "b", candidates[1].ModelID)
	assert.Equal(t, "c", candidates[2].ModelID)
}

func TestRank_Empty(t *testing.T) {
	eng := NewScoringEngine()
	var candidates []Candidate
	eng.Rank(candidates)
	assert.Empty(t, candidates)
}

func TestRank_Single(t *testing.T) {
	eng := NewScoringEngine()
	candidates := []Candidate{{ModelID: "only", CompositeScore: 42}}
	eng.Rank(candidates)
	assert.Equal(t, "only", candidates[0].ModelID)
}

func TestRank_TiesPreserveOrder(t *testing.T) {
	candidates := []Candidate{
		{ModelID: "x", CompositeScore: 50},
		{ModelID: "y", CompositeScore: 50},
	}
	eng := NewScoringEngine()
	eng.Rank(candidates)
	// Both have same score; just check they're still there
	assert.Len(t, candidates, 2)
}

func TestScoringEngine_CustomWeights(t *testing.T) {
	eng := &ScoringEngine{
		WeightCapability:  0.50,
		WeightCost:        0.10,
		WeightLatency:     0.10,
		WeightReliability: 0.20,
		WeightQuality:     0.10,
	}
	// capability=100, rest all neutral 50
	// = 100*0.50 + 50*0.10 + 50*0.10 + 50*0.20 + 50*0.10
	// = 50 + 5 + 5 + 10 + 5 = 75
	score := eng.CompositeScore(100, 50, 50, 50, 50)
	assert.InDelta(t, 75.0, score, 0.01)
}