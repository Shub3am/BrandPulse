package reddit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"brandpulse/internal/anakin"
	"brandpulse/internal/anakin/sources/sourcestest"
	"brandpulse/internal/models"
)

// livePost is one post copied verbatim out of the rt_search response recorded
// on 2026-09-20 and written down in docs/research/wire-schemas.md §3, RSS tail
// and null counters included. A hand-tidied fixture would not prove anything:
// the tail and the nulls are the two things that break a naive adapter.
const livePost = `{
  "id":"1wjvvd1",
  "name":"t3_1wjvvd1",
  "title":"I know they're just things but losing them genuinely broke my heart",
  "subreddit":"delhi",
  "author":"BookishByte",
  "score":null,
  "upvote_ratio":null,
  "num_comments":null,
  "created_utc":"2026-09-18T17:01:44+00:00",
  "url":"https://www.reddit.com/r/delhi/comments/1wjvvd1/i_know_theyre_just_things_but_losing_them/",
  "permalink":"/r/delhi/comments/1wjvvd1/i_know_theyre_just_things_but_losing_them/",
  "selftext":"I lost my black boAt Airdopes Prime 513 ANC while travelling   submitted by   /u/BookishByte   to   r/delhi [link]   [comments]",
  "thumbnail":null,"is_video":null,"over_18":null,"link_flair_text":null
}`

// wireResponse wraps an rt_search payload in the job envelope every Wire
// response arrives in, so a stubbed response has the same shape as a recorded
// one. A test that hands the adapter a bare payload proves nothing: the bare
// payload is what the adapter used to expect, and it is why the recording came
// back with zero mentions.
func wireResponse(payload string) json.RawMessage {
	return json.RawMessage(`{"credits_used":2,"execution_ms":3723,"status":"completed",` +
		`"data":{"status":"ok","error":null,"files":[],"data":` + payload + `,` +
		`"meta":{"action_id":"rt_search","catalog_slug":"reddit","envelope_version":"v1"}}}`)
}

func decodePost(t *testing.T) post {
	t.Helper()
	var p post
	if err := json.Unmarshal([]byte(livePost), &p); err != nil {
		t.Fatalf("decoding the live sample: %v", err)
	}
	return p
}

func TestToDraftMapsTheLiveResponse(t *testing.T) {
	draft, err := toDraft(decodePost(t), "boAt Airdopes")
	if err != nil {
		t.Fatalf("toDraft: %v", err)
	}

	if draft.ExternalID != "1wjvvd1" {
		t.Errorf("ExternalID = %q, want 1wjvvd1", draft.ExternalID)
	}
	if draft.Source != models.SourceReddit {
		t.Errorf("Source = %q, want reddit", draft.Source)
	}
	if draft.Author != "BookishByte" {
		t.Errorf("Author = %q, want BookishByte", draft.Author)
	}
	if draft.MatchedKeyword != "boAt Airdopes" {
		t.Errorf("MatchedKeyword = %q", draft.MatchedKeyword)
	}
	if draft.Lang != "en" {
		t.Errorf("Lang = %q, want en; an empty Lang fails Mention.Validate", draft.Lang)
	}
	want := time.Date(2026, 9, 18, 17, 1, 44, 0, time.UTC)
	if !draft.PostedAt.Equal(want) {
		t.Errorf("PostedAt = %v, want %v", draft.PostedAt, want)
	}
	if draft.PostedAt.Location() != time.UTC {
		t.Errorf("PostedAt is in %v, want UTC", draft.PostedAt.Location())
	}
	if draft.Rating != nil {
		t.Errorf("Rating = %v, want nil; Reddit is not a review source", *draft.Rating)
	}
	if draft.Engagement.Total() != 0 || draft.AuthorFollowers != 0 {
		t.Errorf("engagement=%+v followers=%d, want zeros: this path is RSS-backed and carries no counters",
			draft.Engagement, draft.AuthorFollowers)
	}
	if draft.Raw["subreddit"] != "delhi" {
		t.Errorf("Raw[subreddit] = %v, want delhi", draft.Raw["subreddit"])
	}
}

// The RSS footer is appended to every selftext. Leaving it in means the same
// post found by two keywords hashes to two different rows.
func TestToDraftStripsTheRSSTail(t *testing.T) {
	draft, err := toDraft(decodePost(t), "boAt Airdopes")
	if err != nil {
		t.Fatalf("toDraft: %v", err)
	}

	for _, fragment := range []string{"submitted by", "/u/BookishByte", "[link]", "[comments]"} {
		if strings.Contains(draft.Text, fragment) {
			t.Errorf("Text still contains %q:\n%s", fragment, draft.Text)
		}
	}
	want := "I know they're just things but losing them genuinely broke my heart\n" +
		"I lost my black boAt Airdopes Prime 513 ANC while travelling"
	if draft.Text != want {
		t.Errorf("Text =\n%q\nwant\n%q", draft.Text, want)
	}
}

func TestToDraftTextComposition(t *testing.T) {
	cases := []struct {
		name  string
		title string
		body  string
		want  string
	}{
		{"title and body are joined", "Airdopes died", "after two weeks", "Airdopes died\nafter two weeks"},
		{"a body of only the RSS tail leaves the title alone", "Airdopes died",
			"  submitted by   /u/x   to   r/audio [link]   [comments]", "Airdopes died"},
		{"an empty body leaves the title alone", "Airdopes died", "", "Airdopes died"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := text(post{Title: tc.title, Selftext: tc.body}); got != tc.want {
				t.Errorf("text() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestToDraftRejectsAnUnparseableDate(t *testing.T) {
	// created_utc is named like a Unix epoch and is not one. An adapter that
	// treated it as an integer would produce a zero PostedAt, which fails
	// Validate at the insert rather than here.
	p := decodePost(t)
	p.CreatedUTC = "1758214904"

	if _, err := toDraft(p, "boAt"); err == nil {
		t.Fatal("toDraft accepted an epoch-shaped created_utc, want an error")
	}
}

func TestToDraftFallsBackToPermalink(t *testing.T) {
	p := decodePost(t)
	p.URL = ""

	draft, err := toDraft(p, "boAt")
	if err != nil {
		t.Fatalf("toDraft: %v", err)
	}
	want := "https://www.reddit.com/r/delhi/comments/1wjvvd1/i_know_theyre_just_things_but_losing_them/"
	if draft.URL != want {
		t.Errorf("URL = %q, want %q", draft.URL, want)
	}
}

func TestTimeBucketCoversTheWindow(t *testing.T) {
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		span time.Duration
		want string
	}{
		{30 * time.Minute, "hour"},
		{time.Hour, "hour"},
		{6 * time.Hour, "day"},
		{24 * time.Hour, "day"},
		{7 * 24 * time.Hour, "week"},
		{14 * 24 * time.Hour, "month"},
		{31 * 24 * time.Hour, "month"},
		{90 * 24 * time.Hour, "year"},
		{500 * 24 * time.Hour, "all"},
	}

	for _, tc := range cases {
		if got := timeBucket(start, start.Add(tc.span)); got != tc.want {
			t.Errorf("timeBucket(%v) = %q, want %q", tc.span, got, tc.want)
		}
	}
}

// profile is the shape the adapter is driven with. The window deliberately
// excludes the live sample's date so Fetch can be exercised end to end without
// reaching mentions.Stamp, which calls B1's still-panicking stubs.
func profile(keywords ...string) models.BrandProfile {
	return models.BrandProfile{BrandID: "boat-lifestyle", Keywords: keywords, Version: 1}
}

var (
	emptyStart = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	emptyEnd   = time.Date(2020, 1, 15, 0, 0, 0, 0, time.UTC)
)

func TestFetchIssuesOneSearchPerKeyword(t *testing.T) {
	client := &sourcestest.Client{
		WireFunc: func(ctx context.Context, platform, query string, opt anakin.WireOpt) (json.RawMessage, error) {
			return wireResponse(`{"posts":[` + livePost + `],"post_count":1}`), nil
		},
	}

	if _, err := Fetch(context.Background(), client, profile("boAt Airdopes", "boAt Rockerz", "boAt Stone"), emptyStart, emptyEnd); err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	got := client.Queries()
	want := []string{"boAt Airdopes", "boAt Rockerz", "boAt Stone"}
	if len(got) != len(want) {
		t.Fatalf("issued %d searches (%v), want %d: match=true is a phrase filter, so terms cannot be combined",
			len(got), got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("search %d = %q, want %q", i, got[i], want[i])
		}
	}
	for _, call := range client.Calls {
		if call.Platform != "reddit" || call.WireOpt.Action != "rt_search" {
			t.Errorf("called %s/%s, want reddit/rt_search", call.Platform, call.WireOpt.Action)
		}
		if call.WireOpt.Params["match"] != true {
			t.Errorf("match = %v, want true: sort=new ignores the query server-side without it", call.WireOpt.Params["match"])
		}
	}
}

// post_count 0 with a non-zero drop count means the query was too specific.
// Reporting it as "no mentions" would read as a quiet week.
func TestFetchReportsTheSilentZero(t *testing.T) {
	client := &sourcestest.Client{
		WireFunc: func(ctx context.Context, platform, query string, opt anakin.WireOpt) (json.RawMessage, error) {
			return wireResponse(`{"posts":[],"post_count":0,"posts_dropped_by_filter":22,"error":null}`), nil
		},
	}

	got, err := Fetch(context.Background(), client, profile("boAt vs Noise vs boult earbuds"), emptyStart, emptyEnd)
	if len(got) != 0 {
		t.Fatalf("got %d mentions, want 0", len(got))
	}
	if err == nil {
		t.Fatal("Fetch reported no error for a filter-dropped result, want the drop count surfaced")
	}
	if !strings.Contains(err.Error(), "22") || !strings.Contains(err.Error(), "too specific") {
		t.Errorf("error = %q, want it to name the 22 dropped posts and blame the query", err)
	}
}

// A genuinely quiet week has a zero drop count and must not be reported as a
// problem.
func TestFetchIsSilentOnAGenuinelyEmptyResult(t *testing.T) {
	client := &sourcestest.Client{
		WireFunc: func(ctx context.Context, platform, query string, opt anakin.WireOpt) (json.RawMessage, error) {
			return wireResponse(`{"posts":[],"post_count":0,"posts_dropped_by_filter":0,"error":null}`), nil
		},
	}

	got, err := Fetch(context.Background(), client, profile("boAt Nirvana"), emptyStart, emptyEnd)
	if err != nil {
		t.Fatalf("Fetch: %v, want no error when Reddit simply had nothing", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d mentions, want 0", len(got))
	}
}

func TestFetchStopsAtTheBudgetCeiling(t *testing.T) {
	client := &sourcestest.Client{
		WireFunc: func(ctx context.Context, platform, query string, opt anakin.WireOpt) (json.RawMessage, error) {
			if query == "boAt Airdopes" {
				return wireResponse(`{"posts":[],"post_count":0}`), nil
			}
			return nil, fmt.Errorf("reddit: %w", anakin.ErrBudgetExceeded)
		},
	}

	_, err := Fetch(context.Background(), client, profile("boAt Airdopes", "boAt Rockerz", "boAt Stone"), emptyStart, emptyEnd)
	if !errors.Is(err, anakin.ErrBudgetExceeded) {
		t.Fatalf("err = %v, want it to wrap ErrBudgetExceeded so the collector can set Truncated", err)
	}
	if len(client.Calls) != 2 {
		t.Errorf("issued %d searches, want 2: every later query hits the same ceiling", len(client.Calls))
	}
}

// One failing query must not cost the others. The collector turns the error
// into a MentionBatch.Errors entry and keeps the partial batch.
func TestFetchContinuesPastOneFailedQuery(t *testing.T) {
	client := &sourcestest.Client{
		WireFunc: func(ctx context.Context, platform, query string, opt anakin.WireOpt) (json.RawMessage, error) {
			if query == "boAt Rockerz" {
				return nil, errors.New("upstream 502")
			}
			return wireResponse(`{"posts":[],"post_count":0}`), nil
		},
	}

	_, err := Fetch(context.Background(), client, profile("boAt Airdopes", "boAt Rockerz", "boAt Stone"), emptyStart, emptyEnd)
	if err == nil || !strings.Contains(err.Error(), "502") {
		t.Fatalf("err = %v, want the 502 surfaced", err)
	}
	if len(client.Calls) != 3 {
		t.Errorf("issued %d searches, want all 3 attempted", len(client.Calls))
	}
}

func TestFetchSurfacesADecodeFailure(t *testing.T) {
	client := &sourcestest.Client{
		WireFunc: func(ctx context.Context, platform, query string, opt anakin.WireOpt) (json.RawMessage, error) {
			return wireResponse(`{"posts":"not an array"}`), nil
		},
	}

	if _, err := Fetch(context.Background(), client, profile("boAt Airdopes"), emptyStart, emptyEnd); err == nil {
		t.Fatal("Fetch swallowed a malformed response, want an error")
	}
}

func TestFetchDropsPostsOutsideTheWindow(t *testing.T) {
	client := &sourcestest.Client{
		WireFunc: func(ctx context.Context, platform, query string, opt anakin.WireOpt) (json.RawMessage, error) {
			return wireResponse(`{"posts":[` + livePost + `],"post_count":1}`), nil
		},
	}

	// The sample is dated 2026-09-18 and rt_search's time parameter is coarse,
	// so the adapter always receives posts it has to trim itself.
	got, err := Fetch(context.Background(), client, profile("boAt Airdopes"), emptyStart, emptyEnd)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d mentions, want 0: the post is outside [%v, %v)", len(got), emptyStart, emptyEnd)
	}
}
