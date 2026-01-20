// artifact.go builds the one reply envelope every agent returns.
//
// It is a separate file from serve.go because the envelope is the contract
// DronaHQ binds to, and it changes for different reasons than the server
// wiring does.
//
// It must not know what any agent's output means. name and v come from the
// caller; nothing here inspects a field.
package a2a

import (
	"encoding/json"
	"fmt"
	"reflect"

	a2aproto "github.com/a2aproject/a2a-go/v2/a2a"
)

const jsonMimeType = "application/json"

// JSONArtifact marshals v into the single application/json artifact an agent
// replies with. name is the output type name, such as "MentionBatch", which is
// what DronaHQ keys its bindings on.
func JSONArtifact(name string, v any) (Artifact, error) {
	if name == "" {
		return Artifact{}, fmt.Errorf("a2a: an artifact needs a name; DronaHQ binds on it")
	}
	body, err := json.Marshal(v)
	if err != nil {
		return Artifact{}, fmt.Errorf("a2a: marshal the %s artifact: %w", name, err)
	}
	return Artifact{MimeType: jsonMimeType, Name: name, Body: body}, nil
}

// toParts converts the envelope to the SDK's shape.
//
// The body goes into a Data part, not a Raw part. Raw is []byte, so the SDK
// base64-encodes it and DronaHQ would have to decode a string before it could
// bind to a field. Data puts the output inline under "data" as real JSON.
//
// It goes in decoded into plain map/slice/string/float64, not as the original
// struct and not as a json.RawMessage, and the round trip is not waste. The
// in-memory task store deep-copies every artifact with encoding/gob, and gob
// refuses an interface value whose concrete type is not registered. The SDK
// registers exactly map[string]any and []any (taskstore/inmemory.go:59), which
// is its way of saying Data.Value holds decoded JSON. A MentionBatch or a
// json.RawMessage in there fails the copy and the task lands in
// TASK_STATE_FAILED with "gob: type not registered".
//
// The media type lives on the part, not on the artifact: v2.5.0 has no mime
// field on Artifact at all. See the note in HACKATHON_NOTES for B6.
func (a Artifact) toParts() (*a2aproto.Artifact, error) {
	if a.Name == "" || len(a.Body) == 0 {
		return nil, fmt.Errorf("a2a: refusing to send an empty artifact %q", a.Name)
	}

	var decoded any
	if err := json.Unmarshal(a.Body, &decoded); err != nil {
		return nil, fmt.Errorf("a2a: decode the %s body for the wire: %w", a.Name, err)
	}

	part := a2aproto.NewDataPart(decoded)
	part.MediaType = a.MimeType
	return &a2aproto.Artifact{
		ID:    a2aproto.NewArtifactID(),
		Name:  a.Name,
		Parts: a2aproto.ContentParts{part},
	}, nil
}

// typeName is the bare name of T, used for the artifact name on the way out
// and for the error message on the way in.
//
// Taking the artifact name from the type rather than from an argument is what
// keeps every agent's main() to a2a.Serve(card, handler): CONTRACTS §1 says
// the name is the output type name, so asking an agent to repeat it would only
// create a way for the two to disagree.
func typeName[T any]() string {
	t := reflect.TypeFor[T]()
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t.Name()
}
