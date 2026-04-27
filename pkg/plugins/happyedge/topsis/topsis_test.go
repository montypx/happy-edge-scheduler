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
