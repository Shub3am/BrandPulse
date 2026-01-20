// Package a2a wires a plain Go handler to the A2A protocol, so every agent's
// main() is the same twenty lines and nobody hand-rolls the artifact envelope.
//
// # The error rule, stated once
//
// A malformed input part is an A2A task failure. A non-nil error out of Handle
// is also a task failure. An agent that hit trouble but still has a partial
// answer does not return an error: it returns its normal output struct with
// Errors populated and a nil error. A dead source must not kill a run.
//
// The four agents whose output is a bare domain type have no Errors field and
// are not getting one, so they do fail the task. The orchestrator records that
// in RunRecord.DegradedReason and carries on.
//
// # What this package must not do
//
// It must not know a single agent's input or output type. Everything here is
// generic over In and Out, and the one place a concrete type is touched is
// json.Unmarshal.
package a2a

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"net/http"
	"os"

	a2aproto "github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// healthPath is not part of A2A and Nasiko does not probe it. It is here for
// B7's smoke tests and for a human with curl, because the alternative is
// reading a JSON-RPC error to find out whether a process is alive.
const healthPath = "/healthz"

// defaultPort is what Serve listens on when PORT is unset. Nasiko always
// injects PORT; this is so `go run ./agents/bp-collector` works locally.
const defaultPort = "8080"

// Handler is what every agent implements: one method, its own input type in,
// its own output type out. No agent implements the SDK's executor interface
// directly.
type Handler[In, Out any] interface {
	Handle(ctx context.Context, in In) (Out, error)
}

// Artifact is the single application/json artifact an agent replies with.
type Artifact struct {
	MimeType string `json:"mimeType"`
	Name     string `json:"name"`
	Body     []byte `json:"body"`
}

// Serve reads PORT, registers the health path, wraps the mux in otelhttp and
// blocks.
//
// The type parameters are inferred from h, so the call site is
// a2a.Serve(card, handler) with no type arguments written out and every
// agent's main() keeps the shape docs/CONTRACTS.md §3 prints.
//
// JSON-RPC is mounted at "/" so it answers on whatever path the card
// advertises. The more specific patterns above it still win: Go's ServeMux
// matches the longest pattern, not the first registered.
func Serve[In, Out any](card Card, h Handler[In, Out]) error {
	if err := validateCard(card); err != nil {
		return fmt.Errorf("a2a: refusing to serve %s: %w", card.Name, err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}
	return http.ListenAndServe(":"+port, newMux(card, h))
}

// newMux is everything Serve does except bind a socket, which is the whole of
// what serve_test.go can drive through httptest.
func newMux[In, Out any](card Card, h Handler[In, Out]) http.Handler {
	mux := http.NewServeMux()
	mux.Handle(a2asrv.WellKnownAgentCardPath, a2asrv.NewStaticAgentCardHandler(&card))
	mux.HandleFunc(healthPath, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.Handle("/", a2asrv.NewJSONRPCHandler(a2asrv.NewHandler(executorFor(h))))

	// otelhttp goes on once, here, rather than in nine main()s. internal/obs
	// builds the provider; this is the only thing in the repository that
	// instruments a handler.
	return otelhttp.NewHandler(mux, card.Name)
}

// executorFor adapts one Handle call to the SDK's event stream.
//
// The sequence the SDK expects is submitted, working, artifact, completed, and
// it stops consuming at the first terminal state. Failure is a status update
// in TaskStateFailed, not an error out of yield: yield's error is reserved for
// trouble before a task exists, and by then one has.
func executorFor[In, Out any](h Handler[In, Out]) a2asrv.AgentExecutorFunc {
	return func(ctx context.Context, execCtx *a2asrv.ExecutorContext) iter.Seq2[a2aproto.Event, error] {
		return func(yield func(a2aproto.Event, error) bool) {
			if execCtx.StoredTask == nil {
				if !yield(a2aproto.NewSubmittedTask(execCtx, execCtx.Message), nil) {
					return
				}
			}
			if !yield(a2aproto.NewStatusUpdateEvent(execCtx, a2aproto.TaskStateWorking, nil), nil) {
				return
			}

			artifact, err := run(ctx, h, execCtx.Message)
			if err != nil {
				yield(taskFailure(execCtx, err), nil)
				return
			}
			if !yield(&a2aproto.TaskArtifactUpdateEvent{
				Artifact:  artifact,
				TaskID:    execCtx.TaskID,
				ContextID: execCtx.ContextID,
				LastChunk: true,
			}, nil) {
				return
			}

			yield(a2aproto.NewStatusUpdateEvent(execCtx, a2aproto.TaskStateCompleted, nil), nil)
		}
	}
}

// run is the whole non-protocol part of an execution: decode, hand to the
// agent, wrap the answer. It is separate from executorFor so the three ways it
// can fail collapse into one error return instead of three copies of the
// failure event.
func run[In, Out any](ctx context.Context, h Handler[In, Out], message *a2aproto.Message) (*a2aproto.Artifact, error) {
	in, err := decodeInput[In](message)
	if err != nil {
		return nil, err
	}
	out, err := h.Handle(ctx, in)
	if err != nil {
		return nil, err
	}
	artifact, err := JSONArtifact(typeName[Out](), out)
	if err != nil {
		return nil, err
	}
	return artifact.toParts()
}

// decodeInput pulls the agent's input out of the request's single part.
//
// Three of the four part kinds carry JSON and all three are accepted: the
// orchestrator sends data, a recorded fixture replays raw, and a human with
// curl sends text. A url part is not fetched, because an agent that follows a
// URL out of a request is an agent that can be asked to fetch anything.
func decodeInput[In any](message *a2aproto.Message) (In, error) {
	var in In
	if message == nil || len(message.Parts) == 0 {
		return in, fmt.Errorf("a2a: the request carries no parts")
	}
	if len(message.Parts) != 1 {
		return in, fmt.Errorf("a2a: the request carries %d parts, want exactly 1", len(message.Parts))
	}

	part := message.Parts[0]
	var body []byte
	switch content := part.Content.(type) {
	case a2aproto.Data:
		// Data.Value arrived as map[string]any, so it has to go back through
		// json.Marshal before it can land in a typed struct.
		marshalled, err := json.Marshal(content.Value)
		if err != nil {
			return in, fmt.Errorf("a2a: re-encode the data part: %w", err)
		}
		body = marshalled
	case a2aproto.Raw:
		body = content
	case a2aproto.Text:
		body = []byte(content)
	default:
		return in, fmt.Errorf("a2a: the part carries %T, want JSON as data, raw or text", part.Content)
	}

	if err := json.Unmarshal(body, &in); err != nil {
		return in, fmt.Errorf("a2a: the part is not a %s: %w", typeName[In](), err)
	}
	return in, nil
}

// taskFailure carries the reason into the task status, because an alert that
// says only "failed" costs whoever is on call the whole debugging session.
func taskFailure(execCtx *a2asrv.ExecutorContext, err error) a2aproto.Event {
	reason := a2aproto.NewMessage(a2aproto.MessageRoleAgent, a2aproto.NewTextPart(err.Error()))
	return a2aproto.NewStatusUpdateEvent(execCtx, a2aproto.TaskStateFailed, reason)
}

// Call reaches a peer agent through the Nasiko proxy. bp-orchestrator is the
// only caller.
//
// No agent constructs a peer URL: the proxy address and the routing header are
// Nasiko's, and a hardcoded one breaks on the next redeploy.
//
// STUB: B1 Task 11 does not cover this and nothing calls it yet. See the
// blocker row in HACKATHON_NOTES; whoever writes bp-orchestrator needs it.
func Call(ctx context.Context, agent string, in any, out any) error {
	panic("not implemented")
}
