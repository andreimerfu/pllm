package use_cases

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

// fakeRemote implements remoteDecider with a canned result + call counter.
type fakeRemote struct {
	d     Decision
	err   error
	calls int
}

func (f *fakeRemote) AllowLLM(context.Context, *uuid.UUID, string) (Decision, error) {
	f.calls++
	return f.d, f.err
}
func (f *fakeRemote) AllowMCPTool(context.Context, *uuid.UUID, string) (Decision, error) {
	f.calls++
	return f.d, f.err
}
func (f *fakeRemote) AllowAgent(context.Context, *uuid.UUID, string) (Decision, error) {
	f.calls++
	return f.d, f.err
}
func (f *fakeRemote) AllowPrompt(context.Context, *uuid.UUID, string) (Decision, error) {
	f.calls++
	return f.d, f.err
}

// fakeLocal implements Decider with a canned result + call counter.
type fakeLocal struct {
	d     Decision
	calls int
}

func (f *fakeLocal) AllowLLM(context.Context, *uuid.UUID, string) Decision { f.calls++; return f.d }
func (f *fakeLocal) AllowMCPTool(context.Context, *uuid.UUID, string) Decision {
	f.calls++
	return f.d
}
func (f *fakeLocal) AllowAgent(context.Context, *uuid.UUID, string) Decision  { f.calls++; return f.d }
func (f *fakeLocal) AllowPrompt(context.Context, *uuid.UUID, string) Decision { f.calls++; return f.d }

func TestComposite_RemoteSuccessSkipsLocal(t *testing.T) {
	remote := &fakeRemote{d: Decision{Allowed: true, Reason: "pdp ok"}}
	local := &fakeLocal{d: Decision{Allowed: false}}
	c := NewCompositeDecider(remote, local, "fallback_local", nil)

	d := c.AllowLLM(context.Background(), ptr(uuid.New()), "gpt-4")
	if !d.Allowed || d.Reason != "pdp ok" {
		t.Fatalf("expected remote decision; got %+v", d)
	}
	if local.calls != 0 {
		t.Fatalf("local must not be consulted on remote success; calls=%d", local.calls)
	}
}

func TestComposite_FallbackLocalOnError(t *testing.T) {
	remote := &fakeRemote{err: errors.New("pdp down")}
	local := &fakeLocal{d: Decision{Allowed: true, EnforceMode: true, Reason: "local scope"}}
	c := NewCompositeDecider(remote, local, "fallback_local", nil)

	d := c.AllowMCPTool(context.Background(), ptr(uuid.New()), "filesystem")
	if local.calls != 1 {
		t.Fatalf("expected local fallback; calls=%d", local.calls)
	}
	if !d.Allowed || d.Reason != "local scope" {
		t.Fatalf("expected local decision; got %+v", d)
	}
}

func TestComposite_DenyOnError(t *testing.T) {
	remote := &fakeRemote{err: errors.New("pdp down")}
	local := &fakeLocal{d: Decision{Allowed: true}}
	c := NewCompositeDecider(remote, local, "deny", nil)

	d := c.AllowAgent(context.Background(), ptr(uuid.New()), "io.github.org/agent")
	if d.Allowed || !d.EnforceMode {
		t.Fatalf("deny mode should fail closed; got %+v", d)
	}
	if local.calls != 0 {
		t.Fatalf("deny mode must not consult local; calls=%d", local.calls)
	}
}

func TestComposite_AllowOnError(t *testing.T) {
	remote := &fakeRemote{err: errors.New("pdp down")}
	local := &fakeLocal{d: Decision{Allowed: false}}
	c := NewCompositeDecider(remote, local, "allow", nil)

	d := c.AllowPrompt(context.Background(), ptr(uuid.New()), "greeting")
	if !d.Allowed {
		t.Fatalf("allow mode should fail open; got %+v", d)
	}
	if local.calls != 0 {
		t.Fatalf("allow mode must not consult local; calls=%d", local.calls)
	}
}

func TestComposite_UnknownOnErrorDefaultsToFallback(t *testing.T) {
	remote := &fakeRemote{err: errors.New("pdp down")}
	local := &fakeLocal{d: Decision{Allowed: true, Reason: "local scope"}}
	c := NewCompositeDecider(remote, local, "bogus-mode", nil)

	d := c.AllowLLM(context.Background(), ptr(uuid.New()), "gpt-4")
	if local.calls != 1 || d.Reason != "local scope" {
		t.Fatalf("unknown mode should default to fallback_local; calls=%d decision=%+v", local.calls, d)
	}
}
