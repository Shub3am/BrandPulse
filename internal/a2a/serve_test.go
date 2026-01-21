// serve_test.go drives the real JSON-RPC surface over httptest, not the
// executor in isolation.
//
// The envelope is what DronaHQ and bp-orchestrator bind to, so the assertions
// are on bytes off the wire. A test that called executorFor directly would
// pass even if the SDK serialised the artifact into a shape nobody can read.
package a2a

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	a2aproto "github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
)

// echoInput and MentionBatch stand in for a real agent's types. MentionBatch
// is named after the real one on purpose: the artifact name comes from the Go
// type, so the name in the assertions is the name the type gives it.
type echoInput struct {
	Brand string `json:"brand"`
	Limit int    `json:"limit"`
}

type MentionBatch struct {
	Brand    string   `json:"brand"`
	Mentions []string `json:"mentions"`
	Errors   []string `json:"errors,omitempty"`
}

type echoHandler struct {
	err error
}

func (h echoHandler) Handle(_ context.Context, in echoInput) (MentionBatch, error) {
	if h.err != nil {
		return MentionBatch{}, h.err
	}
	return MentionBatch{Brand: in.Brand, Mentions: []string{"one", "two"}}, nil
}

func TestAHandlerProducesExactlyOneJSONArtifact(t *testing.T) {
	server := serveTestAgent(t, echoHandler{})

	task := sendMessage(t, server, dataPart(t, echoInput{Brand: "boat", Limit: 2}))

	part := task.firstArtifactPart(t)
	if got := task.Artifacts[0].Name; got != "MentionBatch" {
		t.Errorf("artifact name is %q, want %q; DronaHQ keys its bindings on it", got, "MentionBatch")
	}
	if part.MediaType != "application/json" {
		t.Errorf("part mediaType is %q, want application/json", part.MediaType)
	}
	if task.Status.State != string(a2aproto.TaskStateCompleted) {
		t.Errorf("task state is %q, want %q", task.Status.State, a2aproto.TaskStateCompleted)
	}
}

func TestTheArtifactBodyIsInlineJSONAndNotBase64(t *testing.T) {
	// This is the one assertion DronaHQ's bindings live or die on. A Raw part
	// would serialise the output struct as a base64 string under "raw", which
	// still passes a mimeType check and still says application/json, and then
	// nothing in the dashboard can reach a field.
	server := serveTestAgent(t, echoHandler{})

	task := sendMessage(t, server, dataPart(t, echoInput{Brand: "boat"}))

	part := task.firstArtifactPart(t)
	if part.Raw != "" {
		t.Fatalf("the part came back as base64 under \"raw\": %q", part.Raw)
	}
	if len(part.Data) == 0 {
		t.Fatalf("the part has no \"data\" key: %s", task.raw)
	}

	var batch MentionBatch
	if err := json.Unmarshal(part.Data, &batch); err != nil {
		t.Fatalf("the data part does not decode as a MentionBatch: %v\n%s", err, part.Data)
	}
	if batch.Brand != "boat" || len(batch.Mentions) != 2 {
		t.Errorf("decoded %+v, want the brand and two mentions the handler returned", batch)
	}
}

func TestAMalformedInputPartIsATaskFailure(t *testing.T) {
	server := serveTestAgent(t, echoHandler{})

	// A number where the struct wants an object. The agent is never reached.
	task := sendMessage(t, server, rawJSONPart(t, `"not an object"`))

	if task.Status.State != string(a2aproto.TaskStateFailed) {
		t.Fatalf("task state is %q, want %q: %s", task.Status.State, a2aproto.TaskStateFailed, task.raw)
	}
	if len(task.Artifacts) != 0 {
		t.Errorf("a failed task carries %d artifacts, want none", len(task.Artifacts))
	}
	if !strings.Contains(task.reason(), "echoInput") {
		t.Errorf("the failure reason is %q, and it must name the type that did not parse", task.reason())
	}
}

func TestAnErrorOutOfHandleIsATaskFailure(t *testing.T) {
	// The other half of the error rule. A partial answer comes back as a
	// normal output with Errors populated and a nil error; a non-nil error
	// means the agent has nothing, and that is a task failure.
	server := serveTestAgent(t, echoHandler{err: errors.New("every source probe timed out")})

	task := sendMessage(t, server, dataPart(t, echoInput{Brand: "boat"}))

	if task.Status.State != string(a2aproto.TaskStateFailed) {
		t.Fatalf("task state is %q, want %q: %s", task.Status.State, a2aproto.TaskStateFailed, task.raw)
	}
	if !strings.Contains(task.reason(), "every source probe timed out") {
		t.Errorf("the failure reason is %q; it must carry the agent's own message, or whoever is on call has nothing to go on", task.reason())
	}
}

func TestTheHealthPathAnswers(t *testing.T) {
	server := serveTestAgent(t, echoHandler{})

	response, err := http.Get(server.URL + healthPath)
	if err != nil {
		t.Fatalf("GET %s: %v", healthPath, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Errorf("GET %s returned %d, want 200", healthPath, response.StatusCode)
	}
}

func TestTheCardIsServedAtTheWellKnownPath(t *testing.T) {
	// Mounting JSON-RPC at "/" must not shadow this. ServeMux matches the
	// longest pattern rather than the first registered, and this is the test
	// that says so out loud.
	server := serveTestAgent(t, echoHandler{})

	response, err := http.Get(server.URL + a2asrv.WellKnownAgentCardPath)
	if err != nil {
		t.Fatalf("GET %s: %v", a2asrv.WellKnownAgentCardPath, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET %s returned %d, want 200", a2asrv.WellKnownAgentCardPath, response.StatusCode)
	}

	var card Card
	if err := json.NewDecoder(response.Body).Decode(&card); err != nil {
		t.Fatalf("decode the served card: %v", err)
	}
	if card.Name != "bp-test" {
		t.Errorf("the served card is named %q, want bp-test", card.Name)
	}
}

func TestDecodeInputAcceptsEveryJSONBearingPart(t *testing.T) {
	// data is what the orchestrator sends, raw is what a recorded fixture
	// replays, text is what a human with curl sends.
	want := echoInput{Brand: "boat", Limit: 7}
	body, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}

	cases := map[string]*a2aproto.Part{
		"data": a2aproto.NewDataPart(json.RawMessage(body)),
		"raw":  a2aproto.NewRawPart(body),
		"text": a2aproto.NewTextPart(string(body)),
	}

	for kind, part := range cases {
		t.Run(kind, func(t *testing.T) {
			got, err := decodeInput[echoInput](a2aproto.NewMessage(a2aproto.MessageRoleUser, part))
			if err != nil {
				t.Fatalf("decodeInput from a %s part: %v", kind, err)
			}
			if got != want {
				t.Errorf("decodeInput = %+v, want %+v", got, want)
			}
		})
	}
}

func TestDecodeInputRefusesAURLPart(t *testing.T) {
	// An agent that fetches a URL out of a request is an agent that can be
	// asked to fetch anything, including the metadata endpoint.
	part := a2aproto.NewFileURLPart("http://169.254.169.254/latest/meta-data/", "application/json")

	_, err := decodeInput[echoInput](a2aproto.NewMessage(a2aproto.MessageRoleUser, part))
	if err == nil {
		t.Fatal("decodeInput accepted a url part")
	}
	if !strings.Contains(err.Error(), "want JSON as data, raw or text") {
		t.Errorf("the error is %q, and it must say which part kinds are allowed", err)
	}
}

func TestDecodeInputRefusesMoreThanOnePart(t *testing.T) {
	message := a2aproto.NewMessage(a2aproto.MessageRoleUser,
		a2aproto.NewTextPart(`{"brand":"boat"}`),
		a2aproto.NewTextPart(`{"brand":"noise"}`),
	)

	_, err := decodeInput[echoInput](message)
	if err == nil {
		t.Fatal("decodeInput accepted two parts and silently picked one")
	}
}

func TestJSONArtifactRefusesAnEmptyName(t *testing.T) {
	if _, err := JSONArtifact("", MentionBatch{}); err == nil {
		t.Fatal("JSONArtifact accepted an empty name; DronaHQ binds on it")
	}
}

func TestTypeNameUnwrapsAPointer(t *testing.T) {
	if got := typeName[*MentionBatch](); got != "MentionBatch" {
		t.Errorf("typeName[*MentionBatch] = %q, want MentionBatch", got)
	}
}

// --- test plumbing ---

func testCard() Card {
	return Card{
		Name:        "bp-test",
		Description: "a test agent",
		Version:     "0.1.0",
		SupportedInterfaces: []*a2aproto.AgentInterface{
			a2aproto.NewAgentInterface("http://127.0.0.1:0/", a2aproto.TransportProtocolJSONRPC),
		},
	}
}

func serveTestAgent[In, Out any](t *testing.T, h Handler[In, Out]) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(newMux(testCard(), h))
	t.Cleanup(server.Close)
	return server
}

func dataPart(t *testing.T, v any) json.RawMessage {
	t.Helper()
	body, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return partJSON(t, a2aproto.NewDataPart(json.RawMessage(body)))
}

func rawJSONPart(t *testing.T, body string) json.RawMessage {
	t.Helper()
	return partJSON(t, a2aproto.NewTextPart(body))
}

func partJSON(t *testing.T, part *a2aproto.Part) json.RawMessage {
	t.Helper()
	encoded, err := json.Marshal(part)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

// wireTask is the reply decoded structurally rather than through the SDK's own
// types, so an assertion fails when the bytes change even if the SDK still
// round-trips them.
type wireTask struct {
	Status struct {
		State   string `json:"state"`
		Message struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"message"`
	} `json:"status"`
	Artifacts []struct {
		Name  string     `json:"name"`
		Parts []wirePart `json:"parts"`
	} `json:"artifacts"`

	raw string
}

func (w wireTask) reason() string {
	var parts []string
	for _, p := range w.Status.Message.Parts {
		parts = append(parts, p.Text)
	}
	return strings.Join(parts, " ")
}

// sendMessage makes one JSON-RPC SendMessage call and returns the task.
//
// The method name is "SendMessage", not the "message/send" older A2A material
// prints; v2.5.0 renamed them. internal/jsonrpc/jsonrpc.go:38 is the list.
func sendMessage(t *testing.T, server *httptest.Server, part json.RawMessage) wireTask {
	t.Helper()

	request := fmt.Sprintf(
		`{"jsonrpc":"2.0","id":1,"method":"SendMessage","params":{"message":{"messageId":"m1","role":"ROLE_USER","parts":[%s]}}}`,
		part)

	response, err := http.Post(server.URL+"/", "application/json", strings.NewReader(request))
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	defer response.Body.Close()

	var envelope struct {
		Result json.RawMessage `json:"result"`
		Error  json.RawMessage `json:"error"`
	}
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode the JSON-RPC reply: %v", err)
	}
	if len(envelope.Error) > 0 {
		t.Fatalf("the server answered a JSON-RPC error, which is not how a task failure is reported: %s", envelope.Error)
	}

	// SendMessage answers a a2a.StreamResponse, so the task sits one level
	// down under "task" rather than being the result itself.
	var result struct {
		Task json.RawMessage `json:"task"`
	}
	if err := json.Unmarshal(envelope.Result, &result); err != nil {
		t.Fatalf("decode the JSON-RPC result %s: %v", envelope.Result, err)
	}
	if len(result.Task) == 0 {
		t.Fatalf("the result carries no \"task\": %s", envelope.Result)
	}

	var task wireTask
	if err := json.Unmarshal(result.Task, &task); err != nil {
		t.Fatalf("decode the task out of %s: %v", result.Task, err)
	}
	task.raw = string(result.Task)
	return task
}

// wirePart is one part of an artifact as it comes back on the wire. It is
// declared once because wireTask embeds it and firstArtifactPart returns it,
// and Go requires two anonymous struct types to match field for field.
type wirePart struct {
	MediaType string          `json:"mediaType"`
	Data      json.RawMessage `json:"data"`
	Raw       string          `json:"raw"`
}

// firstArtifactPart fails the test rather than panicking, so a regression that
// drops the artifact reports the envelope it did send.
func (w wireTask) firstArtifactPart(t *testing.T) wirePart {
	t.Helper()
	if len(w.Artifacts) != 1 {
		t.Fatalf("the task carries %d artifacts, want exactly 1: %s", len(w.Artifacts), w.raw)
	}
	if len(w.Artifacts[0].Parts) != 1 {
		t.Fatalf("the artifact carries %d parts, want exactly 1: %s", len(w.Artifacts[0].Parts), w.raw)
	}
	return w.Artifacts[0].Parts[0]
}
