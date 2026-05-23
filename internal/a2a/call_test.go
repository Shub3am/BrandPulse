// call_test.go drives Call against a real agent served by newMux, so the two
// halves of this package are proved against each other rather than against a
// hand-written fixture of what the wire is assumed to look like.
//
// It reuses echoInput, MentionBatch and echoHandler from serve_test.go.
package a2a

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCallRoundTripsAnAgentsArtifact(t *testing.T) {
	server := serveTestAgent(t, echoHandler{})
	t.Setenv(peerURLEnv, server.URL+"?agent="+agentPlaceholder)

	var out MentionBatch
	if err := Call(context.Background(), "bp-collector", echoInput{Brand: "boat", Limit: 2}, &out); err != nil {
		t.Fatalf("Call: %v", err)
	}

	if out.Brand != "boat" {
		t.Errorf("Brand = %q, want %q; the input did not reach the handler", out.Brand, "boat")
	}
	if len(out.Mentions) != 2 {
		t.Errorf("Mentions = %v, want the two the handler returns", out.Mentions)
	}
}

func TestCallReportsAPeersTaskFailure(t *testing.T) {
	// A failed task is an error here. The orchestrator turns that into
	// DegradedReason; this package does not decide which peers are optional.
	reason := "every source probe timed out"
	server := serveTestAgent(t, echoHandler{err: errors.New(reason)})
	t.Setenv(peerURLEnv, server.URL+"?agent="+agentPlaceholder)

	var out MentionBatch
	err := Call(context.Background(), "bp-collector", echoInput{Brand: "boat"}, &out)
	if err == nil {
		t.Fatal("Call returned nil for a task that failed")
	}
	if !strings.Contains(err.Error(), "bp-collector") || !strings.Contains(err.Error(), reason) {
		t.Errorf("the error is %q, and it must name the peer and the reason it gave", err)
	}
}

func TestCallReplaysTheInboundAgentToken(t *testing.T) {
	// The token is per request, so it threads through ctx. DEPLOY-NOTES
	// Finding 5 calls this an inference to confirm on the first live call;
	// this pins the half that is ours.
	var seen string
	agent := newMux(testCard(), echoHandler{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get(agentTokenHeader)
		agent.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	t.Setenv(peerURLEnv, server.URL+"?agent="+agentPlaceholder)

	ctx := WithAgentToken(context.Background(), "jwt-from-the-inbound-request")
	var out MentionBatch
	if err := Call(ctx, "bp-collector", echoInput{Brand: "boat"}, &out); err != nil {
		t.Fatalf("Call: %v", err)
	}
	if seen != "jwt-from-the-inbound-request" {
		t.Errorf("the peer saw %s = %q, want the caller's own token", agentTokenHeader, seen)
	}
}

func TestCallWithoutATokenSendsNoHeader(t *testing.T) {
	// A local compose run has no JWT_SECRET on the control plane and so no
	// token. An empty header would look like a forged one.
	var present bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, present = r.Header[http.CanonicalHeaderKey(agentTokenHeader)]
		http.Error(w, "not the point of this test", http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)
	t.Setenv(peerURLEnv, server.URL+"?agent="+agentPlaceholder)

	var out MentionBatch
	if err := Call(context.Background(), "bp-collector", echoInput{}, &out); err == nil {
		t.Fatal("the stub answered 500 and Call reported success")
	}
	if present {
		t.Errorf("Call sent %s with no token on the context", agentTokenHeader)
	}
}

func TestPeerURL(t *testing.T) {
	cases := []struct {
		name     string
		template string
		agent    string
		want     string
		wantErr  string
	}{
		{
			name:     "the template carries the placeholder",
			template: "https://plane.example/api/agents/{agent}",
			agent:    "bp-collector",
			want:     "https://plane.example/api/agents/bp-collector",
		},
		{
			name:     "a compose network addresses the peer by service name",
			template: "http://{agent}:8000/",
			agent:    "bp-detector",
			want:     "http://bp-detector:8000/",
		},
		{
			name:     "an unset template names the variable to set",
			template: "",
			agent:    "bp-collector",
			wantErr:  peerURLEnv,
		},
		{
			name:     "a template without the placeholder is refused",
			template: "https://plane.example/api/agents/",
			agent:    "bp-collector",
			wantErr:  agentPlaceholder,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv(peerURLEnv, c.template)
			got, err := peerURL(c.agent)
			if c.wantErr != "" {
				if err == nil {
					t.Fatalf("peerURL = %q, want an error naming %q", got, c.wantErr)
				}
				if !strings.Contains(err.Error(), c.wantErr) {
					t.Errorf("the error is %q, and it must name %q", err, c.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("peerURL: %v", err)
			}
			if got != c.want {
				t.Errorf("peerURL = %q, want %q", got, c.want)
			}
		})
	}
}
