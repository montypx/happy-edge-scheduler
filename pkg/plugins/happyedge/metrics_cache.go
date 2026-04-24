package happyedge

import (
	"context"
	"fmt"
	"sync"
	"time"

	promapi "github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

type MetricsReader interface {
	GetNodeMetric(metricName, nodeName string) (float64, bool)
	GetClusterMetric(metricName string) (float64, bool)
}

type MetricsCache struct {
	mu             sync.RWMutex
	client         promv1.API
	scrapeInterval time.Duration
	metrics        map[string]MetricConfig
	clusterMetrics map[string]ClusterMetricConfig
	nodeValues     map[string]map[string]float64
	clusterValues  map[string]float64
}

func NewMetricsCache(url string, scrapeInterval time.Duration, metrics map[string]MetricConfig, clusterMetrics map[string]ClusterMetricConfig) (*MetricsCache, error) {
	c, err := promapi.NewClient(promapi.Config{Address: url})
	if err != nil {
		return nil, err
	}
	mc := &MetricsCache{
		client:         promv1.NewAPI(c),
		scrapeInterval: scrapeInterval,
		metrics:        metrics,
		clusterMetrics: clusterMetrics,
		nodeValues:     make(map[string]map[string]float64),
		clusterValues:  make(map[string]float64),
	}
	mc.scrape(context.Background())
	go mc.run()
	return mc, nil
}

func (mc *MetricsCache) run() {
	ticker := time.NewTicker(mc.scrapeInterval)
	defer ticker.Stop()
	for range ticker.C {
		mc.scrape(context.Background())
	}
}

func (mc *MetricsCache) scrape(ctx context.Context) {
	nodeValues := make(map[string]map[string]float64, len(mc.metrics))
	for name, cfg := range mc.metrics {
		nodeValues[name] = mc.queryNodeVector(ctx, cfg.Query)
	}

	clusterValues := make(map[string]float64, len(mc.clusterMetrics))
	for name, cfg := range mc.clusterMetrics {
		val, err := mc.queryScalar(ctx, cfg.Query)
		if err != nil {
			continue
		}
		clusterValues[name] = val
	}

	mc.mu.Lock()
	mc.nodeValues = nodeValues
	mc.clusterValues = clusterValues
	mc.mu.Unlock()
}

func (mc *MetricsCache) queryScalar(ctx context.Context, query string) (float64, error) {
	result, _, err := mc.client.Query(ctx, query, time.Now())
	if err != nil {
		return 0, err
	}
	switch val := result.(type) {
	case *model.Scalar:
		return float64(val.Value), nil
	case model.Vector:
		if len(val) == 0 {
			return 0, fmt.Errorf("empty vector")
		}
		return float64(val[0].Value), nil
	default:
		return 0, fmt.Errorf("unexpected result type %T", result)
	}
}

func (mc *MetricsCache) queryNodeVector(ctx context.Context, query string) map[string]float64 {
	result, _, err := mc.client.Query(ctx, query, time.Now())
	if err != nil {
		return nil
	}
	vector, ok := result.(model.Vector)
	if !ok {
		return nil
	}
	out := make(map[string]float64, len(vector))
	for _, sample := range vector {
		node := string(sample.Metric["node"])
		if node == "" {
			continue
		}
		out[node] = float64(sample.Value)
	}
	return out
}

func (mc *MetricsCache) GetNodeMetric(metricName, nodeName string) (float64, bool) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	byNode, ok := mc.nodeValues[metricName]
	if !ok {
		return 0, false
	}
	v, ok := byNode[nodeName]
	return v, ok
}

func (mc *MetricsCache) GetClusterMetric(metricName string) (float64, bool) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	v, ok := mc.clusterValues[metricName]
	return v, ok
}
