package topsis

import "math"

type Criterion struct {
	Value    float64
	Ideal    float64
	NegIdeal float64
	Weight   float64
}

type NodeCriteria struct {
	NodeName string
	Criteria []Criterion
}

func Score(nodes []NodeCriteria) map[string]float64 {
	if len(nodes) == 0 {
		return map[string]float64{}
	}

	distIdeal := make([]float64, len(nodes))
	distNegIdeal := make([]float64, len(nodes))

	for i, node := range nodes {
		var sumIdeal, sumNeg float64
		for _, c := range node.Criteria {
			span := math.Abs(c.NegIdeal - c.Ideal)
			var norm float64
			if span != 0 {
				norm = (c.Value - c.Ideal) / span
			}
			wn := c.Weight * norm
			sumIdeal += wn * wn

			var normNeg float64
			if span != 0 {
				normNeg = (c.NegIdeal - c.Value) / span
			}
			wnNeg := c.Weight * normNeg
			sumNeg += wnNeg * wnNeg
		}
		distIdeal[i] = math.Sqrt(sumIdeal)
		distNegIdeal[i] = math.Sqrt(sumNeg)
	}

	scores := make(map[string]float64, len(nodes))
	var minRel, maxRel float64
	rel := make([]float64, len(nodes))

	for i := range nodes {
		denom := distIdeal[i] + distNegIdeal[i]
		if denom == 0 {
			rel[i] = 0
		} else {
			rel[i] = distNegIdeal[i] / denom
		}
		if i == 0 || rel[i] < minRel {
			minRel = rel[i]
		}
		if i == 0 || rel[i] > maxRel {
			maxRel = rel[i]
		}
	}

	for i, nc := range nodes {
		var scaled float64
		if maxRel == minRel {
			scaled = 100
		} else {
			scaled = (rel[i] - minRel) / (maxRel - minRel) * 100
		}
		scores[nc.NodeName] = scaled
	}

	return scores
}
