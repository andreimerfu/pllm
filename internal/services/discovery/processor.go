package discovery

import (
	"net/url"
	"strings"
)

// CallKindStr is a typed string mirror of models.DiscoveredCallKind.
// We keep the discovery package independent of `core/models` so it
// stays unit-testable; the service-layer wrapper translates.
type CallKindStr string

const (
	CallKindLLM      CallKindStr = "llm"
	CallKindMCP      CallKindStr = "mcp"
	CallKindVectorDB CallKindStr = "vector_db"
	CallKindHTTP     CallKindStr = "http"
	CallKindUnknown  CallKindStr = "unknown"
)

// AgentFingerprint is the minimum identity we extract from OTel
// resource attributes. KeyIDRaw is the raw string from the
// `pllm.key_id` attribute — the service layer parses it to uuid.UUID.
type AgentFingerprint struct {
	ServiceName    string
	ServiceVersion string
	KeyIDRaw       string
}

// CallObservation is one (kind, target) edge with its repeat count
// inside the current OTLP request. Counts get added to existing rows
// during persistence, not replaced.
type CallObservation struct {
	Kind       CallKindStr
	TargetName string
	Count      int
}

// ProcessedSpans is the output of one call to ProcessResourceSpans.
type ProcessedSpans struct {
	Agent AgentFingerprint
	Calls []CallObservation
}

// ProcessResourceSpans walks one ResourceSpans block and yields the
// agent fingerprint plus a deduplicated call list. Spans without a
// recognized target are reported as Unknown.
//
// The classification rules, in order:
//
//	gen_ai.system OR gen_ai.request.model            -> llm  (target = model)
//	pllm.mcp.server attribute                        -> mcp  (target = server slug)
//	db.system in {qdrant,pinecone,weaviate,chroma}   -> vector_db
//	http.url OR url.full                             -> http (target = host)
//	otherwise                                        -> unknown (target = span.name)
func ProcessResourceSpans(rs ResourceSpans) ProcessedSpans {
	out := ProcessedSpans{
		Agent: AgentFingerprint{
			ServiceName:    Lookup(rs.Resource.Attributes, "service.name"),
			ServiceVersion: Lookup(rs.Resource.Attributes, "service.version"),
			KeyIDRaw:       Lookup(rs.Resource.Attributes, "pllm.key_id"),
		},
	}
	if out.Agent.ServiceName == "" {
		return out
	}

	dedup := map[string]*CallObservation{}
	add := func(kind CallKindStr, target string) {
		if target == "" {
			return
		}
		k := string(kind) + "|" + target
		if existing, ok := dedup[k]; ok {
			existing.Count++
			return
		}
		dedup[k] = &CallObservation{Kind: kind, TargetName: target, Count: 1}
	}

	for _, ss := range rs.ScopeSpans {
		for _, sp := range ss.Spans {
			if sp.Kind != SpanKindClient && sp.Kind != SpanKindUnspecified {
				continue
			}
			kind, target := classify(sp)
			add(kind, target)
		}
	}

	out.Calls = make([]CallObservation, 0, len(dedup))
	for _, c := range dedup {
		out.Calls = append(out.Calls, *c)
	}
	return out
}

func classify(sp Span) (CallKindStr, string) {
	if model := Lookup(sp.Attributes, "gen_ai.request.model"); model != "" {
		return CallKindLLM, model
	}
	if Lookup(sp.Attributes, "gen_ai.system") != "" {
		return CallKindLLM, sp.Name
	}
	if mcp := Lookup(sp.Attributes, "pllm.mcp.server"); mcp != "" {
		return CallKindMCP, mcp
	}
	if dbSys := Lookup(sp.Attributes, "db.system"); dbSys != "" {
		switch strings.ToLower(dbSys) {
		case "qdrant", "pinecone", "weaviate", "chroma", "milvus":
			coll := Lookup(sp.Attributes, "db.collection.name")
			if coll == "" {
				coll = Lookup(sp.Attributes, "db.name")
			}
			if coll != "" {
				return CallKindVectorDB, dbSys + "://" + coll
			}
			return CallKindVectorDB, dbSys
		}
	}
	if u := Lookup(sp.Attributes, "http.url"); u != "" {
		return CallKindHTTP, hostOf(u)
	}
	if u := Lookup(sp.Attributes, "url.full"); u != "" {
		return CallKindHTTP, hostOf(u)
	}
	if h := Lookup(sp.Attributes, "server.address"); h != "" {
		return CallKindHTTP, h
	}
	return CallKindUnknown, sp.Name
}

func hostOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return rawURL
	}
	return u.Host
}
