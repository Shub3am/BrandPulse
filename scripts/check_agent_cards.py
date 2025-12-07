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

# Read out of cli/src/commands/validate.rs, not out of the Nasiko docs.
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

    version = card.get("protocolVersion")
    if version != REQUIRED_PROTOCOL_VERSION:
        problems.append(
            f"protocolVersion is {version!r}, a real cluster rejects anything "
            f"but {REQUIRED_PROTOCOL_VERSION!r} with -32009 VersionNotSupported"
        )

    if not card.get("skills"):
        problems.append("skills is empty, so the agent is invisible to routing")

    return problems


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
