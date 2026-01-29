package youtube

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"brandpulse/internal/anakin"
	"brandpulse/internal/anakin/sources/sourcestest"
	"brandpulse/internal/models"
)

// Both samples are verbatim from the live responses recorded on 2026-09-20,
// written down in docs/research/wire-schemas.md §4. The string-typed likes and
// the relative published are the two fields that break a naive adapter, so
// they are not tidied here.
const (
	liveSearch = `{"query":"boAt Airdopes review","count":10,"data":[
      {"video_id":"zLXOfWalsMk",
       "title":"Are open earbuds here to stay? boAt Airdopes Loop Review",
       "channel":"Unboxed by Croma",
       "channel_id":"UC2ED_m4SuzuBJMiaJq2Vvfg",
       "views":"32,743 views","published":"1 year ago","duration":"2:31",
       "url":"https://www.youtube.com/watch?v=zLXOfWalsMk"}]}`

	liveComment = `{
      "comment_id":"Ugy1taqzks8jSFGM4gN4AaABAg",
      "author":"@ANBARASANNanbumechanical",
      "author_channel_id":"UCaaAvRM1lxA-R1LU9hH-drg",
      "text":"Im using it , Need to set 100 % volume , u can't experience great music",
      "likes":"6",
      "published":"10 months ago (edited)",
      "is_pinned":false,"is_owner_reply":false,
      "reply_count":1,"parent_id":null,"depth":0}`
)

var reference = time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)

func decodeComment(t *testing.T) comment {
	t.Helper()
	var cm comment
	if err := json.Unmarshal([]byte(liveComment), &cm); err != nil {
		t.Fatalf("decoding the live sample: %v", err)
	}
	return cm
}

func sampleVideo() video {
	return video{
		VideoID: "zLXOfWalsMk",
		Title:   "Are open earbuds here to stay? boAt Airdopes Loop Review",
		Channel: "Unboxed by Croma",
	}
}

func TestToDraftMapsTheLiveComment(t *testing.T) {
	draft, err := toDraft(decodeComment(t), sampleVideo(), "boAt Airdopes review", reference)
	if err != nil {
		t.Fatalf("toDraft: %v", err)
	}

	if draft.ExternalID != "Ugy1taqzks8jSFGM4gN4AaABAg" {
		t.Errorf("ExternalID = %q", draft.ExternalID)
	}
	if draft.Source != models.SourceYoutube {
		t.Errorf("Source = %q, want youtube", draft.Source)
	}
	if draft.Author != "ANBARASANNanbumechanical" {
		t.Errorf("Author = %q, want the @ stripped", draft.Author)
	}
	if draft.Lang != "en" {
		t.Errorf("Lang = %q, want en", draft.Lang)
	}
	if draft.Engagement.Likes != 6 {
		t.Errorf("Likes = %d, want 6 parsed out of the string \"6\"", draft.Engagement.Likes)
	}
	if draft.Engagement.Replies != 1 {
		t.Errorf("Replies = %d, want 1", draft.Engagement.Replies)
	}
	if draft.AuthorFollowers != 0 {
		t.Errorf("AuthorFollowers = %d, want 0: a comment carries no subscriber count", draft.AuthorFollowers)
	}
	if draft.Rating != nil {
		t.Errorf("Rating = %v, want nil", *draft.Rating)
	}
	wantURL := "https://www.youtube.com/watch?v=zLXOfWalsMk&lc=Ugy1taqzks8jSFGM4gN4AaABAg"
	if draft.URL != wantURL {
		t.Errorf("URL = %q, want %q", draft.URL, wantURL)
	}
	if draft.Raw["published_relative"] != "10 months ago (edited)" {
		t.Errorf("Raw[published_relative] = %v, want the original prose kept", draft.Raw["published_relative"])
	}
	if draft.Raw["posted_at_precision"] != "month" {
		t.Errorf("Raw[posted_at_precision] = %v, want month", draft.Raw["posted_at_precision"])
	}
}

func TestRelativeTimeResolvesEveryUnit(t *testing.T) {
	cases := []struct {
		published string
		want      time.Time
		precision string
	}{
		{"30 seconds ago", reference.Add(-30 * time.Second), "second"},
		{"1 minute ago", reference.Add(-time.Minute), "minute"},
		{"5 hours ago", reference.Add(-5 * time.Hour), "hour"},
		{"3 days ago", reference.AddDate(0, 0, -3), "day"},
		{"2 weeks ago", reference.AddDate(0, 0, -14), "week"},
		{"10 months ago", reference.AddDate(0, -10, 0), "month"},
		{"10 months ago (edited)", reference.AddDate(0, -10, 0), "month"},
		{"1 year ago", reference.AddDate(-1, 0, 0), "year"},
		{"  3 days ago  ", reference.AddDate(0, 0, -3), "day"},
	}

	for _, tc := range cases {
		t.Run(tc.published, func(t *testing.T) {
			got, precision, err := relativeTime(tc.published, reference)
			if err != nil {
				t.Fatalf("relativeTime: %v", err)
			}
			if !got.Equal(tc.want) {
				t.Errorf("= %v, want %v", got, tc.want)
			}
			if precision != tc.precision {
				t.Errorf("precision = %q, want %q", precision, tc.precision)
			}
			if got.Location() != time.UTC {
				t.Errorf("resolved to %v, want UTC", got.Location())
			}
		})
	}
}

// An unrecognised format must fail rather than resolve to the reference.
// Dating a year-old comment as today would poison the 14-day baseline that
// every detector rule is built on.
func TestRelativeTimeRefusesToGuess(t *testing.T) {
	for _, published := range []string{"", "just now", "yesterday", "2026-09-20", "a month ago", "10 fortnights ago"} {
		t.Run(published, func(t *testing.T) {
			got, _, err := relativeTime(published, reference)
			if err == nil {
				t.Fatalf("relativeTime(%q) = %v, want an error rather than a guess", published, got)
			}
			if !got.IsZero() {
				t.Errorf("returned %v alongside the error, want the zero time", got)
			}
		})
	}
}

func TestCountParsesOnlyWhatItHasSeen(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"6", 6},
		{"0", 0},
		{"32,743", 32743},
		{"  12  ", 12},
		{"", 0},
		{"1.2K", 0},
		{"one", 0},
	}

	for _, tc := range cases {
		if got := count(tc.in); got != tc.want {
			t.Errorf("count(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func profile(keywords ...string) models.BrandProfile {
	return models.BrandProfile{BrandID: "boat-lifestyle", Keywords: keywords, Version: 1}
}

// The window is old enough that every resolved comment falls outside it, so
// Fetch runs end to end without reaching mentions.Stamp, which calls B1's
// still-panicking stubs.
var (
	emptyStart = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	emptyEnd   = time.Date(2020, 1, 15, 0, 0, 0, 0, time.UTC)
)

// wireStub answers yt_search and yt_comments from the live samples.
func wireStub() *sourcestest.Client {
	return &sourcestest.Client{
		WireFunc: func(ctx context.Context, platform, query string, opt anakin.WireOpt) (json.RawMessage, error) {
			if opt.Action == "yt_search" {
				return json.RawMessage(liveSearch), nil
			}
			return json.RawMessage(`{"video_id":"zLXOfWalsMk","count":1,"comments_count":20,"data":[` + liveComment + `]}`), nil
		},
	}
}

func TestFetchChainsSearchIntoComments(t *testing.T) {
	client := wireStub()

	if _, err := Fetch(context.Background(), client, profile("boAt Airdopes review"), emptyStart, emptyEnd); err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	if len(client.Calls) != 2 {
		t.Fatalf("made %d calls, want 2: one yt_search then one yt_comments", len(client.Calls))
	}
	if client.Calls[0].WireOpt.Action != "yt_search" {
		t.Errorf("first call is %q, want yt_search", client.Calls[0].WireOpt.Action)
	}
	second := client.Calls[1]
	if second.WireOpt.Action != "yt_comments" {
		t.Errorf("second call is %q, want yt_comments", second.WireOpt.Action)
	}
	// yt_search returns video_id directly, so nothing parses it out of a URL.
	if second.WireOpt.Params["video_id"] != "zLXOfWalsMk" {
		t.Errorf("video_id = %v, want zLXOfWalsMk taken straight off the search result", second.WireOpt.Params["video_id"])
	}
}

// yt_comments costs 3 credits a call, so the number of videos a search leads
// to is most of what YouTube costs.
func TestFetchCapsCommentCallsPerQuery(t *testing.T) {
	manyVideos := `{"query":"boAt","count":5,"data":[
      {"video_id":"a","title":"a"},{"video_id":"b","title":"b"},
      {"video_id":"c","title":"c"},{"video_id":"d","title":"d"},
      {"video_id":"e","title":"e"}]}`

	client := &sourcestest.Client{
		WireFunc: func(ctx context.Context, platform, query string, opt anakin.WireOpt) (json.RawMessage, error) {
			if opt.Action == "yt_search" {
				return json.RawMessage(manyVideos), nil
			}
			return json.RawMessage(`{"video_id":"a","count":0,"data":[]}`), nil
		},
	}

	if _, err := Fetch(context.Background(), client, profile("boAt Airdopes"), emptyStart, emptyEnd); err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	comments := 0
	for _, call := range client.Calls {
		if call.WireOpt.Action == "yt_comments" {
			comments++
		}
	}
	if comments != commentVideosPerQuery {
		t.Errorf("made %d yt_comments calls for 5 videos, want %d at 3 credits each", comments, commentVideosPerQuery)
	}
}

func TestFetchStopsAtTheBudgetCeiling(t *testing.T) {
	client := &sourcestest.Client{
		WireFunc: func(ctx context.Context, platform, query string, opt anakin.WireOpt) (json.RawMessage, error) {
			if opt.Action == "yt_search" {
				return json.RawMessage(liveSearch), nil
			}
			return nil, anakin.ErrBudgetExceeded
		},
	}

	_, err := Fetch(context.Background(), client, profile("boAt Airdopes", "boAt Rockerz"), emptyStart, emptyEnd)
	if !errors.Is(err, anakin.ErrBudgetExceeded) {
		t.Fatalf("err = %v, want it to wrap ErrBudgetExceeded", err)
	}
	// One search, one refused comment call, then stop. The second keyword is
	// never searched.
	if len(client.Calls) != 2 {
		t.Errorf("made %d calls, want 2 before the ceiling stopped the run", len(client.Calls))
	}
}

func TestFetchSurfacesAnUnparseableTimestamp(t *testing.T) {
	client := &sourcestest.Client{
		WireFunc: func(ctx context.Context, platform, query string, opt anakin.WireOpt) (json.RawMessage, error) {
			if opt.Action == "yt_search" {
				return json.RawMessage(liveSearch), nil
			}
			return json.RawMessage(`{"video_id":"zLXOfWalsMk","count":1,"data":[
              {"comment_id":"x","author":"@y","text":"hi","likes":"0","published":"yesterday","reply_count":0}]}`), nil
		},
	}

	got, err := Fetch(context.Background(), client, profile("boAt Airdopes"), emptyStart, emptyEnd)
	if len(got) != 0 {
		t.Fatalf("got %d mentions, want the undateable comment dropped", len(got))
	}
	if err == nil || !strings.Contains(err.Error(), "yesterday") {
		t.Fatalf("err = %v, want the unrecognised format named", err)
	}
}

func TestFetchSurfacesADecodeFailure(t *testing.T) {
	client := &sourcestest.Client{
		WireFunc: func(ctx context.Context, platform, query string, opt anakin.WireOpt) (json.RawMessage, error) {
			return json.RawMessage(`{"data":"not an array"}`), nil
		},
	}

	if _, err := Fetch(context.Background(), client, profile("boAt Airdopes"), emptyStart, emptyEnd); err == nil {
		t.Fatal("Fetch swallowed a malformed response, want an error")
	}
}
