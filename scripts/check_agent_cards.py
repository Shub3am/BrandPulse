#!/usr/bin/env python3
"""Check every agents/*/AgentCard.json carries both halves of the union card.

Why this exists: a2a-go v2.5.0 marshals to the A2A 1.0 shape, which moves url,
protocolVersion and preferredTransport into supportedInterfaces[]. Nasiko's
validate.rs requires all three at the top level. A card generated from the Go
struct fails nasiko validate; a card with only the top-level three is invisible
to an A2A 1.0 consumer. Both halves, always. B5 confirmed this in both
directions against the real CLI.

What this must not do: reach the network, or stand in for `nasiko validate`.
It is the half of that check that needs no cluster, so it can run in CI and on
a laptop before anyone has a Nasiko login.
"""

import json
import pathlib
import sys

# Read out of cli/src/commands/validate.rs, not out of the Nasiko docs. The
# card template a track copies from is docs/research/nasiko.md section 2; if
# this tuple and that template ever disagree, the template is what agents
# actually ship, so fix the template first and then this.
NASIKO_REQUIRED_TOP_LEVEL = (
    "name",
    "description",
    "url",
    "version",
    "capabilities",
    "skills",
    "protocolVersion",
    "preferredTransport",
)

# The Nasiko example ships "0.2.9" and a real cluster answers -32009
# VersionNotSupported.
REQUIRED_PROTOCOL_VERSION = "1.0"

# Inside supportedInterfaces[] the transport key is NOT the top level's
# preferredTransport. a2a-go v2.5.0's AgentInterface names it protocolBinding
# (a2a/agent.go:130), and a2a.Serve refuses to start when no interface declares
# the JSONRPC binding, because JSONRPC is the only transport it mounts. An
# interface carrying preferredTransport instead unmarshals to an empty binding
# and every agent dies in a restart loop. This check exists because all nine
# cards shipped exactly that and passed the rest of this script.
REQUIRED_PROTOCOL_BINDING = "JSONRPC"

# The four agents that make no LLM call, so their cards must say llm_provider
# is null. Every other agent must name a provider. The key has to be present
# either way: an absent llm_provider and an explicit null read the same to a
# JSON parser but not to a human deciding what this agent costs to run, and
# bp-responder and bp-briefer both shipped without it and passed everything
# else in this script.
AGENTS_WITHOUT_LLM = frozenset(
    {"bp-collector", "bp-sov", "bp-detector", "bp-orchestrator"}
)


def interface_problems(interfaces: object) -> list[str]:
    """Return one sentence per thing wrong with a card's supportedInterfaces."""
    if not isinstance(interfaces, list) or not interfaces:
        return ["supportedInterfaces is empty, so a2a.Serve has no transport to mount"]

    for index, interface in enumerate(interfaces):
        if not isinstance(interface, dict):
            return [f"supportedInterfaces[{index}] is not an object"]
        if interface.get("protocolBinding") == REQUIRED_PROTOCOL_BINDING:
            return []

    bindings = [interface.get("protocolBinding") for interface in interfaces]
    return [
        f"no interface declares protocolBinding {REQUIRED_PROTOCOL_BINDING!r}, "
        f"which is the only transport a2a.Serve mounts, so the agent refuses to "
        f"start. Found {bindings!r}. Note the key inside the array is "
        f"protocolBinding, not the top level's preferredTransport."
    ]


def problems_with(card: dict) -> list[str]:
    """Return one human sentence per thing wrong with a parsed AgentCard."""
    problems = []

    missing = [key for key in NASIKO_REQUIRED_TOP_LEVEL if key not in card]
    if missing:
        problems.append(
            "nasiko validate needs these at the top level: " + ", ".join(missing)
        )

    if "supportedInterfaces" not in card:
        problems.append(
            "no supportedInterfaces[], so an A2A 1.0 consumer cannot see this "
            "agent's transport. Add it, keep the top-level fields too."
        )
    else:
        problems.extend(interface_problems(card["supportedInterfaces"]))

    # Both of these keys are in the tuple above, so the value checks only run
    # once the key is present. One defect should cost one CI annotation.
    if "protocolVersion" in card and card["protocolVersion"] != REQUIRED_PROTOCOL_VERSION:
        problems.append(
            f"protocolVersion is {card['protocolVersion']!r}, a real cluster rejects "
            f"anything but {REQUIRED_PROTOCOL_VERSION!r} with -32009 VersionNotSupported"
        )

    if "skills" in card and not card["skills"]:
        problems.append("skills is empty, so the agent is invisible to routing")

    problems.extend(llm_provider_problems(card))

    return problems


def llm_provider_problems(card: dict) -> list[str]:
    """Return one sentence if the card's llm_provider disagrees with the code."""
    name = card.get("name")
    makes_no_llm_call = name in AGENTS_WITHOUT_LLM

    if "llm_provider" not in card:
        wanted = "null" if makes_no_llm_call else "the router it calls"
        return [
            f"no llm_provider key. {name!r} must declare it as {wanted}, because "
            f"whether an agent costs tokens to run is the kind of true a reader "
            f"checks the card for"
        ]

    provider = card["llm_provider"]
    if makes_no_llm_call and provider is not None:
        return [
            f"llm_provider is {provider!r} but {name!r} makes no LLM call, so the "
            f"card overstates what this agent costs to run"
        ]
    if not makes_no_llm_call and not provider:
        return [
            f"llm_provider is {provider!r} but {name!r} does call an LLM, so the "
            f"card understates what this agent costs to run"
        ]
    return []


def main() -> int:
    cards = sorted(pathlib.Path("agents").glob("*/AgentCard.json"))
    if not cards:
        print("no AgentCard.json under agents/ yet, nothing to check")
        return 0

    failed = False
    for path in cards:
        try:
            card = json.loads(path.read_text())
        except json.JSONDecodeError as err:
            print(f"::error file={path}::not valid JSON: {err}")
            failed = True
            continue

        problems = problems_with(card)
        for problem in problems:
            print(f"::error file={path}::{problem}")
        if problems:
            failed = True
        else:
            print(f"ok  {path}")

    print(f"\n{len(cards)} card(s) checked")
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
