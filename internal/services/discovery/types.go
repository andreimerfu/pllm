// Package discovery ingests OpenTelemetry traces and turns observed
// agent activity into governable RegistryAgent candidates. The types
// here mirror the OTLP/HTTP JSON wire format (subset).
package discovery

// ExportTraceServiceRequest is the top-level OTLP/HTTP body. We only
// decode `resourceSpans`; everything else is ignored.
type ExportTraceServiceRequest struct {
	ResourceSpans []ResourceSpans `json:"resourceSpans"`
}

type ResourceSpans struct {
	Resource   Resource     `json:"resource"`
	ScopeSpans []ScopeSpans `json:"scopeSpans"`
}

type Resource struct {
	Attributes []KeyValue `json:"attributes"`
}

type ScopeSpans struct {
	Scope InstrumentationScope `json:"scope"`
	Spans []Span               `json:"spans"`
}

type InstrumentationScope struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

// SpanKind values per OTel spec (only Client matters to us — LLM/HTTP/DB
// instrumentations all emit Client spans).
type SpanKind int

const (
	SpanKindUnspecified SpanKind = 0
	SpanKindInternal    SpanKind = 1
	SpanKindServer      SpanKind = 2
	SpanKindClient      SpanKind = 3
	SpanKindProducer    SpanKind = 4
	SpanKindConsumer    SpanKind = 5
)

type Span struct {
	Name              string     `json:"name"`
	Kind              SpanKind   `json:"kind"`
	StartTimeUnixNano string     `json:"startTimeUnixNano"`
	EndTimeUnixNano   string     `json:"endTimeUnixNano"`
	Attributes        []KeyValue `json:"attributes"`
}

type KeyValue struct {
	Key   string   `json:"key"`
	Value AnyValue `json:"value"`
}

// AnyValue mirrors OTLP's oneof. We only read string/int/bool/double —
// arrays and kvlists are flattened to their JSON repr if seen.
type AnyValue struct {
	StringValue *string  `json:"stringValue,omitempty"`
	BoolValue   *bool    `json:"boolValue,omitempty"`
	IntValue    *string  `json:"intValue,omitempty"` // OTLP uses string for int64
	DoubleValue *float64 `json:"doubleValue,omitempty"`
}

// AsString collapses an AnyValue to its scalar string form. Empty if absent.
func (v AnyValue) AsString() string {
	if v.StringValue != nil {
		return *v.StringValue
	}
	if v.IntValue != nil {
		return *v.IntValue
	}
	if v.BoolValue != nil {
		if *v.BoolValue {
			return "true"
		}
		return "false"
	}
	return ""
}

// Lookup returns the first matching attribute or empty string.
func Lookup(attrs []KeyValue, key string) string {
	for _, kv := range attrs {
		if kv.Key == key {
			return kv.Value.AsString()
		}
	}
	return ""
}
