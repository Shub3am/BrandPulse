package scrape

import (
	"encoding/json"
	"strings"
	"testing"
)

// liveResponse is the field set POST /v1/url-scraper/scrape returned on
// 2026-09-20, at the top level with no envelope.
const liveResponse = `{"id":"7599a32d-79c6-4cc6-a049-e988bd0c9391",
 "status":"completed",
 "url":"https://itunes.apple.com/in/rss/customerreviews/id=1592550875/sortBy=mostRecent/json",
 "jobType":"url_scraper",
 "country":"in",
 "html":"<html><body>hello</body></html>",
 "cleanedHtml":"<body>hello</body>",
 "markdown":"hello",
 "cached":false,
 "createdAt":"2026-09-20T09:56:18.794Z",
 "completedAt":"2026-09-20T09:56:21.641Z",
 "durationMs":2470}`

func TestDecodeReadsTheLiveFields(t *testing.T) {
	resp, err := Decode(json.RawMessage(liveResponse))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if resp.Status != "completed" {
		t.Errorf("Status = %q", resp.Status)
	}
	if resp.Country != "in" {
		t.Errorf("Country = %q", resp.Country)
	}
	if resp.HTML != "<html><body>hello</body></html>" {
		t.Errorf("HTML = %q", resp.HTML)
	}
	if resp.CleanedHTML != "<body>hello</body>" || resp.Markdown != "hello" {
		t.Errorf("CleanedHTML = %q, Markdown = %q", resp.CleanedHTML, resp.Markdown)
	}
	if resp.Cached {
		t.Error("Cached = true, want false")
	}
	if resp.DurationMS != 2470 {
		t.Errorf("DurationMS = %d", resp.DurationMS)
	}
}

func TestDecodeRefusesAResponseWithNoContent(t *testing.T) {
	// A 200 with every format empty is the failure that looks like success.
	_, err := Decode(json.RawMessage(`{"status":"completed","html":"","cleanedHtml":"","markdown":""}`))
	if err == nil {
		t.Fatal("want an error, got a usable response")
	}
	if !strings.Contains(err.Error(), "no html") {
		t.Errorf("err = %v", err)
	}
}

func TestDecodeAcceptsAResponseCarryingOnlyOneFormat(t *testing.T) {
	// appstore asks for html alone and web asks for markdown alone.
	for _, body := range []string{`{"html":"<p>x</p>"}`, `{"markdown":"x"}`, `{"cleanedHtml":"<p>x</p>"}`} {
		if _, err := Decode(json.RawMessage(body)); err != nil {
			t.Errorf("Decode(%s): %v", body, err)
		}
	}
}

func TestDecodeReportsMalformedJSON(t *testing.T) {
	if _, err := Decode(json.RawMessage(`{"html":`)); err == nil {
		t.Fatal("want a decode error")
	}
}
