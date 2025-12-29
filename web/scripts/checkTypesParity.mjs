// Fails the build when a TypeScript mirror has drifted from internal/models/models.go.
//
// The mirrors exist because the BFF passes agent artifacts through unreshaped, so
// a Go `json` tag is the field name that arrives in the browser. TypeScript
// cannot catch a rename: the data is JSON at runtime, and a renamed field is
// `undefined` on a panel that still renders. This script is the only thing
// standing between a field rename on `main` and a blank number on stage.
//
// It compares field names and optionality, nothing else. It does not typecheck,
// it does not compare Go types to TS types, and it must not grow an allowlist:
// the single named divergence below is a constant for that reason.

import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

const repoRoot = fileURLToPath(new URL("../../", import.meta.url));

const GO_MODELS = "internal/models/models.go";

/** Every file that claims to mirror models.go. Both are checked identically. */
const MIRRORS = ["web/lib/types.ts", "bff/src/contracts.ts"];

// ReplyDraft.requires_human_approval is on the wire and has no Go struct field:
// it is a guardrail rather than state, and a bool field would decode to false
// from any payload that omitted it, so models.go emits it from MarshalJSON
// unconditionally instead. The mirrors must declare it or the dashboard cannot
// read it.
//
// This is a constant and not a list. A second divergence would mean somebody is
// hiding real drift behind an allowlist, which is the failure this script exists
// to prevent.
const KNOWN_DIVERGENCE = { type: "ReplyDraft", field: "requires_human_approval" };

/** Go structs, as `{ StructName: Map<jsonName, {optional}> }`. */
function parseGoStructs(source) {
  const structs = {};
  let current = null;
  let depth = 0;

  for (const line of source.split("\n")) {
    if (current === null) {
      const opening = /^type (\w+) struct \{/.exec(line);
      if (opening) {
        current = opening[1];
        structs[current] = new Map();
        depth = 1;
      }
      continue;
    }

    // Brace counting rather than a bare `}` test, so an inline anonymous struct
    // cannot end the struct early and silently drop every field after it.
    depth += (line.match(/\{/g) ?? []).length;
    depth -= (line.match(/\}/g) ?? []).length;
    if (depth <= 0) {
      current = null;
      continue;
    }

    const tag = /`json:"([^"]+)"/.exec(line);
    if (!tag) continue;
    const [name, ...options] = tag[1].split(",");
    if (name === "" || name === "-") continue;
    structs[current].set(name, { optional: options.includes("omitempty") });
  }

  return structs;
}

/** TS interfaces, as `{ InterfaceName: Map<fieldName, {optional}> }`. */
function parseTsInterfaces(source) {
  const interfaces = {};
  let current = null;

  for (const line of source.split("\n")) {
    if (current === null) {
      const opening = /^export interface (\w+) \{/.exec(line);
      if (opening) {
        current = opening[1];
        interfaces[current] = new Map();
      }
      continue;
    }

    if (line === "}") {
      current = null;
      continue;
    }

    const field = /^\s{2}(\w+)(\?)?:/.exec(line);
    if (!field) continue;
    interfaces[current].set(field[1], { optional: field[2] === "?" });
  }

  return interfaces;
}

function compare(mirrorPath, goStructs, tsInterfaces) {
  const problems = [];

  for (const [structName, goFields] of Object.entries(goStructs)) {
    const tsFields = tsInterfaces[structName];
    if (!tsFields) {
      problems.push(`${mirrorPath}: ${structName} has no interface. ${GO_MODELS} defines it.`);
      continue;
    }

    for (const [fieldName, go] of goFields) {
      const ts = tsFields.get(fieldName);
      if (!ts) {
        problems.push(`${mirrorPath}: ${structName}.${fieldName} is missing.`);
        continue;
      }
      if (ts.optional !== go.optional) {
        problems.push(
          go.optional
            ? `${mirrorPath}: ${structName}.${fieldName} must be optional. The Go tag says omitempty, so the key is absent when unset.`
            : `${mirrorPath}: ${structName}.${fieldName} must not be optional. The Go tag has no omitempty, so the key is always present.`,
        );
      }
    }

    for (const fieldName of tsFields.keys()) {
      if (goFields.has(fieldName)) continue;
      if (structName === KNOWN_DIVERGENCE.type && fieldName === KNOWN_DIVERGENCE.field) continue;
      problems.push(
        `${mirrorPath}: ${structName}.${fieldName} is not in ${GO_MODELS}. Nothing will ever populate it.`,
      );
    }
  }

  for (const interfaceName of Object.keys(tsInterfaces)) {
    if (interfaceName in goStructs) continue;
    problems.push(
      `${mirrorPath}: ${interfaceName} mirrors no struct in ${GO_MODELS}. Either it was renamed there or it does not belong in a mirror file.`,
    );
  }

  return problems;
}

const goStructs = parseGoStructs(readFileSync(repoRoot + GO_MODELS, "utf8"));
const structCount = Object.keys(goStructs).length;
if (structCount === 0) {
  console.error(`check:types: parsed no structs out of ${GO_MODELS}. The parser is broken, not the mirrors.`);
  process.exit(2);
}

const problems = MIRRORS.flatMap((mirrorPath) =>
  compare(mirrorPath, goStructs, parseTsInterfaces(readFileSync(repoRoot + mirrorPath, "utf8"))),
);

if (problems.length > 0) {
  console.error(`check:types: ${problems.length} divergence(s) from ${GO_MODELS}\n`);
  for (const problem of problems) console.error(`  ${problem}`);
  console.error(
    `\nFix the mirror, not ${GO_MODELS}. The contracts are frozen and change only on main, through B1.`,
  );
  process.exit(1);
}

console.log(
  `check:types: ${structCount} structs match across ${MIRRORS.join(" and ")}, ` +
    `with one documented divergence (${KNOWN_DIVERGENCE.type}.${KNOWN_DIVERGENCE.field}).`,
);
