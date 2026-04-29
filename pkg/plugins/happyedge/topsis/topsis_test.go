package topsis

import (
	"math"
	"testing"
)

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------
const epsilon = 1e-9

func approxEqual(t *testing.T, got, want float64, label string) {
	t.Helper()
	if math.Abs(got-want) > epsilon {
		t.Errorf("%s: got %v, want %v", label, got, want)
	}
}

// -----------------------------------------------------------------------------
// Test Cases
// -----------------------------------------------------------------------------

func TestNilInputReturnsEmptyMap(t *testing.T) {
	if len(Score(nil)) != 0 {
		t.Error("nil input should return an empty map")
	}
}

func TestSingleNode(t *testing.T) {
	scores := Score([]NodeCriteria{
		{NodeName: "a", Criteria: []Criterion{{Value: 5, Ideal: 0, NegIdeal: 10, Weight: 1}}},
	})
	approxEqual(t, scores["a"], 100, "single node")
}

func TestNormalisedRanking(t *testing.T) {
	scores := Score([]NodeCriteria{
		{NodeName: "a", Criteria: []Criterion{{Value: 2, Ideal: 0, NegIdeal: 10, Weight: 1}}},
		{NodeName: "b", Criteria: []Criterion{{Value: 8, Ideal: 0, NegIdeal: 10, Weight: 1}}},
	})
	approxEqual(t, scores["a"], 100, "a near ideal")
	approxEqual(t, scores["b"], 0, "b near negIdeal")
}

// Nodes are symmetric
func TestWeightedCriteria(t *testing.T) {
	nodes := []NodeCriteria{
		{NodeName: "A", Criteria: []Criterion{
			{Value: 1, Ideal: 0, NegIdeal: 10, Weight: 2},
			{Value: 9, Ideal: 0, NegIdeal: 10, Weight: 1},
		}},
		{NodeName: "B", Criteria: []Criterion{
			{Value: 5, Ideal: 0, NegIdeal: 10, Weight: 2},
			{Value: 5, Ideal: 0, NegIdeal: 10, Weight: 1},
		}},
		{NodeName: "C", Criteria: []Criterion{
			{Value: 9, Ideal: 0, NegIdeal: 10, Weight: 2},
			{Value: 1, Ideal: 0, NegIdeal: 10, Weight: 1},
		}},
	}
	scores := Score(nodes)
	approxEqual(t, scores["A"], 100, "A")
	approxEqual(t, scores["B"], 50, "B")
	approxEqual(t, scores["C"], 0, "C")
}

// Boundary test: all at ideal
func TestAllAtIdeal(t *testing.T) {
	scores := Score([]NodeCriteria{
		{NodeName: "A", Criteria: []Criterion{{Value: 0, Ideal: 0, NegIdeal: 10, Weight: 1}}},
		{NodeName: "B", Criteria: []Criterion{{Value: 0, Ideal: 0, NegIdeal: 10, Weight: 1}}},
	})
	approxEqual(t, scores["A"], 100, "A")
	approxEqual(t, scores["B"], 100, "B")
}

// Boundary edge test: identical
func TestIdenticalNodes(t *testing.T) {
	scores := Score([]NodeCriteria{
		{NodeName: "A", Criteria: []Criterion{{Value: 6, Ideal: 0, NegIdeal: 10, Weight: 1}}},
		{NodeName: "B", Criteria: []Criterion{{Value: 6, Ideal: 0, NegIdeal: 10, Weight: 1}}},
		{NodeName: "C", Criteria: []Criterion{{Value: 6, Ideal: 0, NegIdeal: 10, Weight: 1}}},
	})
	approxEqual(t, scores["A"], 100, "A")
	approxEqual(t, scores["B"], 100, "B")
	approxEqual(t, scores["C"], 100, "C")
}

// Boundary edge test: ideal = negideal
func TestZeroSpan(t *testing.T) {
	scores := Score([]NodeCriteria{
		{NodeName: "A", Criteria: []Criterion{{Value: 3, Ideal: 7, NegIdeal: 7, Weight: 1}}},
		{NodeName: "B", Criteria: []Criterion{{Value: 9, Ideal: 7, NegIdeal: 7, Weight: 1}}},
	})
	approxEqual(t, scores["A"], 100, "A zero-span")
	approxEqual(t, scores["B"], 100, "B zero-span")
}

// Higher-is-better vs lower-is-better produce the same ranking when mirrored
func TestCriterionDirection(t *testing.T) {
	t.Run("lower is better: node with lower value scores higher", func(t *testing.T) {
		scores := Score([]NodeCriteria{
			{NodeName: "low", Criteria: []Criterion{{Value: 2, Ideal: 0, NegIdeal: 10, Weight: 1}}},
			{NodeName: "high", Criteria: []Criterion{{Value: 8, Ideal: 0, NegIdeal: 10, Weight: 1}}},
		})
		if scores["low"] <= scores["high"] {
			t.Errorf("lower-is-better: low (%.2f) should outscore high (%.2f)", scores["low"], scores["high"])
		}
	})

	t.Run("higher is better: node with higher value scores higher", func(t *testing.T) {
		scores := Score([]NodeCriteria{
			{NodeName: "low", Criteria: []Criterion{{Value: 2, Ideal: 10, NegIdeal: 0, Weight: 1}}},
			{NodeName: "high", Criteria: []Criterion{{Value: 8, Ideal: 10, NegIdeal: 0, Weight: 1}}},
		})
		if scores["high"] <= scores["low"] {
			t.Errorf("higher-is-better: high (%.2f) should outscore low (%.2f)", scores["high"], scores["low"])
		}
	})

	t.Run("mirrored configs produce identical relative scores", func(t *testing.T) {
		lowerBetter := Score([]NodeCriteria{
			{NodeName: "a", Criteria: []Criterion{{Value: 2, Ideal: 0, NegIdeal: 10, Weight: 1}}},
			{NodeName: "b", Criteria: []Criterion{{Value: 8, Ideal: 0, NegIdeal: 10, Weight: 1}}},
		})
		higherBetter := Score([]NodeCriteria{
			{NodeName: "a", Criteria: []Criterion{{Value: 8, Ideal: 10, NegIdeal: 0, Weight: 1}}},
			{NodeName: "b", Criteria: []Criterion{{Value: 2, Ideal: 10, NegIdeal: 0, Weight: 1}}},
		})
		approxEqual(t, lowerBetter["a"], higherBetter["a"], "mirrored score for a")
		approxEqual(t, lowerBetter["b"], higherBetter["b"], "mirrored score for b")
	})
}

// Higher-is-better and lower-is-bettr in one test
func TestMixedDirectionCriteria(t *testing.T) {
	scores := Score([]NodeCriteria{
		{NodeName: "A", Criteria: []Criterion{
			{Value: 10, Ideal: 0, NegIdeal: 100, Weight: 1}, // lower is better
			{Value: 90, Ideal: 100, NegIdeal: 0, Weight: 1}, // higher is better
		}},
		{NodeName: "B", Criteria: []Criterion{
			{Value: 90, Ideal: 0, NegIdeal: 100, Weight: 1},
			{Value: 10, Ideal: 100, NegIdeal: 0, Weight: 1},
		}},
	})
	if scores["A"] <= scores["B"] {
		t.Errorf("A (good on both criteria) should outscore B, got A=%.2f B=%.2f", scores["A"], scores["B"])
	}
}

// Boundary test: negative values
func TestValueBeyondIdeal(t *testing.T) {
	scores := Score([]NodeCriteria{
		{NodeName: "at_ideal", Criteria: []Criterion{{Value: 0, Ideal: 0, NegIdeal: 10, Weight: 1}}},
		{NodeName: "past_ideal", Criteria: []Criterion{{Value: -3, Ideal: 0, NegIdeal: 10, Weight: 1}}},
	})
	if scores["at_ideal"] <= scores["past_ideal"] {
		t.Errorf("node at ideal (%.2f) should outscore node past ideal (%.2f)",
			scores["at_ideal"], scores["past_ideal"])
	}
}
