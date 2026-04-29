package config

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type HappyEdgeArgs struct {
	metav1.TypeMeta

	PrometheusURL  string
	ScrapeInterval metav1.Duration
	PostBindDelay  metav1.Duration
	Metrics        map[string]MetricConfig
	Groups         map[string]map[string]MetricConfig
	ClusterMetrics map[string]ClusterMetricConfig
}

type MetricConfig struct {
	Query  string
	Ideal  float64
	Worst  float64
	Weight float64
}

type ClusterMetricConfig struct {
	Query     string
	Tolerance float64
}
