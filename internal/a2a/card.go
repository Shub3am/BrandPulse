// card.go loads and validates an agent's AgentCard.json.
//
// Split from serve.go because the card is B5's artifact and the server wiring
// is mine, and they go wrong for different reasons: a bad card is a deploy
// rejection, a bad server is a failed request.
//
// It must not invent a field an agent did not write. A card that is missing
// something fails here, loudly, on the agent's first line, rather than being
// silently completed and rejected by the cluster.
package a2a

import (
	"encoding/json"
	"fmt"
	"os"

	a2aproto "github.com/a2aproject/a2a-go/v2/a2a"
)

// Card is one agent's AgentCard.json.
//
// It is an alias for the SDK's own card, not a copy of it. CONTRACTS §3 kept
// a2a.Card as a separate name so this package could reconcile the two at Task
// 11 without touching a single agent's main(), and the reconciliation that
// turned out to be needed was this: there is nothing left to translate. A copy
// would be sixteen fields to keep in step with an SDK that has already moved
// one of them.
//
// Agents never build one by hand. They call LoadCard and pass the result to
// Serve, and Serve validates again anyway, so the guarantee does not depend on
// which door the card came through.
type Card = a2aproto.AgentCard

// LoadCard reads an AgentCard.json from path and rejects a card the cluster
// would reject.
func LoadCard(path string) (Card, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Card{}, fmt.Errorf("a2a: read the agent card: %w", err)
	}

	var card Card
	if err := json.Unmarshal(raw, &card); err != nil {
		return Card{}, fmt.Errorf("a2a: parse %s: %w", path, err)
	}
	if err := validateCard(card); err != nil {
		return Card{}, fmt.Errorf("a2a: %s: %w", path, err)
	}
	return card, nil
}

// validateCard enforces the three things a BrandPulse agent's card must say.
//
// protocolVersion is checked per interface because that is where a2a-go v2.5.0
// puts it: AgentCard has no protocolVersion field at all, only
// supportedInterfaces[].protocolVersion. A card with the version at the top
// level parses without error, leaves supportedInterfaces empty, and is
// rejected by a real cluster with -32009 VersionNotSupported. That is the
// failure this function exists to move from deploy time to startup.
func validateCard(card Card) error {
	if card.Name == "" {
		return fmt.Errorf("the card has no name")
	}
	if len(card.SupportedInterfaces) == 0 {
		return fmt.Errorf(
			"the card declares no supportedInterfaces; protocolVersion and url live inside that array, not at the top level")
	}

	servesJSONRPC := false
	for i, iface := range card.SupportedInterfaces {
		if iface == nil {
			return fmt.Errorf("supportedInterfaces[%d] is null", i)
		}
		if iface.ProtocolVersion != a2aproto.Version {
			return fmt.Errorf(
				"supportedInterfaces[%d].protocolVersion is %q, want %q; the Nasiko example ships 0.2.9 and a real cluster answers -32009 VersionNotSupported",
				i, iface.ProtocolVersion, a2aproto.Version)
		}
		if iface.URL == "" {
			return fmt.Errorf("supportedInterfaces[%d] has no url", i)
		}
		if iface.ProtocolBinding == a2aproto.TransportProtocolJSONRPC {
			servesJSONRPC = true
		}
	}

	// Serve only mounts JSON-RPC. A card advertising nothing but gRPC would
	// deploy, answer the card request, and then fail every call, which is a
	// worse failure than not starting.
	if !servesJSONRPC {
		return fmt.Errorf(
			"no interface declares protocolBinding %q, which is the only transport Serve mounts",
			a2aproto.TransportProtocolJSONRPC)
	}
	return nil
}
