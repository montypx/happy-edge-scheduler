package happyedge

import (
	"k8s.io/apimachinery/pkg/runtime"
)

func (in *HappyEdgeArgs) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}

	out := new(HappyEdgeArgs)
	*out = *in

	if in.Metrics != nil {
		out.Metrics = make(map[string]MetricConfig, len(in.Metrics))
		for k, v := range in.Metrics {
			out.Metrics[k] = v
		}
	}

	if in.Groups != nil {
		out.Groups = make(map[string]map[string]MetricConfig, len(in.Groups))
		for groupKey, overrides := range in.Groups {
			outOverrides := make(map[string]MetricConfig, len(overrides))
			for k, v := range overrides {
				outOverrides[k] = v
			}
			out.Groups[groupKey] = outOverrides
		}
	}

	if in.ClusterMetrics != nil {
		out.ClusterMetrics = make(map[string]ClusterMetricConfig, len(in.ClusterMetrics))
		for k, v := range in.ClusterMetrics {
			out.ClusterMetrics[k] = v
		}
	}

	return out
}
