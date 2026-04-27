package happyedge

import (
	"fmt"

	config "github.com/montypx/happy-edge-scheduling-plugin/apis/config"
	"k8s.io/apimachinery/pkg/runtime"
)

type HappyEdgeArgs = config.HappyEdgeArgs
type MetricConfig = config.MetricConfig
type ClusterMetricConfig = config.ClusterMetricConfig

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
		if cfg.Query == "" {
			return nil, fmt.Errorf("metrics[%s].query must not be empty", name)
		}
		if cfg.Weight <= 0 {
			return nil, fmt.Errorf("metrics[%s].weight must be > 0", name)
		}
		if cfg.Ideal >= cfg.Worst {
			return nil, fmt.Errorf("metrics[%s].ideal must be less than worst", name)
		}
	}

	for groupKey, overrides := range args.Groups {
		for name, cfg := range overrides {
			if cfg.Query == "" {
				return nil, fmt.Errorf("groups[%s][%s].query must not be empty", groupKey, name)
			}
			if cfg.Weight <= 0 {
				return nil, fmt.Errorf("groups[%s][%s].weight must be > 0", groupKey, name)
			}
			if cfg.Ideal >= cfg.Worst {
				return nil, fmt.Errorf("groups[%s][%s].ideal must be less than worst", groupKey, name)
			}
		}
	}

	for name, cfg := range args.ClusterMetrics {
		if cfg.Query == "" {
			return nil, fmt.Errorf("clusterMetrics[%s].query must not be empty", name)
		}
		if cfg.Tolerance <= 0 {
			return nil, fmt.Errorf("clusterMetrics[%s].tolerance must be > 0", name)
		}
	}

	return args, nil
}
