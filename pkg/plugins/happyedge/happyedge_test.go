package happyedge

import (
	"context"
	"strings"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fwk "k8s.io/kube-scheduler/framework"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

// -----------------------------------------------------------------------------
// Fakers & Helpers
// -----------------------------------------------------------------------------
type fakeMetrics struct {
	cluster map[string]float64
	nodes   map[string]map[string]float64
}

func (f *fakeMetrics) GetClusterMetric(name string) (float64, bool) {
	v, ok := f.cluster[name]
	return v, ok
}

func (f *fakeMetrics) GetNodeMetric(metric, node string) (float64, bool) {
	byNode, ok := f.nodes[metric]
	if !ok {
		return 0, false
	}
	v, ok := byNode[node]
	return v, ok
}

func makeNode(name string) *v1.Node {
	return &v1.Node{ObjectMeta: metav1.ObjectMeta{Name: name}}
}

func makeNodeLabeled(name string, labels map[string]string) *v1.Node {
	return &v1.Node{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels}}
}

func toNodeInfo(n *v1.Node) fwk.NodeInfo {
	ni := framework.NewNodeInfo()
	ni.SetNode(n)
	return ni
}

// -----------------------------------------------------------------------------
// Test Cluster Tolerance Enforcement
// -----------------------------------------------------------------------------
func TestPreFilter(t *testing.T) {
	args := &HappyEdgeArgs{
		ClusterMetrics: map[string]ClusterMetricConfig{
			"cluster_power": {Query: `sum(power_usage_watts)`, Tolerance: 100},
		},
	}

	cases := []struct {
		desc  string
		power float64
		found bool
		want  fwk.Code
	}{
		{"power well under tolerance", 40, true, fwk.Success},
		{"power at exact tolerance boundary", 100, true, fwk.Success},
		{"power just above tolerance", 100.01, true, fwk.Unschedulable},
		{"cluster_power absent from cache", 0, false, fwk.Error},
	}

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			cache := map[string]float64{}
			if c.found {
				cache["cluster_power"] = c.power
			}
			pl := &HappyEdge{metricsCache: &fakeMetrics{cluster: cache}, args: args}
			_, got := pl.PreFilter(context.Background(), nil, &v1.Pod{}, nil)
			if got.Code() != c.want {
				t.Errorf("code: got %v, want %v", got.Code(), c.want)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Test Node Tolerance Enforcement
// -----------------------------------------------------------------------------
func TestFilter(t *testing.T) {
	singleMetricArgs := &HappyEdgeArgs{
		Metrics: map[string]MetricConfig{
			"cpu_temp": {Query: `node_hwmon_temp_celsius`, Ideal: 0, Worst: 80, Weight: 1},
		},
	}

	cases := []struct {
		desc  string
		cache map[string]map[string]float64
		want  fwk.Code
	}{
		{
			"temp well under worst threshold",
			map[string]map[string]float64{"cpu_temp": {"node1": 50}},
			fwk.Success,
		},
		{
			"temp at exact worst boundary still passes",
			map[string]map[string]float64{"cpu_temp": {"node1": 80}},
			fwk.Success,
		},
		{
			"temp just above worst threshold",
			map[string]map[string]float64{"cpu_temp": {"node1": 80.01}},
			fwk.Unschedulable,
		},
		{
			"node has no cached metrics at all",
			nil,
			fwk.Error,
		},
	}

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			pl := &HappyEdge{
				metricsCache: &fakeMetrics{nodes: c.cache},
				args:         singleMetricArgs,
			}
			got := pl.Filter(context.Background(), nil, &v1.Pod{}, toNodeInfo(makeNode("node1")))
			if got.Code() != c.want {
				t.Errorf("code: got %v, want %v", got.Code(), c.want)
			}
		})
	}

	t.Run("node has some but not all configured metrics", func(t *testing.T) {
		pl := &HappyEdge{
			metricsCache: &fakeMetrics{nodes: map[string]map[string]float64{
				"cpu_temp": {"node1": 50},
			}},
			args: &HappyEdgeArgs{
				Metrics: map[string]MetricConfig{
					"cpu_temp": {Query: `node_hwmon_temp_celsius`, Ideal: 0, Worst: 80, Weight: 1},
					"cpu_util": {Query: `cpu_utilization`, Ideal: 0, Worst: 90, Weight: 1},
				},
			},
		}
		got := pl.Filter(context.Background(), nil, &v1.Pod{}, toNodeInfo(makeNode("node1")))
		if got.Code() != fwk.Error {
			t.Errorf("missing metric should return Error, got %v", got.Code())
		}
	})
}

// -----------------------------------------------------------------------------
// Test Node Grouping
// -----------------------------------------------------------------------------
func TestFilterGroupOverride(t *testing.T) {
	pl := &HappyEdge{
		metricsCache: &fakeMetrics{nodes: map[string]map[string]float64{
			"cpu_temp": {"node1": 85},
		}},
		args: &HappyEdgeArgs{
			Metrics: map[string]MetricConfig{
				"cpu_temp": {Query: `node_hwmon_temp_celsius`, Ideal: 0, Worst: 80, Weight: 1},
			},
			Groups: map[string]map[string]MetricConfig{
				"jetson": {
					"cpu_temp": {Query: `node_hwmon_temp_celsius`, Ideal: 0, Worst: 90, Weight: 1},
				},
			},
		},
	}

	t.Run("no group label uses default worst=80, blocks node at 85", func(t *testing.T) {
		got := pl.Filter(context.Background(), nil, &v1.Pod{}, toNodeInfo(makeNode("node1")))
		if got.Code() != fwk.Unschedulable {
			t.Errorf("got %v, want Unschedulable", got.Code())
		}
	})

	t.Run("jetson group raises worst to 90, node at 85 passes", func(t *testing.T) {
		n := makeNodeLabeled("node1", map[string]string{"happyedge.io/group": "jetson"})
		got := pl.Filter(context.Background(), nil, &v1.Pod{}, toNodeInfo(n))
		if got.Code() != fwk.Success {
			t.Errorf("got %v, want Success", got.Code())
		}
	})
}

// -----------------------------------------------------------------------------
// Test Node Comparison
// -----------------------------------------------------------------------------

// Implements the scheduling cycle to run PreScore -> Run TOPSIS -> Run Score
// 2x symmetric nodes for testing weight overrides with TOPSIS

// nodeJetson: cpu_util=10 (good), cpu_temp=90 (bad)
// nodeRk: cpu_util=90 (bad),  cpu_temp=10 (good)
// Both metrics: Ideal=0, Worst=100, Weight=1 by default.
func TestPreScoreAndScore(t *testing.T) {
	args := &HappyEdgeArgs{
		Metrics: map[string]MetricConfig{
			"cpu_util": {Query: `cpu_utilization`, Ideal: 0, Worst: 100, Weight: 1},
			"cpu_temp": {Query: `cpu_temp_celsius`, Ideal: 0, Worst: 100, Weight: 1},
		},
	}

	nodeJetson := makeNode("nodeJetson")
	nodeRk := makeNode("nodeRk")

	symmetricCache := map[string]map[string]float64{
		"cpu_util": {"nodeJetson": 10, "nodeRk": 90},
		"cpu_temp": {"nodeJetson": 90, "nodeRk": 10},
	}

	runPreScore := func(pl *HappyEdge, pod *v1.Pod, nodes ...fwk.NodeInfo) *framework.CycleState {
		state := framework.NewCycleState()
		pl.PreScore(context.Background(), state, pod, nodes)
		return state
	}
	scoreOf := func(pl *HappyEdge, state *framework.CycleState, pod *v1.Pod, n *v1.Node) int64 {
		s, _ := pl.Score(context.Background(), state, pod, toNodeInfo(n))
		return s
	}

	t.Run("symmetric nodes with equal weights score identically", func(t *testing.T) {
		pl := &HappyEdge{metricsCache: &fakeMetrics{nodes: symmetricCache}, args: args}
		state := runPreScore(pl, &v1.Pod{}, toNodeInfo(nodeJetson), toNodeInfo(nodeRk))
		sa := scoreOf(pl, state, &v1.Pod{}, nodeJetson)
		sb := scoreOf(pl, state, &v1.Pod{}, nodeRk)
		if sa != sb {
			t.Errorf("symmetric nodes with equal weights should score the same, got A=%d B=%d", sa, sb)
		}
	})

	t.Run("valid pod-level weight override, low cpu_util more important", func(t *testing.T) {
		pl := &HappyEdge{metricsCache: &fakeMetrics{nodes: symmetricCache}, args: args}
		pod := &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{"happyedge.io/weight-cpu_util": "10.0"},
			},
		}
		state := runPreScore(pl, pod, toNodeInfo(nodeJetson), toNodeInfo(nodeRk))
		sa := scoreOf(pl, state, pod, nodeJetson)
		sb := scoreOf(pl, state, pod, nodeRk)
		if sa <= sb {
			t.Errorf("nodeJetson (cpu_util=10) should outscore nodeRk (cpu_util=90) with added weight, got A=%d B=%d", sa, sb)
		}
	})

	t.Run("malformed weight annotation falls back to default", func(t *testing.T) {
		pl := &HappyEdge{metricsCache: &fakeMetrics{nodes: symmetricCache}, args: args}
		pod := &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{"happyedge.io/weight-cpu_util": "not_a_float"},
			},
		}
		state := runPreScore(pl, pod, toNodeInfo(nodeJetson), toNodeInfo(nodeRk))
		sa := scoreOf(pl, state, pod, nodeJetson)
		sb := scoreOf(pl, state, pod, nodeRk)
		if sa != sb {
			t.Errorf("invalid weight should be ignored, nodes should be equal, got A=%d B=%d", sa, sb)
		}
	})

	t.Run("score node when no cached metric", func(t *testing.T) {
		pl := &HappyEdge{
			metricsCache: &fakeMetrics{nodes: map[string]map[string]float64{
				"cpu_util": {"nodeJetson": 10},
				"cpu_temp": {"nodeJetson": 20},
			}},
			args: args,
		}
		state := runPreScore(pl, &v1.Pod{}, toNodeInfo(nodeJetson), toNodeInfo(nodeRk))
		_, status := pl.Score(context.Background(), state, &v1.Pod{}, toNodeInfo(nodeRk))
		if status.Code() != fwk.Success {
			t.Errorf("node with no cached metrics should still receive a score, got %v", status.Code())
		}
	})

	t.Run("score node when partial cached metric", func(t *testing.T) {
		pl := &HappyEdge{
			metricsCache: &fakeMetrics{nodes: map[string]map[string]float64{
				"cpu_util": {"nodeJetson": 10, "nodeRk": 50},
				"cpu_temp": {"nodeJetson": 20},
			}},
			args: args,
		}
		state := runPreScore(pl, &v1.Pod{}, toNodeInfo(nodeJetson), toNodeInfo(nodeRk))
		_, status := pl.Score(context.Background(), state, &v1.Pod{}, toNodeInfo(nodeRk))
		if status.Code() != fwk.Success {
			t.Errorf("partially cached node should score successfully, got %v", status.Code())
		}
	})

	t.Run("score zero when PreScore not called", func(t *testing.T) {
		pl := &HappyEdge{metricsCache: &fakeMetrics{}, args: args}
		state := framework.NewCycleState()
		score, status := pl.Score(context.Background(), state, &v1.Pod{}, toNodeInfo(nodeJetson))
		if score != 0 || status.Code() != fwk.Success {
			t.Errorf("unpopulated state: got score=%d code=%v, want 0/Success", score, status.Code())
		}
	})

	t.Run("group-exclusive metric pod annotation overrides weight in scoring", func(t *testing.T) {
		groupArgs := &HappyEdgeArgs{
			Metrics: map[string]MetricConfig{
				"cpu_util": {Query: "x", Ideal: 0, Worst: 100, Weight: 1},
			},
			Groups: map[string]map[string]MetricConfig{
				"jetson": {
					"npu_util": {Query: "x", Ideal: 0, Worst: 100, Weight: 1},
				},
			},
		}
		pl := &HappyEdge{
			metricsCache: &fakeMetrics{nodes: map[string]map[string]float64{
				"cpu_util": {"nodeA": 50, "nodeB": 10},
				"npu_util": {"nodeA": 10, "nodeB": 90},
			}},
			args: groupArgs,
		}
		nodeA := makeNodeLabeled("nodeA", map[string]string{"happyedge.io/group": "jetson"})
		nodeB := makeNodeLabeled("nodeB", map[string]string{"happyedge.io/group": "jetson"})
		pod := &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{"happyedge.io/weight-npu_util": "10.0"},
			},
		}
		state := runPreScore(pl, pod, toNodeInfo(nodeA), toNodeInfo(nodeB))
		sa := scoreOf(pl, state, pod, nodeA)
		sb := scoreOf(pl, state, pod, nodeB)
		if sa <= sb {
			t.Errorf("nodeA (npu_util=10) should outscore nodeB (npu_util=90) with weight-npu_util=10, got A=%d B=%d", sa, sb)
		}
	})

	t.Run("score zero for missing score for node", func(t *testing.T) {
		pl := &HappyEdge{metricsCache: &fakeMetrics{nodes: symmetricCache}, args: args}
		state := runPreScore(pl, &v1.Pod{}, toNodeInfo(nodeJetson), toNodeInfo(nodeRk))
		score, _ := pl.Score(context.Background(), state, &v1.Pod{}, toNodeInfo(makeNode("ghost")))
		if score != 0 {
			t.Errorf("unknown node should score 0, got %d", score)
		}
	})
}

// -----------------------------------------------------------------------------
// Test Config Validation Constraints
// -----------------------------------------------------------------------------
func TestValidation(t *testing.T) {
	base := func() *HappyEdgeArgs {
		return &HappyEdgeArgs{
			PrometheusURL:  "http://prometheus:9090",
			ScrapeInterval: metav1.Duration{Duration: 30_000_000_000},
			Metrics: map[string]MetricConfig{
				"cpu_temp": {Query: `node_hwmon_temp_celsius`, Ideal: 0, Worst: 80, Weight: 1},
			},
		}
	}

	cases := []struct {
		desc    string
		mutate  func(*HappyEdgeArgs)
		wantErr string
	}{
		{
			"valid configuration passes without error",
			func(_ *HappyEdgeArgs) {},
			"",
		},
		{
			"missing PrometheusURL",
			func(a *HappyEdgeArgs) { a.PrometheusURL = "" },
			"prometheusURL must not be empty",
		},
		{
			"zero ScrapeInterval",
			func(a *HappyEdgeArgs) { a.ScrapeInterval = metav1.Duration{} },
			"scrapeInterval must be > 0",
		},
		{
			"empty Metrics map",
			func(a *HappyEdgeArgs) { a.Metrics = nil },
			"metrics must define at least one metric",
		},
		{
			"ideal equal to worst in Metrics",
			func(a *HappyEdgeArgs) {
				a.Metrics["cpu_temp"] = MetricConfig{Query: "x", Ideal: 80, Worst: 80, Weight: 1}
			},
			"metrics[cpu_temp].ideal must not be the same as worst",
		},
		{
			"empty query in Metrics",
			func(a *HappyEdgeArgs) { a.Metrics["cpu_temp"] = MetricConfig{Weight: 1} },
			"metrics[cpu_temp].query must not be empty",
		},
		{
			"zero weight in Metrics",
			func(a *HappyEdgeArgs) { a.Metrics["cpu_temp"] = MetricConfig{Query: "x", Weight: 0} },
			"metrics[cpu_temp].weight must be > 0",
		},
		{
			"ideal equal to worst inside group",
			func(a *HappyEdgeArgs) {
				a.Groups = map[string]map[string]MetricConfig{
					"jetson": {"cpu_temp": {Query: "x", Ideal: 80, Worst: 80, Weight: 1}},
				}
			},
			"groups[jetson][cpu_temp].ideal must not be the same as worst",
		},
		{
			"empty query in ClusterMetrics",
			func(a *HappyEdgeArgs) {
				a.ClusterMetrics = map[string]ClusterMetricConfig{
					"cluster_power": {Tolerance: 100},
				}
			},
			"clusterMetrics[cluster_power].query must not be empty",
		},
		{
			"zero tolerance in ClusterMetrics",
			func(a *HappyEdgeArgs) {
				a.ClusterMetrics = map[string]ClusterMetricConfig{
					"cluster_power": {Query: "x", Tolerance: 0},
				}
			},
			"clusterMetrics[cluster_power].tolerance must be > 0",
		},
	}

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			args := base()
			c.mutate(args)
			_, err := NewHappyEdgeArgs(args)
			if c.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Errorf("expected error containing %q, got nil", c.wantErr)
				return
			}
			if !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("error %q does not contain %q", err.Error(), c.wantErr)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Test Scheduling Cooldown
// -----------------------------------------------------------------------------
func TestSchedulingCooldown(t *testing.T) {
	const cooldown = 100 * time.Millisecond
	pod := &v1.Pod{}

	newPl := func(d time.Duration) *HappyEdge {
		return &HappyEdge{
			metricsCache: &fakeMetrics{},
			args:         &HappyEdgeArgs{PostBindDelay: metav1.Duration{Duration: d}},
		}
	}

	t.Run("no cooldown configured, PreFilter always passes", func(t *testing.T) {
		pl := newPl(0)
		pl.PostBind(context.Background(), nil, pod, "node1")
		_, got := pl.PreFilter(context.Background(), nil, pod, nil)
		if got.Code() != fwk.Success {
			t.Errorf("got %v, want Success", got.Code())
		}
	})

	t.Run("PostBind activates cooldown and PreFilter blocks scheduling", func(t *testing.T) {
		pl := newPl(cooldown)
		pl.PostBind(context.Background(), nil, pod, "node1")
		_, got := pl.PreFilter(context.Background(), nil, pod, nil)
		if got.Code() != fwk.Unschedulable {
			t.Errorf("during cooldown: got %v, want Unschedulable", got.Code())
		}
	})

	t.Run("PreFilter allows scheduling after cooldown expires", func(t *testing.T) {
		pl := newPl(cooldown)
		pl.PostBind(context.Background(), nil, pod, "node1")
		time.Sleep(cooldown * 3)
		_, got := pl.PreFilter(context.Background(), nil, pod, nil)
		if got.Code() != fwk.Success {
			t.Errorf("after cooldown: got %v, want Success", got.Code())
		}
	})

	t.Run("second PostBind resets the timer extending the cooldown", func(t *testing.T) {
		pl := newPl(cooldown)
		pl.PostBind(context.Background(), nil, pod, "node1")
		time.Sleep(cooldown / 2)
		pl.PostBind(context.Background(), nil, pod, "node2")
		time.Sleep(cooldown / 2)
		_, got := pl.PreFilter(context.Background(), nil, pod, nil)
		if got.Code() != fwk.Unschedulable {
			t.Errorf("after reset, before new expiry: got %v, want Unschedulable", got.Code())
		}
		time.Sleep(cooldown * 2)
		_, got = pl.PreFilter(context.Background(), nil, pod, nil)
		if got.Code() != fwk.Success {
			t.Errorf("after reset expiry: got %v, want Success", got.Code())
		}
	})
}
