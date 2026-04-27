package happyedge

import (
	"context"
	"fmt"
	"strconv"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	fwk "k8s.io/kube-scheduler/framework"

	"github.com/montypx/happy-edge-scheduling-plugin/pkg/plugins/happyedge/topsis"
)

const Name = "HappyEdge"

const scoreStateKey fwk.StateKey = Name

var logger = klog.NewKlogr().WithName(Name)

type HappyEdge struct {
	handle       fwk.Handle
	metricsCache MetricsReader
	args         *HappyEdgeArgs
}

type scoreState struct {
	scores map[string]float64
}

func (s *scoreState) Clone() fwk.StateData {
	cp := &scoreState{scores: make(map[string]float64, len(s.scores))}
	for k, v := range s.scores {
		cp.scores[k] = v
	}
	return cp
}

var _ fwk.PreFilterPlugin = &HappyEdge{}
var _ fwk.FilterPlugin = &HappyEdge{}
var _ fwk.PreScorePlugin = &HappyEdge{}
var _ fwk.ScorePlugin = &HappyEdge{}

func New(ctx context.Context, obj runtime.Object, h fwk.Handle) (fwk.Plugin, error) {
	args, err := NewHappyEdgeArgs(obj)
	if err != nil {
		return nil, err
	}
	cache, err := NewMetricsCache(ctx, args.PrometheusURL, args.ScrapeInterval.Duration, args.Metrics, args.ClusterMetrics)
	if err != nil {
		return nil, err
	}
	return &HappyEdge{
		handle:       h,
		metricsCache: cache,
		args:         args,
	}, nil
}

func (pl *HappyEdge) Name() string {
	return Name
}

func (pl *HappyEdge) nodeMetricConfig(node *v1.Node, metricName string) MetricConfig {
	if groupName, ok := node.Labels["happyedge.io/group"]; ok {
		if overrides, ok := pl.args.Groups[groupName]; ok {
			if cfg, ok := overrides[metricName]; ok {
				return cfg
			}
		}
	}
	return pl.args.Metrics[metricName]
}

func (pl *HappyEdge) PreFilter(
	ctx context.Context,
	state fwk.CycleState,
	pod *v1.Pod,
	_ []fwk.NodeInfo,
) (*fwk.PreFilterResult, *fwk.Status) {
	for metricName, cfg := range pl.args.ClusterMetrics {
		val, ok := pl.metricsCache.GetClusterMetric(metricName)
		if !ok {
			return nil, fwk.NewStatus(fwk.Error, metricName+" metric not available")
		}
		if val > cfg.Tolerance {
			logger.V(2).Info("pod rejected at PreFilter: cluster metric exceeds tolerance",
				"pod", klog.KObj(pod),
				"metric", metricName,
				"value", val,
				"tolerance", cfg.Tolerance,
			)
			return nil, fwk.NewStatus(
				fwk.Unschedulable,
				fmt.Sprintf("cluster %s=%.2f exceeds tolerance %.2f", metricName, val, cfg.Tolerance),
			)
		}
	}
	return nil, fwk.NewStatus(fwk.Success, "")
}

func (pl *HappyEdge) PreFilterExtensions() fwk.PreFilterExtensions {
	return nil
}

func (pl *HappyEdge) Filter(
	ctx context.Context,
	state fwk.CycleState,
	pod *v1.Pod,
	nodeInfo fwk.NodeInfo,
) *fwk.Status {
	node := nodeInfo.Node()
	if node == nil {
		return fwk.NewStatus(fwk.Error, "node not found")
	}
	for metricName := range pl.args.Metrics {
		val, ok := pl.metricsCache.GetNodeMetric(metricName, node.Name)
		if !ok {
			return fwk.NewStatus(fwk.Error, fmt.Sprintf("%s not available for node %s", metricName, node.Name))
		}
		cfg := pl.nodeMetricConfig(node, metricName)
		if val > cfg.Worst {
			logger.V(2).Info("node rejected at Filter: metric exceeds worst threshold",
				"node", node.Name,
				"metric", metricName,
				"value", val,
				"worst", cfg.Worst,
			)
			return fwk.NewStatus(
				fwk.Unschedulable,
				fmt.Sprintf("node %s: %s=%.2f exceeds worst threshold %.2f", node.Name, metricName, val, cfg.Worst),
			)
		}
	}
	return fwk.NewStatus(fwk.Success, "")
}

func (pl *HappyEdge) PreScore(
	ctx context.Context,
	state fwk.CycleState,
	pod *v1.Pod,
	nodes []fwk.NodeInfo,
) *fwk.Status {
	nodeCriteria := make([]topsis.NodeCriteria, 0, len(nodes))
	for _, nodeInfo := range nodes {
		node := nodeInfo.Node()
		if node == nil {
			continue
		}
		var criteria []topsis.Criterion
		for metricName := range pl.args.Metrics {
			val, ok := pl.metricsCache.GetNodeMetric(metricName, node.Name)
			if !ok {
				continue
			}
			cfg := pl.nodeMetricConfig(node, metricName)
			weight := cfg.Weight
			if ann, ok := pod.Annotations["happyedge.io/weight-"+metricName]; ok {
				if w, err := strconv.ParseFloat(ann, 64); err == nil {
					weight = w
				}
			}
			criteria = append(criteria, topsis.Criterion{
				Value:    val,
				Ideal:    cfg.Ideal,
				NegIdeal: cfg.Worst,
				Weight:   weight,
			})
		}
		nodeCriteria = append(nodeCriteria, topsis.NodeCriteria{
			NodeName: node.Name,
			Criteria: criteria,
		})
	}
	scores := topsis.Score(nodeCriteria)
	for nodeName, score := range scores {
		logger.V(2).Info("TOPSIS score assigned", "node", nodeName, "score", score)
	}
	state.Write(scoreStateKey, &scoreState{scores: scores})
	return fwk.NewStatus(fwk.Success, "")
}

func (pl *HappyEdge) Score(
	ctx context.Context,
	state fwk.CycleState,
	pod *v1.Pod,
	nodeInfo fwk.NodeInfo,
) (int64, *fwk.Status) {
	node := nodeInfo.Node()
	if node == nil {
		return 0, fwk.NewStatus(fwk.Success, "")
	}
	data, err := state.Read(scoreStateKey)
	if err != nil {
		return 0, fwk.NewStatus(fwk.Success, "")
	}
	ss, ok := data.(*scoreState)
	if !ok {
		return 0, fwk.NewStatus(fwk.Success, "")
	}
	score, ok := ss.scores[node.Name]
	if !ok {
		return 0, fwk.NewStatus(fwk.Success, "")
	}
	return int64(score), fwk.NewStatus(fwk.Success, "")
}

func (pl *HappyEdge) ScoreExtensions() fwk.ScoreExtensions {
	return nil
}
