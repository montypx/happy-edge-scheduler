package v1

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SetDefaultsHappyEdgeArgs sets the default parameters for HappyEdge plugin.
func SetDefaultsHappyEdgeArgs(obj *HappyEdgeArgs) {
	if obj.PrometheusURL == "" {
		obj.PrometheusURL = "http://prometheus-kube-prometheus-kube-prome-prometheus.monitoring.svc.cluster.local:9090"
	}

	if obj.ScrapeInterval.Duration == 0 {
		obj.ScrapeInterval = metav1.Duration{Duration: 10 * time.Second}
	}

	if obj.Metrics == nil {
		obj.Metrics = map[string]MetricConfig{}
	}

	if obj.ClusterMetrics == nil {
		obj.ClusterMetrics = map[string]ClusterMetricConfig{}
	}
}
