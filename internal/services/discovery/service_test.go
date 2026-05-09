package discovery_test

import (
	"context"
	"testing"

	"github.com/amerfu/pllm/internal/core/models"
	"github.com/amerfu/pllm/internal/infrastructure/testutil"
	"github.com/amerfu/pllm/internal/services/discovery"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestService_Ingest_CreatesAgentAndCalls(t *testing.T) {
	if testing.Short() {
		t.Skip("requires testcontainer DB")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	svc := discovery.NewService(db, zap.NewNop(), nil, nil, nil)

	keyID := uuid.New()
	req := discovery.ExportTraceServiceRequest{
		ResourceSpans: []discovery.ResourceSpans{{
			Resource: discovery.Resource{Attributes: []discovery.KeyValue{
				kv("service.name", "fraud-investigator"),
				kv("service.version", "0.1.0"),
				kv("pllm.key_id", keyID.String()),
			}},
			ScopeSpans: []discovery.ScopeSpans{{
				Spans: []discovery.Span{
					{Name: "openai.chat", Kind: discovery.SpanKindClient, Attributes: []discovery.KeyValue{
						kv("gen_ai.request.model", "gpt-4o"),
					}},
					{Name: "qdrant.search", Kind: discovery.SpanKindClient, Attributes: []discovery.KeyValue{
						kv("db.system", "qdrant"),
						kv("db.collection.name", "customers"),
					}},
				},
			}},
		}},
	}

	require.NoError(t, svc.Ingest(context.Background(), req))

	var agents []models.DiscoveredAgent
	require.NoError(t, db.Find(&agents).Error)
	require.Len(t, agents, 1)
	assert.Equal(t, "fraud-investigator", agents[0].ServiceName)
	assert.Equal(t, models.DiscoveredStatusPending, agents[0].Status)
	require.NotNil(t, agents[0].KeyID)
	assert.Equal(t, keyID, *agents[0].KeyID)

	var calls []models.DiscoveredCall
	require.NoError(t, db.Where("discovered_agent_id = ?", agents[0].ID).Find(&calls).Error)
	require.Len(t, calls, 2)
}

func TestService_Ingest_UpsertsRepeats(t *testing.T) {
	if testing.Short() {
		t.Skip("requires testcontainer DB")
	}
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()
	svc := discovery.NewService(db, zap.NewNop(), nil, nil, nil)

	build := func() discovery.ExportTraceServiceRequest {
		return discovery.ExportTraceServiceRequest{
			ResourceSpans: []discovery.ResourceSpans{{
				Resource: discovery.Resource{Attributes: []discovery.KeyValue{
					kv("service.name", "fraud-investigator"),
				}},
				ScopeSpans: []discovery.ScopeSpans{{
					Spans: []discovery.Span{
						{Name: "openai.chat", Kind: discovery.SpanKindClient, Attributes: []discovery.KeyValue{
							kv("gen_ai.request.model", "gpt-4o"),
						}},
					},
				}},
			}},
		}
	}

	require.NoError(t, svc.Ingest(context.Background(), build()))
	require.NoError(t, svc.Ingest(context.Background(), build()))

	var calls []models.DiscoveredCall
	require.NoError(t, db.Find(&calls).Error)
	require.Len(t, calls, 1)
	assert.Equal(t, 2, calls[0].Count)
}
