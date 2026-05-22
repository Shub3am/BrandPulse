// call.go is the client half of this package: one agent reaching another.
// bp-orchestrator is the only caller.
//
// It must not compile in a peer's address. Nothing Nasiko injects into a
// container carries one (docs/DEPLOY-NOTES.md Finding 5), so both the base URL
// and the routing shape arrive as deploy configuration, and the credential is a
// per-request value read off the request the caller is already serving.
//
// Like the rest of this package it must not know any agent's input or output
// type. in and out are opaque; the only thing touched here is encoding/json.
package a2a

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	a2aproto "github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2aclient"
)

// peerURLEnv holds a URL template carrying agentPlaceholder, set as a Nasiko
// vault-wide secret. The name is ours, not Nasiko's. Two shapes are in use:
//
//	deployed: https://<control-plane>/api/agents/{agent}
//	compose:  http://{agent}:8000/
//
// A template rather than a bare base because the control plane routes peer
// calls under /api/agents/ and a local compose network does not. Neither
// convention belongs in Go, and the one that is wrong is only wrong at runtime.
const peerURLEnv = "NASIKO_API_URL"

const agentPlaceholder = "{agent}"

// agentTokenHeader is the delegation JWT Nasiko mints and sends inbound to an
// agent. A peer call replays the caller's own, which is an inference from how
// the server mints and consumes it, not a documented contract: DEPLOY-NOTES
// Finding 5 says to confirm it on the first live two-agent call.
const agentTokenHeader = "x-nasiko-agent-token"

// peerCallTimeout matches NASIKO_FLOW_TIMEOUT_SECS, which the brief pins at
// 180. A client that waits longer than the flow guard only turns a rejected
// flow into a hung one.
const peerCallTimeout = 180 * time.Second

// CallFunc is Call's signature. bp-orchestrator holds it as a field so its
// pipeline tests answer every peer in memory.
type CallFunc func(ctx context.Context, agent string, in any, out any) error

// peerHTTPClient is shared across calls for its connection pool. It carries no
// credential: the token rides on ctx, so one client is safe for every peer and
// every inbound request.
var peerHTTPClient = &http.Client{
	Timeout:   peerCallTimeout,
	Transport: replayAgentToken{base: http.DefaultTransport},
}

type agentTokenKey struct{}

// WithAgentToken puts the inbound delegation JWT on ctx so a peer call made
// while serving this request can replay it. newMux calls it for every request;
// a test calls it directly.
func WithAgentToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, agentTokenKey{}, token)
}

// agentTokenFrom returns the empty string when there is none, which is normal:
// Nasiko mints the token only when the server holds JWT_SECRET, and a peer call
// without one is what a local compose run always does.
func agentTokenFrom(ctx context.Context) string {
	token, _ := ctx.Value(agentTokenKey{}).(string)
	return token
}

// replayAgentToken is a RoundTripper rather than a header set once at client
// construction because the token belongs to the inbound request being served,
// not to the process.
type replayAgentToken struct {
	base http.RoundTripper
}

func (r replayAgentToken) RoundTrip(request *http.Request) (*http.Response, error) {
	token := agentTokenFrom(request.Context())
	if token == "" {
		return r.base.RoundTrip(request)
	}
	// Clone, because a RoundTripper must not modify the request it is given.
	request = request.Clone(request.Context())
	request.Header.Set(agentTokenHeader, token)
	return r.base.RoundTrip(request)
}

// Call sends in to agent and decodes its single artifact into out.
//
// It returns an error rather than panicking. The signature already returns one,
// so the caller's existing error path absorbs it, and a panic here would be the
// worst failure this package can produce: taskFailure exists precisely so that
// trouble becomes a reported task state instead of a dead server process.
//
// A peer that failed its task is an error here too. The orchestrator turns that
// into RunRecord.DegradedReason and carries on; this package does not decide
// which peers are optional.
func Call(ctx context.Context, agent string, in any, out any) error {
	url, err := peerURL(agent)
	if err != nil {
		return err
	}

	endpoint := &a2aproto.AgentInterface{
		URL:             url,
		ProtocolBinding: a2aproto.TransportProtocolJSONRPC,
		ProtocolVersion: a2aproto.Version,
	}
	// NewFromEndpoints rather than NewFromCard: the peer's card is already in
	// this repository, and fetching it would add a round trip that can fail on
	// its own before a single mention has moved.
	client, err := a2aclient.NewFromEndpoints(ctx, []*a2aproto.AgentInterface{endpoint},
		a2aclient.WithJSONRPCTransport(peerHTTPClient))
	if err != nil {
		return fmt.Errorf("a2a: building a client for %s at %s: %w", agent, url, err)
	}
	defer client.Destroy()

	part := a2aproto.NewDataPart(in)
	part.MediaType = jsonMimeType
	request := &a2aproto.SendMessageRequest{Message: a2aproto.NewMessage(a2aproto.MessageRoleUser, part)}

	result, err := client.SendMessage(ctx, request)
	if err != nil {
		return fmt.Errorf("a2a: calling %s at %s: %w", agent, url, err)
	}

	task, ok := result.(*a2aproto.Task)
	if !ok {
		return fmt.Errorf("a2a: %s replied with %T, want a task; every agent here answers through Serve", agent, result)
	}
	if task.Status.State != a2aproto.TaskStateCompleted {
		return fmt.Errorf("a2a: %s ended in %s: %s", agent, task.Status.State, statusReason(task.Status))
	}
	return decodeArtifact(agent, task, out)
}

// peerURL substitutes the agent name into the deploy-time template.
func peerURL(agent string) (string, error) {
	if agent == "" {
		return "", fmt.Errorf("a2a: Call needs an agent name")
	}
	template := os.Getenv(peerURLEnv)
	if template == "" {
		return "", fmt.Errorf("a2a: %s is unset, so %s has no address; set it to a URL containing %s", peerURLEnv, agent, agentPlaceholder)
	}
	if !strings.Contains(template, agentPlaceholder) {
		return "", fmt.Errorf("a2a: %s is %q, which has no %s in it, so every peer would resolve to the same address", peerURLEnv, template, agentPlaceholder)
	}
	return strings.ReplaceAll(template, agentPlaceholder, agent), nil
}

// decodeArtifact unwraps the one application/json artifact Serve sends back.
//
// The counts are checked rather than indexed past, because a peer that returned
// nothing and a peer that returned the wrong thing are different bugs and the
// alternative to saying which is a nil map three frames further on.
func decodeArtifact(agent string, task *a2aproto.Task, out any) error {
	if len(task.Artifacts) != 1 {
		return fmt.Errorf("a2a: %s returned %d artifacts, want exactly 1", agent, len(task.Artifacts))
	}
	parts := task.Artifacts[0].Parts
	if len(parts) != 1 {
		return fmt.Errorf("a2a: %s returned an artifact with %d parts, want exactly 1", agent, len(parts))
	}

	body, err := jsonFromPart(parts[0])
	if err != nil {
		return fmt.Errorf("a2a: reading %s's artifact: %w", agent, err)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("a2a: decoding %s's artifact into %T: %w", agent, out, err)
	}
	return nil
}

// statusReason pulls the sentence taskFailure put in the status message, so the
// orchestrator's log says why a peer failed instead of only that it did.
func statusReason(status a2aproto.TaskStatus) string {
	if status.Message == nil {
		return "no reason given"
	}
	reasons := make([]string, 0, len(status.Message.Parts))
	for _, part := range status.Message.Parts {
		if text, ok := part.Content.(a2aproto.Text); ok {
			reasons = append(reasons, string(text))
		}
	}
	if len(reasons) == 0 {
		return "no reason given"
	}
	return strings.Join(reasons, "; ")
}
