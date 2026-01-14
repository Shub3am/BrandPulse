// http.go turns one Client method into one Anakin call: the cache check, the
// credit spend, the live HTTP request with its retry and its async job poll.
//
// It must not know what a mention is. Every method hands back the response body
// as it arrived; turning that into a models.Mention belongs to
// internal/anakin/sources, and stays there until B2's live run supplies the
// real field names.
//
// Endpoints, the auth header and the credit costs here come from
// docs/research/anakin.md. Where that research says UNVERIFIED, the guess is
// isolated in a parse* function so the correction is one function and not a
// rewrite.
//
// The Postgres cache path has no test yet: B1's suite runs in replay, which
// never touches Postgres. B2's first live run is the first time those two
// queries execute.
package anakin

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"brandpulse/internal/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	// baseURL is anakin.io, not anakin.ai, which is an unrelated product.
	baseURL = "https://api.anakin.io/v1"

	// maxAttempts is the brief's retry budget: the first try plus two more on
	// a 429 or a 5xx.
	maxAttempts = 3

	// baseBackoff is the first retry's wait; it doubles per attempt.
	baseBackoff = 500 * time.Millisecond

	// maxPollAttempts bounds an async job. Wire jobs report ~8s execution, so
	// forty polls at two seconds is generous without being unbounded.
	maxPollAttempts = 40

	// defaultPollInterval is used when a poll response carries no
	// retry_after_ms.
	defaultPollInterval = 2 * time.Second

	// defaultHTTPTimeout has to clear the inline scraper's 90s ceiling.
	// http.DefaultClient has no timeout at all, which is why one is set here.
	defaultHTTPTimeout = 120 * time.Second
)

// Per-call credit costs, from docs/research/anakin.md §6. They live here rather
// than in a collector because Stats is the number the whole budget story rests
// on and a second table would be a second source of truth.
const (
	costSearch        = 3
	costMap           = 1
	costScrapeBasic   = 1
	costScrapeSummary = 2
	costScrapeJSON    = 3
	costWireDefault   = 2
)

// httpClient is the one Client implementation. Replay is a mode on it rather
// than a second type, because a fixture and a live call differ only in where
// the bytes come from: the cache, the budget and the hashing are identical, and
// two types would let them drift.
type httpClient struct {
	mode       Mode
	apiKey     string
	budget     *Budget
	pool       *pgxpool.Pool
	http       *http.Client
	fixtureDir string

	mu    sync.Mutex
	stats Stats
	memo  map[string]json.RawMessage
}

// NewHTTPClient returns a Client that checks fetch_cache first, spends against
// the ceiling, and honours cfg.Mode. Once the ceiling is reached every method
// returns ErrBudgetExceeded without calling out.
func NewHTTPClient(cfg Config) (Client, error) {
	switch cfg.Mode {
	case ModeReplay, ModeRecord, ModeLive:
	default:
		return nil, fmt.Errorf("anakin: mode %q is not replay, record or live; BP_FIXTURE_MODE is probably unset", cfg.Mode)
	}
	if cfg.Mode != ModeReplay && cfg.APIKey == "" {
		return nil, fmt.Errorf("anakin: mode %s needs ANAKIN_API_KEY", cfg.Mode)
	}

	// Replay is deliberately cut off from Postgres, so its budget has no
	// persisted total to read and needs the ceiling stated. Failing here
	// rather than on the first call means a track finds out at construction.
	if cfg.Mode == ModeReplay && cfg.MaxCredits <= 0 {
		return nil, errors.New("anakin: replay mode needs Config.MaxCredits; there is no pool to read a ceiling from")
	}

	day := cfg.Day
	if day.IsZero() {
		day = time.Now()
	}
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: defaultHTTPTimeout}
	}
	dir := cfg.FixtureDir
	if dir == "" {
		dir = "fixtures"
	}

	// The budget gets no pool in replay: a replayed run spends no real credits
	// and must not write its pretend spend anywhere near the shared database.
	budgetPool := cfg.Pool
	if cfg.Mode == ModeReplay {
		budgetPool = nil
	}

	return &httpClient{
		mode:       cfg.Mode,
		apiKey:     cfg.APIKey,
		budget:     NewBudget(budgetPool, cfg.BrandID, day, cfg.MaxCredits),
		pool:       cfg.Pool,
		http:       hc,
		fixtureDir: dir,
		memo:       map[string]json.RawMessage{},
	}, nil
}

// Search runs the Search API.
//
// Its cache rows are filed under the web source because the contract's Search
// signature carries no source: a news sweep and a blog sweep issuing the same
// prompt make the same call and deserve the same cache row. The adapter still
// assigns the real Source when it turns a result into a mention.
func (c *httpClient) Search(ctx context.Context, query string, opt SearchOpt) (json.RawMessage, error) {
	if opt.Limit <= 0 {
		opt.Limit = 5
	}
	hash, err := queryHash("search", query, opt)
	if err != nil {
		return nil, err
	}

	return c.fetch(ctx, fetchRequest{
		source:     models.SourceWeb,
		queryHash:  hash,
		timeBucket: hourBucket(time.Now()),
		cost:       costSearch,
		call: func(ctx context.Context) (json.RawMessage, error) {
			return c.do(ctx, models.SourceWeb, http.MethodPost, "/search", map[string]any{
				"prompt": query,
				"limit":  opt.Limit,
			})
		},
	})
}

// Wire runs one prebuilt per-site action through the async job API.
func (c *httpClient) Wire(ctx context.Context, platform, query string, opt WireOpt) (json.RawMessage, error) {
	source := models.Source(platform)
	if !source.Valid() {
		return nil, fmt.Errorf("anakin: wire platform %q is not a models.Source; the cache column is that enum", platform)
	}
	if opt.Action == "" {
		return nil, fmt.Errorf("anakin: wire on %s needs WireOpt.Action, such as rt_search", platform)
	}

	params := wireParams(query, opt)
	hash, err := queryHash("wire:"+opt.Action, platform, params)
	if err != nil {
		return nil, err
	}

	return c.fetch(ctx, fetchRequest{
		source:     source,
		queryHash:  hash,
		timeBucket: hourBucket(time.Now()),
		cost:       wireCost(opt.Action),
		call: func(ctx context.Context) (json.RawMessage, error) {
			accepted, err := c.do(ctx, source, http.MethodPost, "/wire/task", map[string]any{
				"action_id": opt.Action,
				"params":    params,
			})
			if err != nil {
				return nil, err
			}
			jobID, err := parseJobID(accepted)
			if err != nil {
				return nil, err
			}
			return c.pollJob(ctx, source, "/wire/jobs/"+jobID)
		},
	})
}

// Scrape fetches one URL's content through the inline scraper, which blocks
// rather than returning a job.
//
// It buckets by day. The research pins Wire and Search to the hour and Map and
// Crawl to the day and says nothing about the scraper; an article body does not
// change within a day, and the bucket width is the cheapest lever we have on
// credit spend.
func (c *httpClient) Scrape(ctx context.Context, url string, opt ScrapeOpt) (json.RawMessage, error) {
	if len(opt.Formats) == 0 {
		// Plain "text" is unverified; markdown is documented.
		opt.Formats = []string{"markdown"}
	}
	hash, err := queryHash("scrape", url, opt)
	if err != nil {
		return nil, err
	}

	body := map[string]any{
		"url":        url,
		"formats":    opt.Formats,
		"useBrowser": opt.UseBrowser,
	}
	if opt.Country != "" {
		body["country"] = opt.Country
	}

	return c.fetch(ctx, fetchRequest{
		source:     models.SourceWeb,
		queryHash:  hash,
		timeBucket: dayBucket(time.Now()),
		cost:       scrapeCost(opt),
		call: func(ctx context.Context) (json.RawMessage, error) {
			return c.do(ctx, models.SourceWeb, http.MethodPost, "/url-scraper/scrape", body)
		},
	})
}

// Map lists a site's links without fetching them: one credit per job, against
// one credit per page for Crawl.
func (c *httpClient) Map(ctx context.Context, url string, opt MapOpt) (json.RawMessage, error) {
	if opt.Limit <= 0 {
		opt.Limit = 100
	}
	if opt.Depth <= 0 {
		opt.Depth = 2
	}
	hash, err := queryHash("map", url, opt)
	if err != nil {
		return nil, err
	}

	body := map[string]any{
		"url":               url,
		"limit":             opt.Limit,
		"depth":             opt.Depth,
		"includeSubdomains": opt.IncludeSubdomains,
	}
	if opt.Search != "" {
		body["search"] = opt.Search
	}

	return c.fetch(ctx, fetchRequest{
		source:     models.SourceWeb,
		queryHash:  hash,
		timeBucket: dayBucket(time.Now()),
		cost:       costMap,
		call: func(ctx context.Context) (json.RawMessage, error) {
			accepted, err := c.do(ctx, models.SourceWeb, http.MethodPost, "/map", body)
			if err != nil {
				return nil, err
			}
			jobID, err := parseJobID(accepted)
			if err != nil {
				return nil, err
			}
			return c.pollJob(ctx, models.SourceWeb, mapJobPath(jobID))
		},
	})
}

// Crawl fetches a site's pages at one credit each.
func (c *httpClient) Crawl(ctx context.Context, url string, opt CrawlOpt) (json.RawMessage, error) {
	if opt.MaxPages <= 0 {
		opt.MaxPages = 10
	}
	if opt.Depth <= 0 {
		opt.Depth = 1
	}
	hash, err := queryHash("crawl", url, opt)
	if err != nil {
		return nil, err
	}

	body := map[string]any{
		"url":        url,
		"maxPages":   opt.MaxPages,
		"depth":      opt.Depth,
		"useBrowser": opt.UseBrowser,
	}
	if len(opt.IncludePatterns) > 0 {
		body["includePatterns"] = opt.IncludePatterns
	}
	if len(opt.ExcludePatterns) > 0 {
		body["excludePatterns"] = opt.ExcludePatterns
	}

	return c.fetch(ctx, fetchRequest{
		source:     models.SourceWeb,
		queryHash:  hash,
		timeBucket: dayBucket(time.Now()),
		cost:       opt.MaxPages,
		call: func(ctx context.Context) (json.RawMessage, error) {
			accepted, err := c.do(ctx, models.SourceWeb, http.MethodPost, "/crawl", body)
			if err != nil {
				return nil, err
			}
			jobID, err := parseJobID(accepted)
			if err != nil {
				return nil, err
			}
			return c.pollJob(ctx, models.SourceWeb, crawlJobPath(jobID))
		},
	})
}

func (c *httpClient) Stats() Stats {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stats
}

// fetchRequest is one method's call reduced to what every method shares.
type fetchRequest struct {
	source     models.Source
	queryHash  string
	timeBucket string
	cost       int

	// call performs the live HTTP work. It is never invoked in replay.
	call func(context.Context) (json.RawMessage, error)
}

// fetch is the single path every method takes: cache, then budget, then the
// bytes.
//
// The order is the point. A cache hit must not reach the budget, or a cached
// run would still exhaust the ceiling; and the budget must be consulted before
// the fixture read, because replay is how the budget itself is tested.
func (c *httpClient) fetch(ctx context.Context, r fetchRequest) (json.RawMessage, error) {
	key := cacheKey(r.source, r.queryHash, r.timeBucket)

	if payload, ok := c.recall(key); ok {
		c.countCacheHit()
		return payload, nil
	}
	if c.mode != ModeReplay && c.pool != nil {
		payload, ok, err := readFetchCache(ctx, c.pool, r.source, r.queryHash, r.timeBucket)
		if err != nil {
			return nil, err
		}
		if ok {
			c.remember(key, payload)
			c.countCacheHit()
			return payload, nil
		}
	}

	// Anakin does not bill a failed call, and this does: the credits are held
	// before the request and not given back. Refusing one call too many is the
	// right direction to be wrong in when there are 300 credits and no second
	// allocation.
	if err := c.budget.Spend(ctx, r.cost); err != nil {
		return nil, err
	}

	payload, err := c.load(ctx, r)
	if err != nil {
		return nil, err
	}

	if c.mode != ModeReplay && c.pool != nil {
		if err := writeFetchCache(ctx, c.pool, r.source, r.queryHash, r.timeBucket, payload, r.cost); err != nil {
			return nil, err
		}
	}
	c.remember(key, payload)
	c.countCredits(r.cost)
	return payload, nil
}

// load supplies the bytes for this mode: a fixture in replay, a live call
// otherwise, and in record a live call that is then written back as a fixture.
func (c *httpClient) load(ctx context.Context, r fetchRequest) (json.RawMessage, error) {
	if c.mode == ModeReplay {
		return readFixture(c.fixtureDir, r.source, r.queryHash)
	}

	payload, err := r.call(ctx)
	if err != nil {
		return nil, err
	}
	if c.mode == ModeRecord {
		if err := writeFixture(c.fixtureDir, r.source, r.queryHash, payload); err != nil {
			return nil, err
		}
	}
	return payload, nil
}

// do issues one request and retries a 429 or a 5xx twice with an exponential
// backoff. A 4xx that is not a 429 is the caller's fault and is returned at
// once.
func (c *httpClient) do(ctx context.Context, source models.Source, method, path string, payload any) (json.RawMessage, error) {
	var encoded []byte
	if payload != nil {
		var err error
		encoded, err = json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("anakin: encode the %s request: %w", path, err)
		}
	}

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}

		body, status, err := c.roundTrip(ctx, method, path, encoded)
		switch {
		case err != nil:
			lastErr = &FetchError{Reason: classifyTransport(err), Source: source, Err: err}
		case status >= 200 && status < 300:
			return body, nil
		case status == http.StatusTooManyRequests || status >= 500:
			lastErr = &FetchError{
				Reason: classifyStatus(status),
				Source: source,
				Err:    fmt.Errorf("%s %s: %s: %s", method, path, http.StatusText(status), truncate(body)),
			}
		default:
			return nil, &FetchError{
				Reason: classifyStatus(status),
				Source: source,
				Err:    fmt.Errorf("%s %s: %s: %s", method, path, http.StatusText(status), truncate(body)),
			}
		}
	}
	return nil, lastErr
}

func (c *httpClient) roundTrip(ctx context.Context, method, path string, encoded []byte) (json.RawMessage, int, error) {
	var body io.Reader
	if encoded != nil {
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, baseURL+path, body)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Accept", "application/json")
	if encoded != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	read, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return read, resp.StatusCode, nil
}

// pollJob waits for an async job and returns the whole completed poll body.
//
// The whole body, not its data field: Wire documents data, while the crawl and
// map payload field names are unverified, and handing sources/ everything is
// better than handing it a guess.
func (c *httpClient) pollJob(ctx context.Context, source models.Source, path string) (json.RawMessage, error) {
	for range maxPollAttempts {
		body, err := c.do(ctx, source, http.MethodGet, path, nil)
		if err != nil {
			return nil, err
		}

		status, retryAfter, failure := parseJobStatus(body)
		switch status {
		case "completed", "succeeded", "success":
			return body, nil
		case "failed", "error":
			return nil, &FetchError{
				Reason: ReasonUpstream,
				Source: source,
				Err:    fmt.Errorf("anakin job %s failed: %s", path, failure),
			}
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(retryAfter):
		}
	}

	return nil, &FetchError{
		Reason: ReasonTimeout,
		Source: source,
		Err:    fmt.Errorf("anakin job %s was still running after %d polls", path, maxPollAttempts),
	}
}

// parseJobID pulls the job id out of a 202 body.
//
// UNVERIFIED: Wire documents jobId and the map and crawl 202 bodies are not
// documented at all, so id and job_id are accepted too. When B2's live run
// shows the truth this is the one function to correct.
func parseJobID(body json.RawMessage) (string, error) {
	var accepted struct {
		JobID      string `json:"jobId"`
		JobIDSnake string `json:"job_id"`
		ID         string `json:"id"`
	}
	if err := json.Unmarshal(body, &accepted); err != nil {
		return "", fmt.Errorf("anakin: the job acceptance is not JSON: %w", err)
	}
	for _, id := range []string{accepted.JobID, accepted.JobIDSnake, accepted.ID} {
		if id != "" {
			return id, nil
		}
	}
	return "", fmt.Errorf("anakin: no job id in %s", truncate(body))
}

// parseJobStatus reads a poll response.
//
// UNVERIFIED for map and crawl: the Wire envelope (status, retry_after_ms,
// data, error) is assumed to cover all three async APIs. If it does not, the
// fix is here.
func parseJobStatus(body json.RawMessage) (status string, retryAfter time.Duration, failure string) {
	var envelope struct {
		Status       string `json:"status"`
		RetryAfterMs int    `json:"retry_after_ms"`
		Error        struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		// An unparseable poll body is not a terminal state: the next poll may
		// well succeed, and the attempt count bounds the wait either way.
		return "", defaultPollInterval, ""
	}

	retryAfter = time.Duration(envelope.RetryAfterMs) * time.Millisecond
	if retryAfter <= 0 {
		retryAfter = defaultPollInterval
	}
	failure = envelope.Error.Code + " " + envelope.Error.Message
	return envelope.Status, retryAfter, failure
}

// mapJobPath and crawlJobPath are UNVERIFIED. The research records the 202 and
// the poll pattern for both but not the poll path, so they follow the one
// documented example, /wire/jobs/{id}.
func mapJobPath(jobID string) string   { return "/map/" + jobID }
func crawlJobPath(jobID string) string { return "/crawl/" + jobID }

// wireParams assembles one action's parameters. opt.Params carries anything
// with no field of its own, such as an Amazon asin.
func wireParams(query string, opt WireOpt) map[string]any {
	params := map[string]any{}
	for k, v := range opt.Params {
		params[k] = v
	}
	params[wirePrimaryParam(opt.Action)] = query

	if opt.Limit > 0 {
		params["limit"] = opt.Limit
	}
	if opt.Sort != "" {
		params["sort"] = opt.Sort
	}
	if opt.Time != "" {
		params["time"] = opt.Time
	}
	if opt.After != "" {
		params["after"] = opt.After
	}
	return params
}

// wirePrimaryParam names the required first parameter of an action, because
// the Client interface passes one query string and Wire calls it query for
// rt_search, video_id for yt_comments and asin for am_product_reviews. The
// mapping is docs/research/anakin.md §3; an unlisted action gets query, which
// is the common case.
func wirePrimaryParam(action string) string {
	switch action {
	case "rt_subreddit_posts":
		return "subreddit"
	case "rt_post_details":
		return "post_id"
	case "rt_user_profile":
		return "username"
	case "yt_comments", "yt_video":
		return "video_id"
	case "yt_channel":
		return "channel_id"
	case "am_product_details", "am_product_reviews":
		return "asin"
	default:
		return "query"
	}
}

// wireCost prices one action. YouTube is the exception the table exists for:
// yt_comments is three credits where yt_search is one.
func wireCost(action string) int {
	switch action {
	case "yt_search", "yt_video", "yt_channel":
		return 1
	case "yt_comments":
		return 3
	default:
		return costWireDefault
	}
}

// scrapeCost prices a scrape by what it asks the scraper to do: browser
// rendering is free, AI summary and AI JSON extraction are not.
func scrapeCost(opt ScrapeOpt) int {
	cost := costScrapeBasic
	for _, format := range opt.Formats {
		switch format {
		case "summary":
			cost = max(cost, costScrapeSummary)
		case "json":
			cost = max(cost, costScrapeJSON)
		}
	}
	return cost
}

// queryHash is the fetch_cache key and the fixture filename. It has to be
// stable across processes and across runs, so it hashes the canonical JSON of
// the arguments rather than anything with a pointer or a map iteration order
// in it.
func queryHash(method, arg string, opt any) (string, error) {
	canonical, err := json.Marshal(struct {
		Method string `json:"method"`
		Arg    string `json:"arg"`
		Opt    any    `json:"opt"`
	}{method, arg, opt})
	if err != nil {
		return "", fmt.Errorf("anakin: hash the %s query: %w", method, err)
	}

	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), nil
}

func hourBucket(t time.Time) string { return t.UTC().Format("2006-01-02T15") }
func dayBucket(t time.Time) string  { return t.UTC().Format("2006-01-02") }

func backoff(attempt int) time.Duration {
	return baseBackoff * time.Duration(1<<(attempt-2))
}

// classifyTransport names the cause of a failed round trip, so
// RunRecord.DegradedReason can say dns instead of error.
func classifyTransport(err error) FetchReason {
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return ReasonDNS
	}
	var certErr *tls.CertificateVerificationError
	if errors.As(err, &certErr) {
		return ReasonTLS
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return ReasonTimeout
	}
	return ReasonUpstream
}

func classifyStatus(status int) FetchReason {
	switch {
	case status == http.StatusTooManyRequests:
		return ReasonRateLimited
	case status == http.StatusForbidden:
		return ReasonBlocked
	default:
		return ReasonUpstream
	}
}

// truncate keeps an error message readable when the body is a page of HTML.
func truncate(body []byte) string {
	const limit = 300
	if len(body) <= limit {
		return string(body)
	}
	return string(body[:limit]) + "..."
}

func (c *httpClient) countCacheHit() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stats.CacheHits++
}

func (c *httpClient) countCredits(n int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stats.CreditsUsed += n
}
