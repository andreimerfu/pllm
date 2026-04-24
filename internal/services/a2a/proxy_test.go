package a2a

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestProxyInvokeHappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/invoke", r.URL.Path)
		require.Equal(t, "POST", r.Method)
		body, _ := io.ReadAll(r.Body)
		var in InvokeRequest
		require.NoError(t, json.Unmarshal(body, &in))
		require.Equal(t, "t-42", in.TaskID)
		_ = json.NewEncoder(w).Encode(InvokeResponse{Output: "ECHO: " + in.Input.(string)})
	}))
	defer srv.Close()

	p := NewProxy(2 * time.Second)
	res, err := p.Invoke(context.Background(), srv.URL, InvokeRequest{
		Input:  "hello",
		TaskID: "t-42",
	})
	require.NoError(t, err)
	require.Equal(t, "ECHO: hello", res.Output)
	require.Empty(t, res.Error)
}

func TestProxyInvokeErrorsOnNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	p := NewProxy(2 * time.Second)
	_, err := p.Invoke(context.Background(), srv.URL, InvokeRequest{Input: "x", TaskID: "t-1"})
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "500"))
}

func TestProxyInvokePropagatesContextTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_ = json.NewEncoder(w).Encode(InvokeResponse{Output: "slow"})
	}))
	defer srv.Close()

	p := NewProxy(50 * time.Millisecond)
	_, err := p.Invoke(context.Background(), srv.URL, InvokeRequest{Input: "x", TaskID: "t-1"})
	require.Error(t, err)
}
