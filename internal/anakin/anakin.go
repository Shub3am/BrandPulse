// Package anakin is the only code in this repository that fetches public data.
//
// Everything outside it consumes json.RawMessage and parses it in
// internal/anakin/sources, because the literal Anakin response field names are
// still unverified. Putting a guessed struct in a frozen contract would be
// worse than a RawMessage: it would make five tracks compile against a shape
// nobody has seen. When B2's live catalogue read resolves the field names, the
// typed shapes land in sources/ and this package does not change.
//
// Three concerns live here and nowhere else: the fetch cache, the credit
// budget, and fixture replay. An adapter cannot spend a credit past the
// ceiling by forgetting a check, because the ceiling is enforced in the one
// place that knows the real per-action cost.
//
// This file is the contract surface: the interface, the options, the errors
// and the config. The implementation is split by concern across http.go,
// cache.go, replay.go, budget.go and nonetwork.go.
package anakin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"brandpulse/internal/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrBudgetExceeded is returned once a client has spent its credit ceiling.
// It is a sentinel: wrap it with %w and compare it with errors.Is, never by
// string. The collector treats it as MentionBatch.Truncated rather than as a
// failure.
var ErrBudgetExceeded = errors.New("anakin: credit budget exceeded")

// Mode selects where a client's data comes from, set by BP_FIXTURE_MODE.
type Mode string

const (
	// ModeReplay never touches the network and reads
	// fixtures/<source>/<query_hash>.json. The CI default.
	ModeReplay Mode = "replay"

	// ModeRecord calls Anakin for real and writes the response to fixtures/.
	// Used once, by B2, on the demo brand.
	ModeRecord Mode = "record"

	// ModeLive calls Anakin, caches to Postgres, and writes no fixtures.
	// Stage demo only.
	ModeLive Mode = "live"
)

// Client is the fetch surface. It is an interface rather than a struct so
// every agent's test can substitute a stub without a live key or a fixture
// directory.
type Client interface {
	// Search runs the Search API. It returns snippets, not page bodies:
	// full text costs a follow-up Scrape per URL.
	Search(ctx context.Context, query string, opt SearchOpt) (json.RawMessage, error)

	// Wire runs one prebuilt per-site action. platform is the catalogue slug
	// such as "reddit"; opt.Action is the action id such as "rt_search".
	Wire(ctx context.Context, platform, query string, opt WireOpt) (json.RawMessage, error)

	// Scrape fetches one URL's content.
	Scrape(ctx context.Context, url string, opt ScrapeOpt) (json.RawMessage, error)

	// Map lists a site's links without fetching their content. Cheap: one
	// credit per job, against one credit per page for Crawl.
	Map(ctx context.Context, url string, opt MapOpt) (json.RawMessage, error)

	// Crawl fetches a site's pages. One credit per page.
	Crawl(ctx context.Context, url string, opt CrawlOpt) (json.RawMessage, error)

	// Stats reports what this client has spent since it was built. The
	// collector reads it once at the end to fill MentionBatch.CreditsUsed and
	// CacheHits, and keeps no cost table of its own: per-action Wire costs
	// vary, and a second table would be a second source of truth for the
	// number the whole budget story rests on.
	Stats() Stats
}

// Stats is one client's spend since construction.
type Stats struct {
	CreditsUsed int `json:"credits_used"`
	CacheHits   int `json:"cache_hits"`
}

// SearchOpt tunes a Search call. Limit defaults to 5 and caps at 20.
//
// Locale and freshness parameters are unverified and therefore absent: recency
// is filtered client-side on the result date until B2 confirms otherwise.
type SearchOpt struct {
	Limit int
}

// WireOpt tunes a Wire call. Action is the action id and is required; the rest
// are that action's own parameters, which differ per site.
type WireOpt struct {
	Action string
	Limit  int
	Sort   string
	Time   string
	After  string

	// Params carries action-specific parameters that have no field here, such
	// as an Amazon asin or a YouTube video_id.
	Params map[string]any
}

// ScrapeOpt tunes a Scrape call. UseBrowser renders JavaScript and costs
// nothing extra, which is what makes the Play Store probe cheap enough to try.
type ScrapeOpt struct {
	Formats    []string
	UseBrowser bool
	Country    string
}

// MapOpt tunes a Map call. Limit defaults to 100 and caps at 5000.
type MapOpt struct {
	Limit             int
	Depth             int
	Search            string
	IncludeSubdomains bool
}

// CrawlOpt tunes a Crawl call. MaxPages is capped at 20 by bp-onboarder
// because crawling costs a credit per page.
type CrawlOpt struct {
	MaxPages        int
	Depth           int
	IncludePatterns []string
	ExcludePatterns []string
	UseBrowser      bool
}

// FetchReason says why a fetch failed, so RunRecord.DegradedReason can name
// the cause instead of saying "error". Anakin distinguishes these and plain
// errors.New would throw the distinction away.
type FetchReason string

const (
	ReasonBlocked     FetchReason = "blocked"
	ReasonCaptcha     FetchReason = "captcha"
	ReasonTimeout     FetchReason = "timeout"
	ReasonDNS         FetchReason = "dns"
	ReasonTLS         FetchReason = "tls"
	ReasonRateLimited FetchReason = "ratelimited"
	ReasonUpstream    FetchReason = "upstream"
)

// FetchError is a failed fetch with its cause preserved.
type FetchError struct {
	Reason FetchReason
	Source models.Source
	Err    error
}

func (e *FetchError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("anakin: %s fetch failed: %s", e.Source, e.Reason)
	}
	return fmt.Sprintf("anakin: %s fetch failed: %s: %v", e.Source, e.Reason, e.Err)
}

func (e *FetchError) Unwrap() error { return e.Err }

// Config builds an HTTPClient. MaxCredits is the ceiling this client may
// spend: the collector constructs a client bound to its own CollectInput
// .MaxCredits and hands it to the adapter, so the adapter signature needs no
// budget argument.
//
// HTTPClient is not decoration. It is how CI enforces no-network, because Go
// has no pytest-socket: tests inject NoNetwork().
type Config struct {
	APIKey string
	Mode   Mode

	// MaxCredits of 0 means "read brands.daily_credit_budget", which needs
	// Pool. ModeReplay has no pool by design, so a replay Config without
	// MaxCredits is rejected by NewHTTPClient rather than at the first call.
	MaxCredits int

	// BrandID and Day scope the persisted credit total, so a ceiling survives
	// a process restart.
	BrandID string
	Day     time.Time

	Pool       *pgxpool.Pool
	HTTPClient *http.Client

	// FixtureDir is the root read in ModeReplay and written in ModeRecord.
	// Empty means "fixtures".
	FixtureDir string
}

// NewHTTPClient and the Client implementation are in http.go.
// Budget, NewBudget, Spend and Remaining are in budget.go.
