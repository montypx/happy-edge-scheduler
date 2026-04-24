package happyedge

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var validNodeMetrics = map[string]struct{}{
	"cpu_util":     {},
	"gpu_util":     {},
	"npu_util":     {},
	"memory_util":  {},
	"disk_util":    {},
	"cpu_temp":     {},
	"gpu_temp":     {},
	"gpu_power":    {},
	"cpu_power":    {},
	"module_power": {},
}

var validClusterMetrics = map[string]struct{}{
	"cluster_power":      {},
	"cluster_power_rate": {},
}

const validNodeMetricNames = "cpu_util, cpu_temp, memory_util, disk_util, gpu_temp, gpu_util, npu_util, gpu_power, cpu_power, module_power"
const validClusterMetricNames = "cluster_power, cluster_power_rate"

// MetricConfig defines the PromQL query and TOPSIS scoring parameters for a single metric.
type MetricConfig struct {
	Query  string  `json:"query"`
	Ideal  float64 `json:"ideal"`
	Worst  float64 `json:"worst"`
	Weight float64 `json:"weight"`
}

// ClusterMetricConfig defines the PromQL query and scheduling tolerance for a cluster-level metric.
type ClusterMetricConfig struct {
	Query     string  `json:"query"`
	Tolerance float64 `json:"tolerance"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type HappyEdgeArgs struct {
	metav1.TypeMeta `json:",inline"`

	PrometheusURL  string                             `json:"prometheusURL"`
	ScrapeInterval metav1.Duration                    `json:"scrapeInterval"`
	Metrics        map[string]MetricConfig            `json:"metrics"`
	Groups         map[string]map[string]MetricConfig `json:"groups,omitempty"`
	ClusterMetrics map[string]ClusterMetricConfig     `json:"clusterMetrics,omitempty"`
}

func NewHappyEdgeArgs(obj runtime.Object) (*HappyEdgeArgs, error) {
	args, ok := obj.(*HappyEdgeArgs)
	if !ok {
		return nil, fmt.Errorf("expected *HappyEdgeArgs, got %T", obj)
	}

	if args.PrometheusURL == "" {
		return nil, fmt.Errorf("prometheusURL must not be empty")
	}
	if args.ScrapeInterval.Duration <= 0 {
		return nil, fmt.Errorf("scrapeInterval must be > 0")
	}

	if len(args.Metrics) == 0 {
		return nil, fmt.Errorf("metrics must define at least one metric")
	}
	for name, cfg := range args.Metrics {
		if _, ok := validNodeMetrics[name]; !ok {
			return nil, fmt.Errorf("metrics[%s]: unknown metric name, must be one of: %s", name, validNodeMetricNames)
		}
		if cfg.Query == "" {
			return nil, fmt.Errorf("metrics[%s].query must not be empty", name)
		}
		if cfg.Weight <= 0 {
			return nil, fmt.Errorf("metrics[%s].weight must be > 0", name)
		}
	}

	for groupKey, overrides := range args.Groups {
		for name, cfg := range overrides {
			if _, ok := validNodeMetrics[name]; !ok {
				return nil, fmt.Errorf("groups[%s][%s]: unknown metric name, must be one of: %s", groupKey, name, validNodeMetricNames)
			}
			if cfg.Query == "" {
				return nil, fmt.Errorf("groups[%s][%s].query must not be empty", groupKey, name)
			}
			if cfg.Weight <= 0 {
				return nil, fmt.Errorf("groups[%s][%s].weight must be > 0", groupKey, name)
			}
		}
	}

	for name, cfg := range args.ClusterMetrics {
		if _, ok := validClusterMetrics[name]; !ok {
			return nil, fmt.Errorf("clusterMetrics[%s]: unknown cluster metric name, must be one of: %s", name, validClusterMetricNames)
		}
		if cfg.Query == "" {
			return nil, fmt.Errorf("clusterMetrics[%s].query must not be empty", name)
		}
		if cfg.Tolerance <= 0 {
			return nil, fmt.Errorf("clusterMetrics[%s].tolerance must be > 0", name)
		}
	}

	return args, nil
}
