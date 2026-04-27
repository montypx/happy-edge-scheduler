package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:defaulter-gen=true

type HappyEdgeArgs struct {
	metav1.TypeMeta `json:",inline"`

	PrometheusURL  string                             `json:"prometheusURL"`
	ScrapeInterval metav1.Duration                    `json:"scrapeInterval"`
	Metrics        map[string]MetricConfig            `json:"metrics"`
	Groups         map[string]map[string]MetricConfig `json:"groups,omitempty"`
	ClusterMetrics map[string]ClusterMetricConfig     `json:"clusterMetrics,omitempty"`
}

type MetricConfig struct {
	Query  string  `json:"query"`
	Ideal  float64 `json:"ideal"`
	Worst  float64 `json:"worst"`
	Weight float64 `json:"weight"`
}

type ClusterMetricConfig struct {
	Query     string  `json:"query"`
	Tolerance float64 `json:"tolerance"`
}
