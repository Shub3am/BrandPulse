// llm_test.go proves the router contract without a router: every test swaps
// routerHTTPClient for a scripted transport, so no test here can reach a
// network, spend a token, or need OPENAI_API_KEY.
//
// The test that matters most is TestCostIsPricedFromTheModelTheRouterReports.
// Everything downstream of it, eval/cost and the rupee on the dashboard, is
// wrong in the same direction if it ever goes green by accident.
package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

// scriptedRouter answers each call with the next body in its script and keeps
// every request it was sent.
type scriptedRouter struct {
	bodies   []string
	statuses []int
	sent     []*recordedRequest
}

type recordedRequest struct {
	path string
	body map[string]any
}

func (s *scriptedRouter) RoundTrip(req *http.Request) (*http.Response, error) {
	raw, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("the test sent a body that is not JSON: %w", err)
	}
	s.sent = append(s.sent, &recordedRequest{path: req.URL.Path, body: parsed})

	if len(s.sent) > len(s.bodies) {
		return nil, fmt.Errorf("the router was called %d times, and the script has %d replies", len(s.sent), len(s.bodies))
	}
	status := http.StatusOK
	if len(s.statuses) >= len(s.sent) {
		status = s.statuses[len(s.sent)-1]
	}
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(s.bodies[len(s.sent)-1])),
		Request:    req,
	}, nil
}

// useRouter points the package at a scripted transport for the duration of one
// test and restores the nil afterwards.
func useRouter(t *testing.T, bodies ...string) *scriptedRouter {
	t.Helper()

	router := &scriptedRouter{bodies: bodies}
	t.Setenv("OPENAI_BASE_URL", "https://router.invalid/v1")
	// Cleared rather than left alone: a developer with OPENAI_MODEL exported
	// for a local run would otherwise silently override every Opt.Model these
	// tests assert on.
	t.Setenv("OPENAI_MODEL", "")
	routerHTTPClient = &http.Client{Transport: router}
	t.Cleanup(func() { routerHTTPClient = nil })
	return router
}

// sentiment is a caller-shaped schema: exported fields, no omitempty, which is
// what strict mode requires.
type sentiment struct {
	Label string `json:"label" jsonschema:"enum=positive,enum=negative"`
	Score int    `json:"score"`
}

func completionBody(model, content string, promptTokens, completionTokens int) string {
	body, err := json.Marshal(map[string]any{
		"id":      "chatcmpl_test",
		"object":  "chat.completion",
		"created": 1758326400,
		"model":   model,
		"choices": []any{map[string]any{
			"index":         0,
			"finish_reason": "stop",
			"message":       map[string]any{"role": "assistant", "content": content},
		}},
		"usage": map[string]any{
			"prompt_tokens":     promptTokens,
			"completion_tokens": completionTokens,
			"total_tokens":      promptTokens + completionTokens,
		},
	})
	if err != nil {
		panic(err)
	}
	return string(body)
}

func TestChatJSONReturnsTheReplyUndecoded(t *testing.T) {
	router := useRouter(t, completionBody("openai/gpt-4o", `{"label":"positive","score":4}`, 1200, 300))

	reply, usage, err := ChatJSON(context.Background(), "how does this review feel?", sentiment{}, Opt{})
	if err != nil {
		t.Fatalf("ChatJSON: %v", err)
	}
	if string(reply) != `{"label":"positive","score":4}` {
		t.Errorf("reply = %s, want the model's content verbatim", reply)
	}
	if usage.PromptTokens != 1200 || usage.CompletionTokens != 300 {
		t.Errorf("usage = %d/%d tokens, want 1200/300", usage.PromptTokens, usage.CompletionTokens)
	}
	if len(router.sent) != 1 {
		t.Fatalf("the router was called %d times, want 1", len(router.sent))
	}
	if router.sent[0].path != "/v1/chat/completions" {
		t.Errorf("path = %s, want /v1/chat/completions", router.sent[0].path)
	}
}

func TestCostIsPricedFromTheModelTheRouterReports(t *testing.T) {
	// gpt-4o-mini is asked for; the router answers as gpt-4o, which is what
	// Nasiko does when an agent's llm-config says otherwise.
	useRouter(t, completionBody("openai/gpt-4o", `{"label":"positive","score":4}`, 1200, 300))

	_, usage, err := ChatJSON(context.Background(), "prompt", sentiment{}, Opt{Model: "openai/gpt-4o-mini"})
	if err != nil {
		t.Fatalf("ChatJSON: %v", err)
	}

	// 1200 in at $2.50/M plus 300 out at $10.00/M is $0.006, and $0.006 at
	// 95.885 INR is 57.531 paise.
	const want = 57.531
	if diff := usage.CostPaise - want; diff > 0.001 || diff < -0.001 {
		t.Errorf("CostPaise = %v, want %v (the gpt-4o price, not gpt-4o-mini's)", usage.CostPaise, want)
	}
	if mini := costPaise("openai/gpt-4o-mini", 1200, 300); usage.CostPaise == mini {
		t.Errorf("CostPaise = %v, which is the requested model's price; it must be the reported model's", mini)
	}
}

func TestAnUnknownModelIsChargedAtTheDearestKnownPrice(t *testing.T) {
	useRouter(t, completionBody("mistral/whatever-the-router-added", `{"label":"positive","score":4}`, 1200, 300))

	_, usage, err := ChatJSON(context.Background(), "prompt", sentiment{}, Opt{})
	if err != nil {
		t.Fatalf("ChatJSON: %v", err)
	}
	if usage.CostPaise == 0 {
		t.Fatal("CostPaise = 0 for an unpriced model; a silent zero is how a cost dashboard lies")
	}
	for model := range modelPrices {
		if usage.CostPaise < costPaise(model, 1200, 300)-0.001 {
			t.Errorf("CostPaise = %v, cheaper than the known model %s; the fallback must be the dearest row", usage.CostPaise, model)
		}
	}
}

func TestTheRequestCarriesAStrictSchemaThatForbidsExtraKeys(t *testing.T) {
	router := useRouter(t, completionBody("openai/gpt-4o", `{"label":"positive","score":4}`, 10, 10))

	if _, _, err := ChatJSON(context.Background(), "prompt", sentiment{}, Opt{MaxTokens: 512}); err != nil {
		t.Fatalf("ChatJSON: %v", err)
	}

	body := router.sent[0].body
	format, ok := body["response_format"].(map[string]any)
	if !ok {
		t.Fatalf("the request carries no response_format: %v", body)
	}
	if format["type"] != "json_schema" {
		t.Errorf("response_format.type = %v, want json_schema", format["type"])
	}
	jsonSchema, ok := format["json_schema"].(map[string]any)
	if !ok {
		t.Fatalf("response_format carries no json_schema: %v", format)
	}
	if jsonSchema["strict"] != true {
		t.Errorf("json_schema.strict = %v, want true", jsonSchema["strict"])
	}
	if jsonSchema["name"] != "sentiment" {
		t.Errorf("json_schema.name = %v, want the Go type name sentiment", jsonSchema["name"])
	}
	schema, ok := jsonSchema["schema"].(map[string]any)
	if !ok {
		t.Fatalf("json_schema carries no schema: %v", jsonSchema)
	}
	if schema["additionalProperties"] != false {
		t.Errorf("schema.additionalProperties = %v, want false", schema["additionalProperties"])
	}
	if _, ok := schema["properties"].(map[string]any); !ok {
		t.Errorf("the schema has no properties, so the Go type was not reflected: %v", schema)
	}
	if body["max_tokens"] != float64(512) {
		t.Errorf("max_tokens = %v, want 512", body["max_tokens"])
	}
}

func TestUnparseableJSONIsRetriedOnceWithANudge(t *testing.T) {
	router := useRouter(t,
		completionBody("openai/gpt-4o", "Sure! Here you go: {label: positive}", 1200, 300),
		completionBody("openai/gpt-4o", `{"label":"positive","score":4}`, 1400, 300),
	)

	reply, usage, err := ChatJSON(context.Background(), "prompt", sentiment{}, Opt{})
	if err != nil {
		t.Fatalf("ChatJSON: %v", err)
	}
	if string(reply) != `{"label":"positive","score":4}` {
		t.Errorf("reply = %s, want the second attempt's JSON", reply)
	}
	if len(router.sent) != 2 {
		t.Fatalf("the router was called %d times, want 2", len(router.sent))
	}

	messages, ok := router.sent[1].body["messages"].([]any)
	if !ok || len(messages) != 3 {
		t.Fatalf("the retry carried %v, want the prompt, the bad reply and the nudge", router.sent[1].body["messages"])
	}
	last, _ := messages[2].(map[string]any)
	if content, _ := last["content"].(string); content != jsonNudge {
		t.Errorf("the last message is %q, want the nudge", content)
	}

	// Both attempts were billed, so both are counted.
	if usage.PromptTokens != 2600 || usage.CompletionTokens != 600 {
		t.Errorf("usage = %d/%d tokens, want 2600/600 across both attempts", usage.PromptTokens, usage.CompletionTokens)
	}
}

func TestUnparseableJSONTwiceIsAnError(t *testing.T) {
	bad := completionBody("openai/gpt-4o", "still not JSON", 100, 10)
	router := useRouter(t, bad, bad)

	_, usage, err := ChatJSON(context.Background(), "prompt", sentiment{}, Opt{})
	if err == nil {
		t.Fatal("ChatJSON returned no error after two unparseable replies")
	}
	if !strings.Contains(err.Error(), "twice") {
		t.Errorf("error = %q, want it to say the model failed twice", err)
	}
	if len(router.sent) != 2 {
		t.Errorf("the router was called %d times, want exactly 2", len(router.sent))
	}
	if usage.CostPaise == 0 {
		t.Error("CostPaise = 0 on the failure path; two billed attempts are still two billed attempts")
	}
}

func TestChatJSONRefusesToRunWithoutTheRouterURL(t *testing.T) {
	t.Setenv("OPENAI_BASE_URL", "")

	_, _, err := ChatJSON(context.Background(), "prompt", sentiment{}, Opt{})
	if err == nil {
		t.Fatal("ChatJSON ran with no OPENAI_BASE_URL")
	}
	if !strings.Contains(err.Error(), "OPENAI_BASE_URL") {
		t.Errorf("error = %q, want it to name OPENAI_BASE_URL", err)
	}
}

func TestChatJSONRefusesANilSchema(t *testing.T) {
	useRouter(t, completionBody("openai/gpt-4o", `{}`, 1, 1))

	if _, _, err := ChatJSON(context.Background(), "prompt", nil, Opt{}); err == nil {
		t.Fatal("ChatJSON accepted a nil schema, which is an unconstrained reply")
	}
}

func TestEmbedBatchesAtOneHundred(t *testing.T) {
	texts := make([]string, 250)
	for i := range texts {
		texts[i] = fmt.Sprintf("mention %d", i)
	}

	router := useRouter(t, embeddingBody(100), embeddingBody(100), embeddingBody(50))

	vectors, err := Embed(context.Background(), texts, "text-embedding-3-small")
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(vectors) != 250 {
		t.Errorf("got %d vectors, want 250", len(vectors))
	}
	if len(router.sent) != 3 {
		t.Fatalf("the router was called %d times, want 3 batches", len(router.sent))
	}
	for i, wantSize := range []int{100, 100, 50} {
		input, ok := router.sent[i].body["input"].([]any)
		if !ok {
			t.Fatalf("batch %d carried no input array: %v", i, router.sent[i].body)
		}
		if len(input) != wantSize {
			t.Errorf("batch %d carried %d texts, want %d", i, len(input), wantSize)
		}
	}
}

func embeddingBody(count int) string {
	data := make([]any, count)
	for i := range data {
		data[i] = map[string]any{"object": "embedding", "index": i, "embedding": []float64{0.5, -0.25}}
	}
	body, err := json.Marshal(map[string]any{
		"object": "list",
		"model":  "text-embedding-3-small",
		"data":   data,
		"usage":  map[string]any{"prompt_tokens": count, "total_tokens": count},
	})
	if err != nil {
		panic(err)
	}
	return string(body)
}

func TestOPENAIMODELOverridesTheRequestedModel(t *testing.T) {
	// A Nasiko alias is what the agents actually hardcode: bp-briefer passes
	// "bp-briefer-default", which names a router entry and not a model. Off
	// Nasiko that alias reaches the provider verbatim and is rejected, so the
	// variable naming the real model has to win.
	router := useRouter(t, completionBody("openai/gpt-4o", `{"label":"positive","score":4}`, 10, 5))
	t.Setenv("OPENAI_MODEL", "gpt-4o-mini")

	if _, _, err := ChatJSON(context.Background(), "how does this review feel?", sentiment{}, Opt{Model: "bp-briefer-default"}); err != nil {
		t.Fatalf("ChatJSON: %v", err)
	}
	if got := router.sent[0].body["model"]; got != "gpt-4o-mini" {
		t.Errorf("model = %v, want gpt-4o-mini from OPENAI_MODEL", got)
	}
}

func TestTheRequestedModelIsSentWhenOPENAIMODELIsUnset(t *testing.T) {
	router := useRouter(t, completionBody("openai/gpt-4o", `{"label":"positive","score":4}`, 10, 5))

	if _, _, err := ChatJSON(context.Background(), "how does this review feel?", sentiment{}, Opt{Model: "bp-briefer-default"}); err != nil {
		t.Fatalf("ChatJSON: %v", err)
	}
	if got := router.sent[0].body["model"]; got != "bp-briefer-default" {
		t.Errorf("model = %v, want the alias the caller passed", got)
	}
}

func TestNoModelIsSentWhenNeitherIsSet(t *testing.T) {
	// The Nasiko router resolves the agent's model itself and wants the field
	// absent, so an empty string must not be marshalled in its place.
	router := useRouter(t, completionBody("openai/gpt-4o", `{"label":"positive","score":4}`, 10, 5))

	if _, _, err := ChatJSON(context.Background(), "how does this review feel?", sentiment{}, Opt{}); err != nil {
		t.Fatalf("ChatJSON: %v", err)
	}
	if got, ok := router.sent[0].body["model"]; ok && got != "" {
		t.Errorf("model = %v, want it absent or empty", got)
	}
}
