package discovery_test

import (
	"testing"

	"github.com/amerfu/pllm/internal/services/discovery"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func sptr(s string) *string { return &s }

func kv(k, v string) discovery.KeyValue {
	return discovery.KeyValue{Key: k, Value: discovery.AnyValue{StringValue: sptr(v)}}
}

func TestProcessResourceSpans_ExtractsAgentAndCalls(t *testing.T) {
	keyID := uuid.New().String()
	rs := discovery.ResourceSpans{
		Resource: discovery.Resource{Attributes: []discovery.KeyValue{
			kv("service.name", "fraud-investigator"),
			kv("service.version", "0.1.0"),
			kv("pllm.key_id", keyID),
		}},
		ScopeSpans: []discovery.ScopeSpans{{
			Scope: discovery.InstrumentationScope{Name: "opentelemetry.instrumentation.openai"},
			Spans: []discovery.Span{
				{Name: "openai.chat", Kind: discovery.SpanKindClient, Attributes: []discovery.KeyValue{
					kv("gen_ai.system", "openai"),
					kv("gen_ai.request.model", "gpt-4o"),
				}},
				{Name: "openai.chat", Kind: discovery.SpanKindClient, Attributes: []discovery.KeyValue{
					kv("gen_ai.system", "openai"),
					kv("gen_ai.request.model", "gpt-4o"),
				}},
				{Name: "qdrant.search", Kind: discovery.SpanKindClient, Attributes: []discovery.KeyValue{
					kv("db.system", "qdrant"),
					kv("db.collection.name", "customer-embeddings"),
				}},
				{Name: "GET", Kind: discovery.SpanKindClient, Attributes: []discovery.KeyValue{
					kv("http.url", "https://internal.example.com/api/v1/data"),
					kv("http.method", "GET"),
				}},
			},
		}},
	}

	out := discovery.ProcessResourceSpans(rs)

	assert.Equal(t, "fraud-investigator", out.Agent.ServiceName)
	assert.Equal(t, "0.1.0", out.Agent.ServiceVersion)
	assert.Equal(t, keyID, out.Agent.KeyIDRaw)

	require := func(kind discovery.CallKindStr, target string, count int) {
		for _, c := range out.Calls {
			if c.Kind == kind && c.TargetName == target {
				assert.Equal(t, count, c.Count, "count for %s/%s", kind, target)
				return
			}
		}
		t.Fatalf("missing call %s/%s", kind, target)
	}
	require("llm", "gpt-4o", 2)
	require("vector_db", "qdrant://customer-embeddings", 1)
	require("http", "internal.example.com", 1)
}

func TestProcessResourceSpans_NoServiceNameReturnsEmpty(t *testing.T) {
	rs := discovery.ResourceSpans{}
	out := discovery.ProcessResourceSpans(rs)
	assert.Empty(t, out.Agent.ServiceName)
	assert.Empty(t, out.Calls)
}
