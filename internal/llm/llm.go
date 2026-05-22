// Package llm is the only path to a language model in this repository.
//
// Every call goes through the Nasiko router at OPENAI_BASE_URL. No agent
// configures a provider SDK, no agent holds a key, and no agent picks a
// provider: per-agent model choice is set with "nasiko llm-config" outside
// this code.
//
// Two consequences follow from the router, and both are load-bearing:
//
//   - The router ignores the model named in the request body. Usage.CostPaise
//     is therefore priced from the model the router reports back in the
//     response, not from Opt.Model. Costing the requested model would report a
//     number we never paid.
//   - bp-detector must not import this package. Alerting is statistics, and
//     every alert carries the numbers that fired it.
//
// It must not redact. redact.PII runs at the call site, on the mention text,
// before a prompt is ever built; a redactor hidden in here would let a caller
// forget that and still look safe. The price table lives in cost.go.
package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"strings"

	"github.com/invopop/jsonschema"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/packages/param"
)

// embedBatchSize is the router's request granularity for embeddings, not a
// provider limit.
const embedBatchSize = 100

// jsonNudge is appended as a second user turn when the first reply does not
// parse. One retry, then the error: a model that ignores a strict schema twice
// is not going to yield on the third ask, and each attempt is billed.
const jsonNudge = "That reply was not valid JSON. Return only valid JSON matching the schema, with no prose and no code fence."

// routerHTTPClient lets this package's own tests swap the transport without
// widening the frozen ChatJSON signature. It is nil everywhere else, and a
// caller outside the package cannot reach it.
var routerHTTPClient *http.Client

// Opt carries the per-call knobs. Both fields are optional: a zero Model lets
// the router's per-agent configuration decide, and a zero MaxTokens lets the
// provider default apply.
type Opt struct {
	Model     string
	MaxTokens int
}

// Usage is what one call actually cost. CostPaise is priced from the model the
// router reports in its response, never from the model requested.
type Usage struct {
	PromptTokens, CompletionTokens int
	CostPaise                      float64
}

// ChatJSONFunc is ChatJSON's signature. An agent that calls a model holds this
// as a field and is constructed with llm.ChatJSON, so a test hands it a canned
// reply instead of reaching the router. bp-enricher, bp-briefer and
// bp-responder each reached for this shape independently.
type ChatJSONFunc func(ctx context.Context, prompt string, schema any, opt Opt) (json.RawMessage, Usage, error)

// ChatJSON sends prompt and returns a response constrained to schema by a
// strict JSON-schema response format. schema is any Go value whose shape
// describes the expected reply; the raw JSON comes back undecoded so the
// caller owns the unmarshal.
//
// A reply that does not parse is retried once with a "return only valid JSON"
// nudge, then returned as an error.
func ChatJSON(ctx context.Context, prompt string, schema any, opt Opt) (json.RawMessage, Usage, error) {
	client, err := newRouterClient()
	if err != nil {
		return nil, Usage{}, err
	}

	format, err := responseFormat(schema)
	if err != nil {
		return nil, Usage{}, err
	}

	messages := []openai.ChatCompletionMessageParamUnion{openai.UserMessage(prompt)}
	var spent Usage

	for attempt := 1; attempt <= 2; attempt++ {
		params := openai.ChatCompletionNewParams{
			Messages:       messages,
			ResponseFormat: format,
		}
		if opt.Model != "" {
			params.Model = opt.Model
		}
		if opt.MaxTokens > 0 {
			// max_tokens, not max_completion_tokens: the router fronts
			// Anthropic and Gemini too, and max_tokens is the field every
			// OpenAI-compatible proxy translates.
			params.MaxTokens = param.NewOpt(int64(opt.MaxTokens))
		}

		completion, err := client.Chat.Completions.New(ctx, params)
		if err != nil {
			return nil, spent, fmt.Errorf("llm: chat completion: %w", err)
		}

		reply, err := onlyChoice(completion)
		if err != nil {
			return nil, spent, err
		}
		spent = addUsage(spent, completion)

		if json.Valid([]byte(reply)) {
			return json.RawMessage(reply), spent, nil
		}
		messages = append(messages, openai.AssistantMessage(reply), openai.UserMessage(jsonNudge))
	}

	return nil, spent, fmt.Errorf("llm: the model returned unparseable JSON twice, with the schema attached both times")
}

// Embed returns one vector per text, batched at 100 per request.
//
// Nothing on the critical path calls this: the router lists chat models only
// and bp-clusterer clusters on local TF-IDF. The signature exists so the
// contract is complete, and the body is deliberately the plainest thing that
// honours it.
func Embed(ctx context.Context, texts []string, model string) ([][]float32, error) {
	client, err := newRouterClient()
	if err != nil {
		return nil, err
	}

	vectors := make([][]float32, 0, len(texts))
	for start := 0; start < len(texts); start += embedBatchSize {
		end := min(start+embedBatchSize, len(texts))

		batch, err := embedBatch(ctx, client, texts[start:end], model)
		if err != nil {
			return nil, err
		}
		vectors = append(vectors, batch...)
	}
	return vectors, nil
}

// embedBatch retries once when the response body does not parse. There is no
// nudge to send here: an embeddings reply carries no model-authored JSON, so
// unparseable means a truncated or mangled response, and the only useful
// answer to that is to ask again.
func embedBatch(ctx context.Context, client openai.Client, texts []string, model string) ([][]float32, error) {
	params := openai.EmbeddingNewParams{
		Input: openai.EmbeddingNewParamsInputUnion{OfArrayOfStrings: texts},
		Model: openai.EmbeddingModel(model),
	}

	response, err := client.Embeddings.New(ctx, params)
	if err != nil && isJSONError(err) {
		response, err = client.Embeddings.New(ctx, params)
	}
	if err != nil {
		return nil, fmt.Errorf("llm: embed %d texts with %s: %w", len(texts), model, err)
	}

	vectors := make([][]float32, len(response.Data))
	for _, embedding := range response.Data {
		narrowed := make([]float32, len(embedding.Embedding))
		for i, value := range embedding.Embedding {
			narrowed[i] = float32(value)
		}
		vectors[embedding.Index] = narrowed
	}
	return vectors, nil
}

// newRouterClient builds a client per call. There is no cached singleton on
// purpose: OPENAI_BASE_URL is read at call time, so a test or a redeployed
// agent never talks to a router it configured an hour ago.
func newRouterClient() (openai.Client, error) {
	baseURL := os.Getenv("OPENAI_BASE_URL")
	if baseURL == "" {
		return openai.Client{}, errors.New("llm: OPENAI_BASE_URL is unset; every model call goes through the Nasiko router and there is no direct-to-provider fallback")
	}

	opts := []option.RequestOption{option.WithBaseURL(baseURL)}
	if routerHTTPClient != nil {
		opts = append(opts, option.WithHTTPClient(routerHTTPClient))
	}
	// The key is left to the SDK's own OPENAI_API_KEY default: Nasiko injects
	// it at runtime and CI never has one.
	return openai.NewClient(opts...), nil
}

// responseFormat turns a Go value into the strict JSON-schema response format.
//
// Strict mode requires every property to be listed in "required", and invopop
// only marks a field required when it has no omitempty. A caller whose schema
// struct uses omitempty will be rejected by the provider, not by us.
func responseFormat(schema any) (openai.ChatCompletionNewParamsResponseFormatUnion, error) {
	if schema == nil {
		return openai.ChatCompletionNewParamsResponseFormatUnion{}, errors.New("llm: ChatJSON needs a schema value; a nil schema is an unconstrained reply")
	}

	reflector := jsonschema.Reflector{AllowAdditionalProperties: false, DoNotReference: true}
	reflected := reflector.Reflect(schema)

	return openai.ChatCompletionNewParamsResponseFormatUnion{
		OfJSONSchema: &openai.ResponseFormatJSONSchemaParam{
			JSONSchema: openai.ResponseFormatJSONSchemaJSONSchemaParam{
				Name:   schemaName(schema),
				Strict: param.NewOpt(true),
				Schema: reflected,
			},
		},
	}, nil
}

// schemaName is what the provider echoes in errors, so it is worth being the
// Go type name. The API allows letters, digits, underscore and dash only.
func schemaName(schema any) string {
	name := reflect.TypeOf(schema).Name()
	name = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
			return r
		default:
			return -1
		}
	}, name)
	if name == "" {
		return "response"
	}
	return name
}

func onlyChoice(completion *openai.ChatCompletion) (string, error) {
	if len(completion.Choices) == 0 {
		return "", fmt.Errorf("llm: the router returned no choices for model %s", completion.Model)
	}
	return completion.Choices[0].Message.Content, nil
}

// addUsage accumulates across the retry, because both attempts were billed.
func addUsage(spent Usage, completion *openai.ChatCompletion) Usage {
	spent.PromptTokens += int(completion.Usage.PromptTokens)
	spent.CompletionTokens += int(completion.Usage.CompletionTokens)
	spent.CostPaise += costPaise(completion.Model, int(completion.Usage.PromptTokens), int(completion.Usage.CompletionTokens))
	return spent
}

func isJSONError(err error) bool {
	var syntax *json.SyntaxError
	var unmarshalType *json.UnmarshalTypeError
	return errors.As(err, &syntax) || errors.As(err, &unmarshalType)
}
