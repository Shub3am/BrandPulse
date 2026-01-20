// card_test.go is written against JSON text rather than against Go structs.
//
// B5 writes nine AgentCard.json files by hand. What matters is whether those
// bytes load, and a test that built a Card in Go would never see the mistake
// this file exists to catch: a protocolVersion written at the top level, where
// the old A2A material puts it, parses cleanly and means nothing.
package a2a

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const goodCard = `{
  "name": "bp-collector",
  "description": "collects mentions",
  "version": "0.1.0",
  "capabilities": {},
  "defaultInputModes": ["application/json"],
  "defaultOutputModes": ["application/json"],
  "skills": [],
  "supportedInterfaces": [
    {
      "url": "https://bp-collector.nasiko.internal/",
      "protocolBinding": "JSONRPC",
      "protocolVersion": "1.0"
    }
  ]
}`

func TestLoadCardAcceptsAGoodCard(t *testing.T) {
	card, err := LoadCard(writeCard(t, goodCard))
	if err != nil {
		t.Fatalf("LoadCard rejected a good card: %v", err)
	}
	if card.Name != "bp-collector" {
		t.Errorf("card name is %q, want bp-collector", card.Name)
	}
	if len(card.SupportedInterfaces) != 1 {
		t.Fatalf("card has %d interfaces, want 1", len(card.SupportedInterfaces))
	}
	if got := card.SupportedInterfaces[0].ProtocolVersion; got != "1.0" {
		t.Errorf("protocolVersion is %q, want 1.0", got)
	}
}

func TestACardWithProtocolVersionAtTheTopLevelIsRejected(t *testing.T) {
	// This is the shape the brief and the older A2A docs describe, and it is
	// the one that costs a deploy: a2a-go v2.5.0 moved protocolVersion onto
	// each supportedInterfaces entry, so at the top level encoding/json drops
	// it without a word and the card ships advertising no interfaces at all.
	topLevel := `{
  "name": "bp-collector",
  "protocolVersion": "1.0",
  "url": "https://bp-collector.nasiko.internal/",
  "version": "0.1.0"
}`

	_, err := LoadCard(writeCard(t, topLevel))
	if err == nil {
		t.Fatal("LoadCard accepted a card whose protocolVersion is at the top level, where the SDK never reads it")
	}
	if !strings.Contains(err.Error(), "supportedInterfaces") {
		t.Errorf("the error is %q; it has to say where protocolVersion actually goes, or B5 reads it nine times", err)
	}
}

func TestTheNasikoExampleVersionIsRejected(t *testing.T) {
	// 0.2.9 is what the Nasiko sample ships. A cluster answers -32009
	// VersionNotSupported, which surfaces at deploy time as a routing failure
	// with no obvious cause.
	stale := strings.Replace(goodCard, `"protocolVersion": "1.0"`, `"protocolVersion": "0.2.9"`, 1)

	_, err := LoadCard(writeCard(t, stale))
	if err == nil {
		t.Fatal("LoadCard accepted protocolVersion 0.2.9")
	}
	if !strings.Contains(err.Error(), "0.2.9") || !strings.Contains(err.Error(), "-32009") {
		t.Errorf("the error is %q; it must name the version it found and the code the cluster answers", err)
	}
}

func TestACardThatServesOnlyGRPCIsRejected(t *testing.T) {
	// Serve mounts JSON-RPC and nothing else. Such a card would deploy, answer
	// the agent-card request, and fail every actual call.
	grpcOnly := strings.Replace(goodCard, `"protocolBinding": "JSONRPC"`, `"protocolBinding": "GRPC"`, 1)

	_, err := LoadCard(writeCard(t, grpcOnly))
	if err == nil {
		t.Fatal("LoadCard accepted a card that advertises no JSONRPC interface")
	}
	if !strings.Contains(err.Error(), "JSONRPC") {
		t.Errorf("the error is %q, and it must name the transport Serve does mount", err)
	}
}

func TestACardWithNoURLIsRejected(t *testing.T) {
	noURL := strings.Replace(goodCard, `"url": "https://bp-collector.nasiko.internal/",`, "", 1)

	if _, err := LoadCard(writeCard(t, noURL)); err == nil {
		t.Fatal("LoadCard accepted an interface with no url")
	}
}

func TestACardWithNoNameIsRejected(t *testing.T) {
	noName := strings.Replace(goodCard, `"name": "bp-collector",`, "", 1)

	if _, err := LoadCard(writeCard(t, noName)); err == nil {
		t.Fatal("LoadCard accepted a card with no name")
	}
}

func TestLoadCardSaysWhichFileIsWrong(t *testing.T) {
	// Nine agents, nine cards, one CI log. An error that does not name the
	// file is an error that costs a bisect.
	path := writeCard(t, `{"name": "bp-collector"}`)

	_, err := LoadCard(path)
	if err == nil {
		t.Fatal("LoadCard accepted a card with no interfaces")
	}
	if !strings.Contains(err.Error(), filepath.Base(path)) {
		t.Errorf("the error is %q and does not name %s", err, filepath.Base(path))
	}
}

func TestLoadCardOnAMissingFile(t *testing.T) {
	if _, err := LoadCard(filepath.Join(t.TempDir(), "AgentCard.json")); err == nil {
		t.Fatal("LoadCard returned no error for a file that does not exist")
	}
}

func writeCard(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "AgentCard.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
