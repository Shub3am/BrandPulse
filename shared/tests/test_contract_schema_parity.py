"""Guards the Phase 0 contract freeze.

`bp_core.models` is the wire format and `db/migrations/001_init.sql` is storage.
They are allowed to differ, but only in the ways documented in
docs/CONTRACTS.md section 3b. Anything else is a field that exists on one side
and not the other, which surfaces as a NOT NULL insert failure in an agent
nobody is looking at.

This file must not import an agent, open a socket, or touch a database.
"""

from __future__ import annotations

import re
from pathlib import Path

import pytest
from pydantic import BaseModel

from bp_core import models

MIGRATION = Path(__file__).resolve().parents[2] / "db" / "migrations" / "001_init.sql"

# Constraint keywords that open a line inside CREATE TABLE but are not columns.
_NOT_A_COLUMN = {"UNIQUE", "PRIMARY", "FOREIGN", "CHECK", "CONSTRAINT"}

# (model, table, fields allowed only on the model, columns allowed only in SQL).
# Every entry here is justified in docs/CONTRACTS.md section 3b. Adding a row
# without adding the reason there is how the contract rots.
PARITY_CASES: list[tuple[type[BaseModel], str, set[str], set[str]]] = [
    (models.BrandProfile, "brand_profiles", {"name", "website"}, {"confirmed_at", "created_at"}),
    (models.Mention, "mentions", set(), {"collected_at"}),
    (models.Enrichment, "mention_enrichment", set(), {"created_at"}),
    (models.Topic, "topics", {"mention_ids", "top_examples"}, {"created_at"}),
    (models.Alert, "alerts", {"sample_mentions"}, {"sample_mention_ids"}),
    (models.ReplyDraft, "reply_drafts", {"requires_human_approval"}, {"created_at"}),
    (models.RunRecord, "runs", set(), set()),
]


def _parse_columns(body: str) -> set[str]:
    columns = set()
    for line in body.splitlines():
        line = line.strip()
        if not line or line.startswith("--"):
            continue
        first = line.split()[0]
        if first.upper() in _NOT_A_COLUMN:
            continue
        columns.add(first)
    return columns


TABLE_COLUMNS: dict[str, set[str]] = {
    table: _parse_columns(body)
    for table, body in re.findall(
        r"CREATE TABLE (\w+) \((.*?)\n\);", MIGRATION.read_text(), re.S
    )
}


@pytest.mark.parametrize(
    ("model", "table", "model_only", "sql_only"),
    PARITY_CASES,
    ids=[case[1] for case in PARITY_CASES],
)
def test_model_matches_table(
    model: type[BaseModel], table: str, model_only: set[str], sql_only: set[str]
) -> None:
    fields = set(model.model_fields)
    columns = TABLE_COLUMNS[table]
    assert (fields - columns) - model_only == set(), (
        f"{model.__name__} has fields with no column in {table}"
    )
    assert (columns - fields) - sql_only == set(), (
        f"{table} has columns with no field on {model.__name__}"
    )


def test_idempotency_keys_are_required():
    """Both unique constraints are NOT NULL, so neither field may be optional."""
    assert models.Alert.model_fields["dedupe_key"].is_required()
    assert models.RunRecord.model_fields["time_bucket"].is_required()


def test_a_reply_draft_always_requires_human_approval():
    draft = models.ReplyDraft(id="d1", alert_id="a1", channel="whatsapp", text="hi", tone="calm")
    assert draft.requires_human_approval is True
    assert draft.status == "draft"
