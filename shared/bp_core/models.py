"""Wire format for every BrandPulse agent.

This module is the single source of truth for the JSON that crosses agent
boundaries. Every A2A artifact is one of these models dumped with
`model_dump(mode="json")`, so a field rename here is a breaking change for
DronaHQ bindings and for every downstream agent.

This module must not import any agent, any Anakin client, or anything that
touches the network or the database. It is pure schema.
"""

from __future__ import annotations

from datetime import datetime
from enum import StrEnum
from typing import Any, Literal

from pydantic import BaseModel, Field, HttpUrl


# --------------------------------------------------------------------------
# Enumerations
# --------------------------------------------------------------------------


class Source(StrEnum):
    """A public surface BrandPulse collects from. Value is the DB column value."""

    X = "x"
    REDDIT = "reddit"
    YOUTUBE = "youtube"
    NEWS = "news"
    PLAYSTORE = "playstore"
    APPSTORE = "appstore"
    AMAZON = "amazon"
    FLIPKART = "flipkart"
    INSTAGRAM = "instagram"
    WEB = "web"


class SentimentLabel(StrEnum):
    NEGATIVE = "negative"
    NEUTRAL = "neutral"
    POSITIVE = "positive"
    MIXED = "mixed"


class Intent(StrEnum):
    COMPLAINT = "complaint"
    PRAISE = "praise"
    QUESTION = "question"
    PURCHASE_INTENT = "purchase_intent"
    COMPARISON = "comparison"
    SPAM = "spam"
    NEWS = "news"
    OTHER = "other"


class Emotion(StrEnum):
    ANGER = "anger"
    JOY = "joy"
    SADNESS = "sadness"
    FEAR = "fear"
    DISGUST = "disgust"
    SURPRISE = "surprise"
    NEUTRAL = "neutral"


class AlertKind(StrEnum):
    SPIKE = "spike"
    CRISIS = "crisis"
    INFLUENCER_MENTION = "influencer_mention"
    COMPETITOR_MOVE = "competitor_move"
    REVIEW_BOMB = "review_bomb"


class Severity(StrEnum):
    LOW = "low"
    MEDIUM = "medium"
    HIGH = "high"
    CRITICAL = "critical"


class AlertStatus(StrEnum):
    OPEN = "open"
    ACKED = "acked"
    SNOOZED = "snoozed"
    RESOLVED = "resolved"


class RunKind(StrEnum):
    ONBOARD = "onboard"
    SCHEDULED = "scheduled"
    ON_DEMAND = "on_demand"
    CRISIS_REPLAY = "crisis_replay"


class RunStatus(StrEnum):
    RUNNING = "running"
    OK = "ok"
    PARTIAL = "partial"
    FAILED = "failed"


# --------------------------------------------------------------------------
# Brand profile  (produced by bp-onboarder, consumed by everything)
# --------------------------------------------------------------------------


class BrandVoice(BaseModel):
    """Style knobs bp-responder uses when drafting. Free text, no enum."""

    tone: str = "warm, direct, no corporate filler"
    language: str = "English with light Hinglish if the mention is Hinglish"
    signature: str | None = None
    do_not_say: list[str] = Field(default_factory=list)


class BrandProfile(BaseModel):
    """The keyword set and rules that drive collection for one brand.

    `version` increments on every user-confirmed edit in DronaHQ; mentions
    record the profile version that found them so a keyword change is auditable.
    """

    brand_id: str
    name: str
    website: HttpUrl | None = None
    keywords: list[str] = Field(default_factory=list)
    hashtags: list[str] = Field(default_factory=list)
    products: list[str] = Field(default_factory=list)
    competitors: list[str] = Field(default_factory=list)
    sources: list[Source] = Field(default_factory=list)
    # Terms that, when present, mean the mention is NOT about this brand.
    negative_keywords: list[str] = Field(default_factory=list)
    # Play Store / App Store / Amazon need an app or product id, not a keyword.
    source_handles: dict[str, str] = Field(default_factory=dict)
    voice: BrandVoice = Field(default_factory=BrandVoice)
    version: int = 1


# --------------------------------------------------------------------------
# Mentions  (produced by bp-collector)
# --------------------------------------------------------------------------


class Engagement(BaseModel):
    likes: int = 0
    replies: int = 0
    shares: int = 0
    views: int = 0

    @property
    def total(self) -> int:
        return self.likes + self.replies + self.shares


class Mention(BaseModel):
    """One public post, comment, review or article that matched a keyword.

    `content_hash` is sha256 of normalised text + source and is the dedupe key;
    the same Reddit thread found by two keywords must produce one row.
    """

    id: str
    brand_id: str
    source: Source
    external_id: str
    url: HttpUrl | None = None
    author: str | None = None
    author_followers: int = 0
    text: str
    lang: str = "en"
    posted_at: datetime
    engagement: Engagement = Field(default_factory=Engagement)
    # Star rating, present only for review sources (playstore/appstore/amazon).
    rating: float | None = None
    matched_keyword: str | None = None
    content_hash: str
    raw: dict[str, Any] = Field(default_factory=dict)


# --------------------------------------------------------------------------
# Enrichment  (produced by bp-enricher)
# --------------------------------------------------------------------------


class Enrichment(BaseModel):
    """Classifier output for one mention. Cached by mention content_hash."""

    mention_id: str
    sentiment: float = Field(ge=-1.0, le=1.0)
    sentiment_label: SentimentLabel
    emotion: Emotion = Emotion.NEUTRAL
    intent: Intent = Intent.OTHER
    # Product/service facets the text is about: "delivery", "price", "packaging".
    aspects: list[str] = Field(default_factory=list)
    # False when the keyword matched something unrelated (brand-name collision).
    is_about_brand: bool = True
    # Set when the mention is about a competitor rather than the brand.
    about_competitor: str | None = None
    model: str = ""
    cost_paise: float = 0.0


class EnrichedMention(BaseModel):
    """A Mention joined to its Enrichment. The unit every analytic agent reads."""

    mention: Mention
    enrichment: Enrichment


# --------------------------------------------------------------------------
# Topics  (produced by bp-clusterer)
# --------------------------------------------------------------------------


class Topic(BaseModel):
    id: str
    brand_id: str
    window_start: datetime
    window_end: datetime
    label: str
    summary: str
    size: int
    sentiment_mix: dict[SentimentLabel, int] = Field(default_factory=dict)
    top_examples: list[Mention] = Field(default_factory=list)
    # Ratio of this window's size to the prior window's; 1.0 means flat.
    trend: float = 1.0
    mention_ids: list[str] = Field(default_factory=list)


# --------------------------------------------------------------------------
# Alerts  (produced by bp-detector — deterministic, never LLM)
# --------------------------------------------------------------------------


class AlertEvidence(BaseModel):
    """One statistical fact that contributed to an alert firing.

    Every alert must carry the numbers that fired it so the UI can show the
    rule instead of asserting that "AI detected a crisis".
    """

    metric: str
    value: float
    threshold: float
    window: str
    detail: str = ""


class Alert(BaseModel):
    id: str
    brand_id: str
    kind: AlertKind
    severity: Severity
    title: str
    # Plain-English statement of the rule that fired.
    why: str
    evidence: list[AlertEvidence] = Field(default_factory=list)
    sample_mentions: list[Mention] = Field(default_factory=list)
    status: AlertStatus = AlertStatus.OPEN
    # Suppresses a duplicate alert for the same rule in the same bucket.
    # Format is f"{kind}:{source}:{hour_bucket}" and it backs
    # alerts.UNIQUE (brand_id, dedupe_key), so it is required, not optional.
    dedupe_key: str
    created_at: datetime


# --------------------------------------------------------------------------
# Reply drafts  (produced by bp-responder — never auto-posted)
# --------------------------------------------------------------------------


class ReplyDraft(BaseModel):
    id: str
    alert_id: str | None = None
    mention_id: str | None = None
    channel: Source | Literal["whatsapp", "email", "statement"]
    text: str
    tone: str
    # Guardrail phrases that were forbidden when drafting, echoed for audit.
    do_not_say: list[str] = Field(default_factory=list)
    # Moved by a human in DronaHQ. Nothing in this repo advances it past
    # "approved", because nothing in this repo posts.
    status: Literal["draft", "approved", "rejected", "sent"] = "draft"
    requires_human_approval: bool = True


# --------------------------------------------------------------------------
# Share of voice  (produced by bp-sov — pure counting)
# --------------------------------------------------------------------------


class ShareOfVoice(BaseModel):
    brand_id: str
    window_start: datetime
    window_end: datetime
    # Percentages summing to ~100 across brand + competitors.
    brand_share: float
    competitor_shares: dict[str, float] = Field(default_factory=dict)
    by_source: dict[Source, dict[str, float]] = Field(default_factory=dict)
    total_mentions: int = 0


# --------------------------------------------------------------------------
# Daily brief  (produced by bp-briefer)
# --------------------------------------------------------------------------


class BriefNumbers(BaseModel):
    mentions: int = 0
    mentions_delta_pct: float = 0.0
    sentiment_avg: float = 0.0
    negative_share: float = 0.0
    share_of_voice: float = 0.0


class DailyBrief(BaseModel):
    brand_id: str
    period_start: datetime
    period_end: datetime
    headline: str
    numbers: BriefNumbers
    top_topics: list[Topic] = Field(default_factory=list)
    alerts: list[Alert] = Field(default_factory=list)
    competitor_watch: list[str] = Field(default_factory=list)
    suggested_actions: list[str] = Field(default_factory=list)
    markdown: str = ""
    # <=600 chars, WhatsApp-safe, no markdown tables.
    whatsapp_short: str = ""


# --------------------------------------------------------------------------
# Run accounting  (produced by bp-orchestrator, shown on the cost dashboard)
# --------------------------------------------------------------------------


class RunRecord(BaseModel):
    id: str
    brand_id: str
    kind: RunKind
    # Idempotency key, backing runs.UNIQUE (brand_id, kind, time_bucket).
    # Produced by bp_core.stats.hour_bucket or day_bucket.
    time_bucket: str
    started_at: datetime
    finished_at: datetime | None = None
    status: RunStatus = RunStatus.RUNNING
    credits_used: int = 0
    tokens_used: int = 0
    cost_paise: float = 0.0
    sources_attempted: list[Source] = Field(default_factory=list)
    sources_skipped: list[Source] = Field(default_factory=list)
    # Set when the Nasiko flow guard or the credit budget cut the fan-out.
    degraded_reason: str | None = None
    mentions_collected: int = 0
    errors: list[str] = Field(default_factory=list)
