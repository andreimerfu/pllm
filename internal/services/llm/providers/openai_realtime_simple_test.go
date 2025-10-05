package providers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAIProvider_RealtimeSupport(t *testing.T) {
	provider, err := NewOpenAIProvider("test-provider", ProviderConfig{
		APIKey:  "test-key",
		BaseURL: "https://api.openai.com/v1",
	})
	require.NoError(t, err)

	// Test that OpenAI provider supports realtime
	assert.True(t, provider.SupportsRealtime())

	// Test that it implements the RealtimeProvider interface
	var _ RealtimeProvider = provider // Compile-time check that it implements the interface

	// Test that the helper function recognizes it as supporting realtime
	var providerInterface Provider = provider
	assert.True(t, ProviderSupportsRealtime(providerInterface))
}

func TestGetRealtimeProvider_OpenAI(t *testing.T) {
	provider, err := NewOpenAIProvider("test-provider", ProviderConfig{
		APIKey:  "test-key",
		BaseURL: "https://api.openai.com/v1",
	})
	require.NoError(t, err)

	// Test that GetRealtimeProvider can cast it properly
	var providerInterface Provider = provider
	rtProvider, err := GetRealtimeProvider(providerInterface)
	assert.NoError(t, err)
	assert.NotNil(t, rtProvider)
	assert.True(t, rtProvider.SupportsRealtime())
}