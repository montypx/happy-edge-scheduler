[![License](https://img.shields.io/badge/License-GNU%20GPL%20v3.0-blue.svg)](https://github.com/montypx/happy-edge-scheduler/blob/main/LICENSE) [![Unit Testing (CI)](https://github.com/montypx/happy-edge-scheduler/actions/workflows/test.yml/badge.svg?branch=main)](https://github.com/montypx/happy-edge-scheduler/actions/workflows/test.yml)

# HappyEdge Scheduling Plugin

![Kubernetes](https://img.shields.io/badge/Kubernetes-326CE5?style=flat&logo=kubernetes&logoColor=white)
![Go](https://img.shields.io/badge/Go-00ADD8?style=flat&logo=go&logoColor=white)

A Kubernetes scheduler plugin that lets you configure scheduling behaviour using Prometheus metrics data. Unofficial out-of-tree plugin that has been tested on a four node heterogeneous testbed.

## Maturity Level

- [x] 💡 Sample (for demonstrating and inspiring purpose)
- [ ] 👶 Alpha (used in companies for pilot projects)
- [ ] 👦 Beta (used in companies and developed actively)
- [ ] 👨 Stable (used in companies for production workloads)

## Images

| Compiled Go Version | Compiled With k8s Version | Container Image                             | Arch                       |
| ------------------- | ------------------------- | ------------------------------------------- | -------------------------- |
| 1.26.2              | v1.35.2                   | ghcr.io/montypx/happy-edge-scheduler:latest | linux/amd64<br>linux/arm64 |
| 1.26.2              | v1.35.2                   | ghcr.io/montypx/happy-edge-scheduler:dev    | linux/amd64<br>linux/arm64 |

## Plugin Design

The plugin implements five scheduling extension points. Each is opt-in and they compose on top of each other.

**PreFilter** and **Filter** create a circuit breaker pattern. PreFilter runs once per pod and checks cluster-wide metrics against configured tolerance thresholds. It takes advantage of Kubernetes internal unschedulable / backoff queues to do rescheduling.

**PreScore** and **Score** rank candidate nodes using [TOPSIS](https://en.wikipedia.org/wiki/TOPSIS) (Technique for Order of Preference by Similarity to Ideal Solution). Each node is scored 0-100 based on its weighted distance from the ideal and worst values across all configured metrics.

The direction of each metric is inferred from the config: if `ideal < worst`, lower values are better. If `ideal > worst`, higher values are better.

**PostBind** is a rate limiter. When `postBindDelay` is set, it starts a cooldown timer after each successful bind. Any pod that tries to schedule during the cooldown is blocked at PreFilter until the timer expires. Each new bind resets the timer, so back-to-back scheduling extends the gap.

---

**TLDR:** Run PreFilter and Filter alone for hard constraint enforcement, PostBind can be used ontop of these. Add PreScore and Score for MCDM ranking.

## Installation

### As a secondary scheduler

Deploy HappyEdge alongside the default scheduler. Pods opt in by setting `schedulerName: happy-edge-scheduler` in their spec. Here is an example deployment manifest:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: happy-edge-scheduler
  namespace: kube-system
spec:
  replicas: 1
  selector:
    matchLabels:
      app: happy-edge-scheduler
  template:
    metadata:
      labels:
        app: happy-edge-scheduler
    spec:
      serviceAccountName: happy-edge-scheduler
      containers:
        - name: scheduler
          image: ghcr.io/montypx/happy-edge-scheduler:latest
          command:
            - /usr/local/bin/kube-scheduler
            - --config=/etc/kubernetes/scheduler-config.yaml
            - --v=2
          volumeMounts:
            - name: config
              mountPath: /etc/kubernetes/scheduler-config.yaml
              subPath: config.yaml
      volumes:
        - name: config
          configMap:
            name: happy-edge-scheduler-config
```

The `--v=2` flag enables verbose logging. Use `ghcr.io/montypx/happy-edge-scheduler:dev` for the latest pre-release build.

### As the default scheduler

To replace `kube-scheduler` entirely, consult your distribution's documentation.

## Configuration

The plugin is configured through a standard `KubeSchedulerConfiguration` manifest. Load it as a ConfigMap before deploying:

```bash
kubectl create configmap happy-edge-scheduler-config \
  --from-file=config.yaml=scheduler-config.yaml \
  -n kube-system
```

Full example with all features enabled:

```yaml
apiVersion: kubescheduler.config.k8s.io/v1
kind: KubeSchedulerConfiguration
profiles:
  - schedulerName: happy-edge-scheduler
    plugins:
      preFilter:
        enabled:
          - name: HappyEdge
      filter:
        enabled:
          - name: HappyEdge
      preScore:
        enabled:
          - name: HappyEdge
      score:
        disabled:
          - name: NodeResourcesBalancedAllocation
          - name: NodeResourcesFit
          - name: PodTopologySpread
          - name: ImageLocality
          - name: TaintToleration
          - name: NodeAffinity
          - name: InterPodAffinity
        enabled:
          - name: HappyEdge
      postBind:
        enabled:
          - name: HappyEdge
    pluginConfig:
      - name: HappyEdge
        args:
          prometheusURL: "http://prometheus.monitoring.svc.cluster.local:9090"
          scrapeInterval: "8s"
          postBindDelay: "10s"
          metrics:
            cpu_util:
              query: '100 - (avg(rate(node_cpu_seconds_total{mode="idle"}[30s])) by (node) * 100)'
              ideal: 40.0
              worst: 90.0
              weight: 1.0
            cpu_temp:
              query: "avg by (node) (node_thermal_zone_temp)"
              ideal: 30.0
              worst: 80.0
              weight: 2.0
            cpu_temp_predicted:
              query: "predict_linear(avg by (node) (node_thermal_zone_temp)[5m:8s], 300)"
              ideal: 30.0
              worst: 80.0
              weight: 3.0
            memory_util:
              query: "avg by (node) (100 - (node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes * 100))"
              ideal: 40.0
              worst: 85.0
              weight: 1.0
          clusterMetrics:
            cluster_power:
              query: "sum(node_power_usage_watts)"
              tolerance: 500.0
          groups:
            accelerated:
              gpu_util:
                query: "gpu_usage_percent"
                ideal: 0.0
                worst: 90.0
                weight: 1.0
              gpu_temp:
                query: "gpu_temperature_celsius"
                ideal: 30.0
                worst: 85.0
                weight: 0.5
              gpu_temp_predicted:
                query: "predict_linear(gpu_temperature_celsius[5m:8s], 300)"
                ideal: 30.0
                worst: 85.0
                weight: 0.5
              module_power:
                query: "node_power_usage_watts"
                ideal: 0.0
                worst: 10.0
                weight: 1.0
            inference:
              cpu_util:
                query: '100 - (avg(rate(node_cpu_seconds_total{mode="idle"}[30s])) by (node) * 100)'
                ideal: 0.0
                worst: 40.0
                weight: 2.0
              npu_util:
                query: "avg by (node) (avg_over_time(npu_load_percent[2m]))"
                ideal: 80.0
                worst: 0.0
                weight: 0.1
            embedded:
              cpu_temp:
                query: "avg by (node) (node_thermal_zone_temp)"
                ideal: 20.0
                worst: 50.0
                weight: 4.0
              cpu_temp_predicted:
                query: "predict_linear(avg by (node) (node_thermal_zone_temp)[5m:8s], 300)"
                ideal: 30.0
                worst: 70.0
                weight: 4.0
```

### `metrics`

Defines the criteria applied to all nodes. Every entry requires:

| Field    | Description                                                                                   |
| -------- | --------------------------------------------------------------------------------------------- |
| `query`  | PromQL expression returning a vector with a `node` label matching Kubernetes node names       |
| `ideal`  | The best expected value for the metric                                                        |
| `worst`  | The threshold beyond which Filter rejects the node. Also used as the negative-ideal in TOPSIS |
| `weight` | Relative importance in scoring. Higher means more influence on the final rank                 |

Direction is inferred automatically. Set `ideal < worst` for lower-is-better (e.g. temperature, utilisation). Set `ideal > worst` for higher-is-better. The `npu_util` entry above (ideal=80, worst=0) is an example: a node running at 80% NPU utilisation is preferred over an idle one.

At startup, the plugin performs a test query against `prometheusURL` and refuses to start if the endpoint is unreachable.

### `groups`

Add the label `happyedge.io/group: <group-name>` to a node to assign it to a group. Metrics defined under a group name override the base metric config for those nodes. You can also define metrics that only exist for a group. In the example above, `gpu_temp` is only evaluated on nodes labelled `accelerated`.

### `clusterMetrics`

Checked at PreFilter before any node is evaluated. If a cluster metric exceeds its `tolerance`, the pod is blocked immediately. Useful for hard limits like total cluster power draw. Each entry requires a `query` and a `tolerance` value greater than zero.

### `postBindDelay`

A Go duration string (e.g. `"10s"`, `"500ms"`). Omit it or set it to `"0s"` to disable the cooldown.

## Usage

### Choosing which extension points to enable

Start with just the circuit breakers:

```yaml
plugins:
  preFilter:
    enabled:
      - name: HappyEdge
  filter:
    enabled:
      - name: HappyEdge
```

This enforces cluster and node thresholds without touching the default scoring. Pods are rejected if metrics are out of range; scheduling otherwise proceeds normally.

Add TOPSIS ranking when you want HappyEdge to influence which node wins, not just which nodes are eligible:

```yaml
preScore:
  enabled:
    - name: HappyEdge
score:
  disabled:
    - name: NodeResourcesBalancedAllocation
    - name: NodeResourcesFit
    # ... other default plugins
  enabled:
    - name: HappyEdge
```

Disabling the default Score plugins avoids conflicts. Leave them enabled and their scores will be added to HappyEdge's output, which may produce rankings that do not reflect what you configured.

Add PostBind to rate-limit scheduling:

```yaml
postBind:
  enabled:
    - name: HappyEdge
```

This requires `postBindDelay` to be set in `pluginConfig`. Without it, PostBind is a no-op.

### Per-pod weight overrides

A pod can override the weight of any metric at scheduling time using annotations:

```yaml
metadata:
  annotations:
    happyedge.io/weight-cpu_temp: "5.0"
    happyedge.io/weight-npu_util: "0.1"
```

Useful when a specific workload has different priorities than the global defaults. Invalid values are silently ignored and fall back to the configured weight.

### Logging

At `--v=2`, the plugin logs:

- TOPSIS scores per node per scheduling cycle
- Per-metric rejection reasons at Filter and PreFilter, including the metric name and value
- Cooldown state changes at PostBind
- A warning at each scrape interval when a configured query returns no results
