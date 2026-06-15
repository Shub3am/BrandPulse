// replay_test.go drives the adapter through the real replay client over the
// fixtures recorded live on 2026-09-20.
//
// The stubbed tests in youtube_test.go all passed while the recording session
// failed to decode a single yt_search response, because every stub handed the
// adapter a bare payload and Anakin sends an envelope. This file reads the
// bytes on disk instead.
package youtube

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"brandpulse/internal/anakin"
	"brandpulse/internal/anakin/sources/wire"
	"brandpulse/internal/models"
)

var (
	replayEnd   = time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	replayStart = replayEnd.AddDate(0, 0, -21)
)

func replayClient(t *testing.T) anakin.Client {
	t.Helper()

	c, err := anakin.NewHTTPClient(anakin.Config{
		Mode:       anakin.ModeReplay,
		MaxCredits: 300,
		BrandID:    "brd_replay",
		Day:        replayEnd,
		HTTPClient: anakin.NoNetwork(),
		FixtureDir: "../../../../fixtures",
	})
	if err != nil {
		t.Fatalf("NewHTTPClient: %v", err)
	}
	return c
}

// The recorded yt_search response is what failed with "cannot unmarshal object
// into Go struct field searchResponse.data of type []youtube.video". The object
// Go refused was the inner envelope, so the fix is the unwrap and not the
// struct: these tags are the payload's own.
func TestSearchResponseDecodesTheRecording(t *testing.T) {
	raw, err := replayClient(t).Wire(context.Background(), "youtube", "Dot & Key", anakin.WireOpt{
		Action: "yt_search",
		Limit:  commentVideosPerQuery,
	})
	if err != nil {
		t.Fatalf("Wire: %v", err)
	}

	payload, err := wire.Payload(raw)
	if err != nil {
		t.Fatalf("wire.Payload: %v", err)
	}

	var search searchResponse
	if err := json.Unmarshal(payload, &search); err != nil {
		t.Fatalf("decoding the recorded yt_search: %v", err)
	}
	if search.Query != "Dot & Key" {
		t.Errorf("Query = %q, want the recorded query", search.Query)
	}
	if len(search.Videos) != 2 {
		t.Fatalf("got %d videos, want the 2 recorded", len(search.Videos))
	}
	if search.Videos[0].VideoID != "v5gsmv3F-6o" {
		t.Errorf("first video_id = %q, want v5gsmv3F-6o", search.Videos[0].VideoID)
	}
	if search.Videos[0].Published == "" || search.Videos[0].URL == "" {
		t.Errorf("video %+v lost fields the payload carries", search.Videos[0])
	}
}

// YouTube's mention unit is the comment, and the recording session never got
// past the search decode, so no yt_comments response was ever written. Fetch
// therefore reports a missing fixture in replay. That is the honest outcome and
// it is asserted here so the gap is visible rather than read as a quiet week:
// the moment the comment calls are recorded, this test tells whoever did it
// that the expectation needs updating.
func TestFetchReachesTheCommentCallItHasNoRecordingFor(t *testing.T) {
	profile := models.BrandProfile{BrandID: "dot-key", Keywords: []string{"Dot & Key"}, Version: 1}

	got, err := Fetch(context.Background(), replayClient(t), profile, replayStart, replayEnd)
	if len(got) != 0 {
		t.Fatalf("got %d mentions, want 0 until a yt_comments response is recorded", len(got))
	}
	if err == nil {
		t.Fatal("want the missing yt_comments fixture reported")
	}
	if !strings.Contains(err.Error(), "yt_comments") || !strings.Contains(err.Error(), "no fixture") {
		t.Fatalf("err = %v, want a missing yt_comments fixture rather than a decode failure", err)
	}
	// The search itself has to have decoded, or the adapter would never have
	// learned a video id to ask for comments on.
	if !strings.Contains(err.Error(), "v5gsmv3F-6o") {
		t.Errorf("err = %v, want the video id the recorded search yielded", err)
	}
}
