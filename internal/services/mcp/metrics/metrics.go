// Package metrics publishes Prometheus counters and histograms for the MCP
// gateway. Kept lightweight on purpose — LLM usage is tracked through the
// async/Redis path; tool calls don't need that pipeline.
package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// Outcome labels for the metrics. Stable strings so Grafana queries don't
// rot when we add new failure shapes.
const (
	OutcomeSuccess = "success"
	OutcomeError   = "error"
	OutcomeDenied  = "denied"
)

var (
	toolCallsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "pllm",
		Subsystem: "mcp",
		Name:      "tool_calls_total",
		Help:      "Total number of MCP tool calls through the gateway.",
	}, []string{"server", "tool", "outcome"})

	toolCallDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "pllm",
		Subsystem: "mcp",
		Name:      "tool_call_duration_seconds",
		Help:      "Latency of MCP tool calls by server and tool.",
		// Tool calls vary widely: cached lookup vs. shell-out vs. network IO.
		// Buckets span 10ms → 30s to cover that range without going absurd.
		Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
	}, []string{"server", "tool", "outcome"})
)

// RecordToolCall emits a counter increment and a duration observation.
// Denied calls pass dur=0; the counter still increments so dashboards can
// graph authz-rejections.
func RecordToolCall(server, tool, outcome string, dur time.Duration) {
	toolCallsTotal.WithLabelValues(server, tool, outcome).Inc()
	if outcome != OutcomeDenied {
		toolCallDuration.WithLabelValues(server, tool, outcome).Observe(dur.Seconds())
	}
}

// CounterValue returns the current tool-calls counter value. Exposed so
// handler-level tests in sibling packages can assert metric emission
// without reaching into the Prometheus default registry directly.
func CounterValue(server, tool, outcome string) float64 {
	return testutil.ToFloat64(toolCallsTotal.WithLabelValues(server, tool, outcome))
}
