// Package models is the wire format for every BrandPulse agent.
//
// This package is the single source of truth for the JSON that crosses agent
// boundaries. Every A2A artifact is one of these structs marshalled with
// encoding/json, so a json tag rename here is a breaking change for DronaHQ
// bindings, for the Fastify BFF, and for every downstream agent.
//
// This package must not import any agent, any Anakin client, or anything that
// touches the network or the database. It is pure schema.
//
// # Zero values are the hazard of this port
//
// Go has no field defaults. Python's pydantic gave BrandProfile.version a
// default of 1, Topic.Trend a default of 1.0 and Mention.Lang a default of
// "en"; here those decode as 0, 0.0 and "". Where a wrong zero is merely
// untidy, the New* constructors carry the intended value. Where a wrong zero
// would be unsafe, the type prevents it outright: ReplyDraft has no
// requires_human_approval field at all, because it is an invariant rather than
// state, and its MarshalJSON always emits true.
//
// One consequence: ReplyDraft does not round-trip. Marshal emits
// requires_human_approval, Unmarshal has nowhere to put it and drops it. That
// is intended. Any code that reads the field back off the wire is asking a
// question whose answer is always true.
//
// Call Validate before persisting anything that has one.
package models

import (
	"encoding/json"
	"fmt"
	"time"
)

// ---------------------------------------------------------------------------
// Enumerations. Values are the Postgres enum labels; do not rename one without
// a migration in the same commit.
// ---------------------------------------------------------------------------

// Source is a public surface BrandPulse collects from.
//
// The enum is wider than what any brand actually enables. SourceX,
// SourceInstagram and SourceFlipkart exist because the schema keeps them, but
// Anakin's Wire catalogue does not carry them as mention sources. See
// docs/SOURCE-STRATEGY.md before enabling one.
type Source string

const (
	SourceX         Source = "x"
	SourceReddit    Source = "reddit"
	SourceYoutube   Source = "youtube"
	SourceNews      Source = "news"
	SourcePlaystore Source = "playstore"
	SourceAppstore  Source = "appstore"
	SourceAmazon    Source = "amazon"
	SourceFlipkart  Source = "flipkart"
	SourceInstagram Source = "instagram"
	SourceWeb       Source = "web"
)

// Valid reports whether s is a member of the enum. An invalid Source reaching
// Postgres is an insert error, so adapters check before emitting.
//
// A switch rather than a lookup table because the values are then listed once,
// here and in the const block above, instead of a third time in a slice that a
// new source can be left out of silently.
func (s Source) Valid() bool {
	switch s {
	case SourceX, SourceReddit, SourceYoutube, SourceNews, SourcePlaystore,
		SourceAppstore, SourceAmazon, SourceFlipkart, SourceInstagram, SourceWeb:
		return true
	}
	return false
}

// IsReviewSource reports whether this source carries a star rating. The
// review_bomb detector rule reads Mention.Rating, which only these three set.
func (s Source) IsReviewSource() bool {
	return s == SourceAmazon || s == SourceAppstore || s == SourcePlaystore
}

type SentimentLabel string

const (
	SentimentNegative SentimentLabel = "negative"
	SentimentNeutral  SentimentLabel = "neutral"
	SentimentPositive SentimentLabel = "positive"
	SentimentMixed    SentimentLabel = "mixed"
)

type Intent string

const (
	IntentComplaint      Intent = "complaint"
	IntentPraise         Intent = "praise"
	IntentQuestion       Intent = "question"
	IntentPurchaseIntent Intent = "purchase_intent"
	IntentComparison     Intent = "comparison"
	IntentSpam           Intent = "spam"
	IntentNews           Intent = "news"
	IntentOther          Intent = "other"
)

type Emotion string

const (
	EmotionAnger    Emotion = "anger"
	EmotionJoy      Emotion = "joy"
	EmotionSadness  Emotion = "sadness"
	EmotionFear     Emotion = "fear"
	EmotionDisgust  Emotion = "disgust"
	EmotionSurprise Emotion = "surprise"
	EmotionNeutral  Emotion = "neutral"
)

type AlertKind string

const (
	AlertSpike             AlertKind = "spike"
	AlertCrisis            AlertKind = "crisis"
	AlertInfluencerMention AlertKind = "influencer_mention"
	AlertCompetitorMove    AlertKind = "competitor_move"
	AlertReviewBomb        AlertKind = "review_bomb"
)

type Severity string

const (
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

type AlertStatus string

const (
	AlertStatusOpen     AlertStatus = "open"
	AlertStatusAcked    AlertStatus = "acked"
	AlertStatusSnoozed  AlertStatus = "snoozed"
	AlertStatusResolved AlertStatus = "resolved"
)

type RunKind string

const (
	RunOnboard      RunKind = "onboard"
	RunScheduled    RunKind = "scheduled"
	RunOnDemand     RunKind = "on_demand"
	RunCrisisReplay RunKind = "crisis_replay"
)

type RunStatus string

const (
	RunStatusRunning RunStatus = "running"
	RunStatusOK      RunStatus = "ok"
	RunStatusPartial RunStatus = "partial"
	RunStatusFailed  RunStatus = "failed"
)

// DraftStatus is moved by a human in DronaHQ. Nothing in this repo advances it
// past approved, because nothing in this repo posts.
type DraftStatus string

const (
	DraftStatusDraft    DraftStatus = "draft"
	DraftStatusApproved DraftStatus = "approved"
	DraftStatusRejected DraftStatus = "rejected"
	DraftStatusSent     DraftStatus = "sent"
)

// ---------------------------------------------------------------------------
// Brand profile. Produced by bp-onboarder, consumed by everything.
// ---------------------------------------------------------------------------

// BrandVoice holds the style knobs bp-responder uses when drafting. Free text
// on purpose: an enum here would make the responder's prompt worse, not better.
type BrandVoice struct {
	Tone      string   `json:"tone"`
	Language  string   `json:"language"`
	Signature string   `json:"signature,omitempty"`
	DoNotSay  []string `json:"do_not_say"`
}

// NewBrandVoice carries the defaults pydantic used to supply.
func NewBrandVoice() BrandVoice {
	return BrandVoice{
		Tone:     "warm, direct, no corporate filler",
		Language: "English with light Hinglish if the mention is Hinglish",
		DoNotSay: []string{},
	}
}

// BrandProfile is the keyword set and rules that drive collection for one
// brand.
//
// Version increments on every user-confirmed edit in DronaHQ. Mentions record
// the profile version that found them, so a keyword change stays auditable.
type BrandProfile struct {
	BrandID  string   `json:"brand_id"`
	Name     string   `json:"name"`
	Website  string   `json:"website,omitempty"`
	Keywords []string `json:"keywords"`
	Hashtags []string `json:"hashtags"`
	Products []string `json:"products"`

	Competitors []string `json:"competitors"`
	Sources     []Source `json:"sources"`

	// NegativeKeywords are terms that, when present, mean the mention is NOT
	// about this brand. Applied in the source adapter, before a mention can
	// cost an enrichment token.
	NegativeKeywords []string `json:"negative_keywords"`

	// SourceHandles carries the ids keyword search cannot supply: an App Store
	// app id, a Play package name, an Amazon ASIN. Keyed by Source value.
	SourceHandles map[string]string `json:"source_handles"`

	Voice   BrandVoice `json:"voice"`
	Version int        `json:"version"`
}

// NewBrandProfile returns a profile with the non-zero defaults filled in.
// Version starts at 1, because version 0 means "never onboarded" in the DB.
func NewBrandProfile(brandID, name string) BrandProfile {
	return BrandProfile{
		BrandID:          brandID,
		Name:             name,
		Keywords:         []string{},
		Hashtags:         []string{},
		Products:         []string{},
		Competitors:      []string{},
		Sources:          []Source{},
		NegativeKeywords: []string{},
		SourceHandles:    map[string]string{},
		Voice:            NewBrandVoice(),
		Version:          1,
	}
}

func (p BrandProfile) Validate() error {
	if p.BrandID == "" {
		return fmt.Errorf("models: BrandProfile.brand_id is required")
	}
	if p.Version < 1 {
		return fmt.Errorf("models: BrandProfile.version must be >= 1, got %d", p.Version)
	}
	for _, s := range p.Sources {
		if !s.Valid() {
			return fmt.Errorf("models: BrandProfile.sources has unknown source %q", s)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Mentions. Produced by bp-collector.
// ---------------------------------------------------------------------------

type Engagement struct {
	Likes   int `json:"likes"`
	Replies int `json:"replies"`
	Shares  int `json:"shares"`
	Views   int `json:"views"`
}

// Total deliberately excludes Views: a view is not an interaction, and
// including it would let one viral video outrank every real conversation when
// bp-clusterer picks top examples.
func (e Engagement) Total() int {
	return e.Likes + e.Replies + e.Shares
}

// Mention is one public post, comment, review or article that matched a
// keyword.
//
// ContentHash is sha256 of normalised text plus source and is the dedupe key.
// The same Reddit thread found by two different keywords must produce one row,
// which mentions.UNIQUE (brand_id, content_hash) enforces.
type Mention struct {
	ID         string `json:"id"`
	BrandID    string `json:"brand_id"`
	Source     Source `json:"source"`
	ExternalID string `json:"external_id"`
	URL        string `json:"url,omitempty"`
	Author     string `json:"author,omitempty"`

	// AuthorFollowers is 0 when the source does not expose it. Do not invent a
	// value: the influencer_mention rule fires on this field.
	AuthorFollowers int `json:"author_followers"`

	Text string `json:"text"`
	Lang string `json:"lang"`

	// PostedAt must be timezone-aware UTC. Every source formats dates
	// differently; adapters normalise here so nothing downstream has to.
	PostedAt   time.Time  `json:"posted_at"`
	Engagement Engagement `json:"engagement"`

	// Rating is the star rating, set only by review sources. Nil rather than 0
	// when absent, because a 0 would read as the worst possible review.
	Rating *float64 `json:"rating,omitempty"`

	MatchedKeyword string         `json:"matched_keyword,omitempty"`
	ContentHash    string         `json:"content_hash"`
	Raw            map[string]any `json:"raw,omitempty"`
}

func (m Mention) Validate() error {
	if m.ID == "" || m.BrandID == "" {
		return fmt.Errorf("models: Mention.id and brand_id are required")
	}
	if !m.Source.Valid() {
		return fmt.Errorf("models: Mention.source %q is not a known source", m.Source)
	}
	if m.ContentHash == "" {
		return fmt.Errorf("models: Mention.content_hash is required, it is the dedupe key")
	}
	if m.PostedAt.IsZero() {
		return fmt.Errorf("models: Mention.posted_at is required, baselines are built from it")
	}
	// Catches the zero value pydantic used to fill with "en". An empty lang
	// silently drops the mention from every language-filtered query.
	if m.Lang == "" {
		return fmt.Errorf("models: Mention %q has no lang; set it explicitly, there is no default", m.ID)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Enrichment. Produced by bp-enricher.
// ---------------------------------------------------------------------------

// Enrichment is classifier output for one mention, cached by the mention's
// content hash so a re-run over the same corpus costs no tokens.
type Enrichment struct {
	MentionID      string         `json:"mention_id"`
	Sentiment      float64        `json:"sentiment"`
	SentimentLabel SentimentLabel `json:"sentiment_label"`
	Emotion        Emotion        `json:"emotion"`
	Intent         Intent         `json:"intent"`

	// Aspects are the facets the text is about: "delivery", "price",
	// "packaging".
	Aspects []string `json:"aspects"`

	// IsAboutBrand is false when the keyword matched something unrelated, which
	// is the brand-name collision case: "Mamaearth" the brand against "mama
	// earth" in an unrelated sentence.
	IsAboutBrand bool `json:"is_about_brand"`

	// AboutCompetitor is set when the mention is about a competitor instead.
	AboutCompetitor string `json:"about_competitor,omitempty"`

	Model     string  `json:"model"`
	CostPaise float64 `json:"cost_paise"`
}

func (e Enrichment) Validate() error {
	if e.MentionID == "" {
		return fmt.Errorf("models: Enrichment.mention_id is required")
	}
	if e.Sentiment < -1.0 || e.Sentiment > 1.0 {
		return fmt.Errorf("models: Enrichment.sentiment must be within [-1, 1], got %v", e.Sentiment)
	}
	return nil
}

// EnrichedMention is a Mention joined to its Enrichment, the unit every
// analytic agent reads.
type EnrichedMention struct {
	Mention    Mention    `json:"mention"`
	Enrichment Enrichment `json:"enrichment"`
}

// ---------------------------------------------------------------------------
// Topics. Produced by bp-clusterer.
// ---------------------------------------------------------------------------

type Topic struct {
	ID          string    `json:"id"`
	BrandID     string    `json:"brand_id"`
	WindowStart time.Time `json:"window_start"`
	WindowEnd   time.Time `json:"window_end"`
	Label       string    `json:"label"`
	Summary     string    `json:"summary"`
	Size        int       `json:"size"`

	SentimentMix map[SentimentLabel]int `json:"sentiment_mix"`

	// TopExamples holds at most 3 mentions, chosen by engagement.
	TopExamples []Mention `json:"top_examples"`

	// Trend is this window's size over the prior window's. 1.0 means flat, and
	// 1.0 is also the right answer when there is no prior window, so this is
	// the field most likely to be wrong if you build a Topic literal by hand.
	Trend float64 `json:"trend"`

	MentionIDs []string `json:"mention_ids"`
}

// NewTopic returns a Topic with Trend set to flat rather than Go's zero.
func NewTopic(id, brandID string) Topic {
	return Topic{
		ID:           id,
		BrandID:      brandID,
		SentimentMix: map[SentimentLabel]int{},
		TopExamples:  []Mention{},
		Trend:        1.0,
		MentionIDs:   []string{},
	}
}

// Validate catches the zero Trend, which is the failure NewTopic exists to
// prevent and which a decoder bypasses entirely. A trend of 0 would render as
// a topic that vanished, when it means nobody set the field.
func (t Topic) Validate() error {
	if t.ID == "" || t.BrandID == "" {
		return fmt.Errorf("models: Topic.id and brand_id are required")
	}
	if t.Trend <= 0 {
		return fmt.Errorf("models: Topic %q has trend %v; 1.0 means flat, 0 means unset", t.ID, t.Trend)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Alerts. Produced by bp-detector, which is deterministic and never calls an
// LLM.
// ---------------------------------------------------------------------------

// AlertEvidence is one statistical fact that contributed to an alert firing.
//
// Every alert carries the numbers that fired it so the UI can show the rule,
// instead of asserting that "AI detected a crisis". Value and Threshold are
// the real observed number and the real constant it was compared against.
type AlertEvidence struct {
	Metric    string  `json:"metric"`
	Value     float64 `json:"value"`
	Threshold float64 `json:"threshold"`
	Window    string  `json:"window"`
	Detail    string  `json:"detail,omitempty"`
}

type Alert struct {
	ID       string    `json:"id"`
	BrandID  string    `json:"brand_id"`
	Kind     AlertKind `json:"kind"`
	Severity Severity  `json:"severity"`
	Title    string    `json:"title"`

	// Why is a plain-English statement of the rule that fired, not a summary of
	// the mentions.
	Why string `json:"why"`

	Evidence []AlertEvidence `json:"evidence"`

	// SampleMentions carries whole mentions on the wire while the alerts table
	// stores sample_mention_ids. That divergence is deliberate: DronaHQ renders
	// an alert without a second fetch. See docs/CONTRACTS.md section 3b.
	SampleMentions []Mention `json:"sample_mentions"`

	Status AlertStatus `json:"status"`

	// DedupeKey suppresses a duplicate alert for the same rule in the same
	// bucket, formatted "{kind}:{source}:{hour_bucket}". It backs
	// alerts.UNIQUE (brand_id, dedupe_key) and the column is NOT NULL, so an
	// empty value is an insert failure rather than a missing nicety.
	DedupeKey string `json:"dedupe_key"`

	CreatedAt time.Time `json:"created_at"`
}

// NewAlert returns an alert in the only status a detector may create, with its
// slices ready.
//
// Kind, severity and dedupe key are arguments rather than defaults because
// there is no safe default for any of them: dedupe_key backs a NOT NULL unique
// constraint, and a rule that fired knows its own kind and severity. CreatedAt
// is an argument for the same reason bp-detector takes Now as an input — a
// replayed run must produce the same alert it produced live, and a constructor
// that read the clock would break that.
//
// Evidence starts empty and Validate rejects it that way: every alert must
// show the numbers that fired it.
func NewAlert(id, brandID string, kind AlertKind, severity Severity, dedupeKey string, createdAt time.Time) Alert {
	return Alert{
		ID:             id,
		BrandID:        brandID,
		Kind:           kind,
		Severity:       severity,
		Evidence:       []AlertEvidence{},
		SampleMentions: []Mention{},
		Status:         AlertStatusOpen,
		DedupeKey:      dedupeKey,
		CreatedAt:      createdAt,
	}
}

func (a Alert) Validate() error {
	if a.ID == "" || a.BrandID == "" {
		return fmt.Errorf("models: Alert.id and brand_id are required")
	}
	if a.DedupeKey == "" {
		return fmt.Errorf("models: Alert.dedupe_key is required, it backs the unique constraint")
	}
	if len(a.Evidence) == 0 {
		return fmt.Errorf("models: Alert %q has no evidence, every alert must show the rule that fired", a.ID)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Reply drafts. Produced by bp-responder. Never auto-posted.
// ---------------------------------------------------------------------------

// Channel is where a draft is meant to go. It holds either a Source value or
// one of the three constants below.
type Channel string

const (
	ChannelWhatsapp  Channel = "whatsapp"
	ChannelEmail     Channel = "email"
	ChannelStatement Channel = "statement"
)

// ReplyDraft is a suggested response awaiting a human.
//
// There is deliberately no RequiresHumanApproval field. It is an invariant
// rather than state, and a bool field would decode to false from any payload
// that omitted it. MarshalJSON emits requires_human_approval: true
// unconditionally, so no serialisation of this type can ever claim otherwise.
type ReplyDraft struct {
	ID        string  `json:"id"`
	AlertID   string  `json:"alert_id,omitempty"`
	MentionID string  `json:"mention_id,omitempty"`
	Channel   Channel `json:"channel"`
	Text      string  `json:"text"`
	Tone      string  `json:"tone"`

	// DoNotSay echoes the guardrail phrases that were forbidden when drafting,
	// kept for audit.
	DoNotSay []string    `json:"do_not_say"`
	Status   DraftStatus `json:"status"`
}

// NewReplyDraft returns a draft in the only status this repo may create.
func NewReplyDraft(id string, channel Channel) ReplyDraft {
	return ReplyDraft{
		ID:       id,
		Channel:  channel,
		DoNotSay: []string{},
		Status:   DraftStatusDraft,
	}
}

// MarshalJSON emits the requires_human_approval invariant alongside the fields.
func (d ReplyDraft) MarshalJSON() ([]byte, error) {
	type wire ReplyDraft // sheds the method set, so this does not recurse
	return json.Marshal(struct {
		wire
		RequiresHumanApproval bool `json:"requires_human_approval"`
	}{wire(d), true})
}

func (d ReplyDraft) Validate() error {
	if d.ID == "" {
		return fmt.Errorf("models: ReplyDraft.id is required")
	}
	// Mirrors reply_drafts CHECK (alert_id IS NOT NULL OR mention_id IS NOT NULL).
	if d.AlertID == "" && d.MentionID == "" {
		return fmt.Errorf("models: ReplyDraft %q must reference an alert or a mention", d.ID)
	}
	if d.Text == "" {
		return fmt.Errorf("models: ReplyDraft %q has no text", d.ID)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Share of voice. Produced by bp-sov, pure counting, no LLM.
// ---------------------------------------------------------------------------

type ShareOfVoice struct {
	BrandID     string    `json:"brand_id"`
	WindowStart time.Time `json:"window_start"`
	WindowEnd   time.Time `json:"window_end"`

	// BrandShare and CompetitorShares are percentages summing to about 100.
	// Mentions about neither the brand nor a listed competitor are excluded
	// from the denominator rather than counted as brand mentions.
	BrandShare       float64            `json:"brand_share"`
	CompetitorShares map[string]float64 `json:"competitor_shares"`

	BySource      map[Source]map[string]float64 `json:"by_source"`
	TotalMentions int                           `json:"total_mentions"`
}

// NewShareOfVoice returns a zeroed result with its maps ready, which is also
// the correct answer for a window containing no mentions.
func NewShareOfVoice(brandID string, start, end time.Time) ShareOfVoice {
	return ShareOfVoice{
		BrandID:          brandID,
		WindowStart:      start,
		WindowEnd:        end,
		CompetitorShares: map[string]float64{},
		BySource:         map[Source]map[string]float64{},
	}
}

// ---------------------------------------------------------------------------
// Daily brief. Produced by bp-briefer.
// ---------------------------------------------------------------------------

// BriefNumbers is computed deterministically. The briefer's LLM call writes the
// headline and the suggested actions; it is never asked to restate a number.
type BriefNumbers struct {
	Mentions         int     `json:"mentions"`
	MentionsDeltaPct float64 `json:"mentions_delta_pct"`
	SentimentAvg     float64 `json:"sentiment_avg"`
	NegativeShare    float64 `json:"negative_share"`
	ShareOfVoice     float64 `json:"share_of_voice"`
}

// WhatsappShortLimit is the hard cap on DailyBrief.WhatsappShort.
const WhatsappShortLimit = 600

type DailyBrief struct {
	BrandID     string       `json:"brand_id"`
	PeriodStart time.Time    `json:"period_start"`
	PeriodEnd   time.Time    `json:"period_end"`
	Headline    string       `json:"headline"`
	Numbers     BriefNumbers `json:"numbers"`

	TopTopics        []Topic  `json:"top_topics"`
	Alerts           []Alert  `json:"alerts"`
	CompetitorWatch  []string `json:"competitor_watch"`
	SuggestedActions []string `json:"suggested_actions"`

	Markdown string `json:"markdown"`

	// WhatsappShort is at most WhatsappShortLimit characters, with no markdown
	// tables and no links.
	WhatsappShort string `json:"whatsapp_short"`
}

// NewDailyBrief returns a brief for one period with its slices ready, which is
// also the correct shape for a period in which nothing happened.
//
// It fills no text. Headline, Markdown and WhatsappShort come from the
// briefer's one LLM call, and Numbers is computed deterministically before
// that call rather than asked of the model.
func NewDailyBrief(brandID string, periodStart, periodEnd time.Time) DailyBrief {
	return DailyBrief{
		BrandID:          brandID,
		PeriodStart:      periodStart,
		PeriodEnd:        periodEnd,
		TopTopics:        []Topic{},
		Alerts:           []Alert{},
		CompetitorWatch:  []string{},
		SuggestedActions: []string{},
	}
}

func (b DailyBrief) Validate() error {
	if b.BrandID == "" {
		return fmt.Errorf("models: DailyBrief.brand_id is required")
	}
	if len(b.WhatsappShort) > WhatsappShortLimit {
		return fmt.Errorf(
			"models: DailyBrief.whatsapp_short is %d chars, limit is %d",
			len(b.WhatsappShort), WhatsappShortLimit,
		)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Run accounting. Produced by bp-orchestrator, shown on the cost dashboard.
// ---------------------------------------------------------------------------

// RunRecord is one pipeline execution. Its credit, token and rupee fields are
// what eval/cost reads, so they carry measured values and never estimates.
type RunRecord struct {
	ID      string  `json:"id"`
	BrandID string  `json:"brand_id"`
	Kind    RunKind `json:"kind"`

	// TimeBucket is the idempotency key backing
	// runs.UNIQUE (brand_id, kind, time_bucket). Produced by stats.HourBucket
	// or stats.DayBucket. The column is NOT NULL.
	TimeBucket string `json:"time_bucket"`

	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	Status     RunStatus  `json:"status"`

	CreditsUsed int     `json:"credits_used"`
	TokensUsed  int     `json:"tokens_used"`
	CostPaise   float64 `json:"cost_paise"`

	SourcesAttempted []Source `json:"sources_attempted"`
	SourcesSkipped   []Source `json:"sources_skipped"`

	// DegradedReason is set when the Nasiko flow guard or the credit budget cut
	// the fan-out. A run that degrades is a partial run, not a failed one.
	DegradedReason string `json:"degraded_reason,omitempty"`

	MentionsCollected int      `json:"mentions_collected"`
	Errors            []string `json:"errors"`
}

// NewRunRecord returns a run in the running state with its slices ready.
func NewRunRecord(id, brandID string, kind RunKind, timeBucket string, startedAt time.Time) RunRecord {
	return RunRecord{
		ID:               id,
		BrandID:          brandID,
		Kind:             kind,
		TimeBucket:       timeBucket,
		StartedAt:        startedAt,
		Status:           RunStatusRunning,
		SourcesAttempted: []Source{},
		SourcesSkipped:   []Source{},
		Errors:           []string{},
	}
}

func (r RunRecord) Validate() error {
	if r.ID == "" || r.BrandID == "" {
		return fmt.Errorf("models: RunRecord.id and brand_id are required")
	}
	if r.TimeBucket == "" {
		return fmt.Errorf("models: RunRecord.time_bucket is required, it backs the unique constraint")
	}
	if r.Status == "" {
		return fmt.Errorf("models: RunRecord %q has no status", r.ID)
	}
	return nil
}
