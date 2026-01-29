// Package youtube turns Wire's yt_search and yt_comments responses into
// mentions.
//
// The mention unit is the comment, not the video. A yt_search result has no
// absolute timestamp and no body, so it cannot be a Mention; it is only how
// this package discovers the video ids to ask for comments on.
//
// Both schemas were read live, yt_search on 2026-09-20 during Task 1 and
// yt_comments during Task 4. Both are written down in
// docs/research/wire-schemas.md §4. This package must not guess a field name.
//
// It must not know what a sentiment or a topic is. It emits raw mentions.
package youtube

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"brandpulse/internal/anakin"
	"brandpulse/internal/anakin/sources/mentions"
	"brandpulse/internal/models"
)

// commentVideosPerQuery caps how many of a search's videos get a comment call.
//
// yt_search costs 1 credit and yt_comments costs 3, so this constant is most
// of what YouTube costs: one keyword is 1 + 3n. Two is what the recording
// budget in docs/research/anakin.md §6 allows, 3 searches and 6 comment calls
// for 21 credits. Raising it raises the bill linearly.
const commentVideosPerQuery = 2

// commentLimit is comments per yt_comments call. The action's own default is
// 50 and the cost does not vary with it, so there is no reason to ask for less.
const commentLimit = 50

// searchResponse is the yt_search payload, after internal/anakin has stripped
// the Wire job envelope.
type searchResponse struct {
	Query  string  `json:"query"`
	Count  int     `json:"count"`
	Videos []video `json:"data"`
}

// video is one search result. views and published are both unusable as
// timestamps or counts without conversion, which is why nothing here becomes a
// Mention directly.
type video struct {
	VideoID   string `json:"video_id"`
	Title     string `json:"title"`
	Channel   string `json:"channel"`
	ChannelID string `json:"channel_id"`
	Views     string `json:"views"`
	Published string `json:"published"`
	URL       string `json:"url"`
}

// commentsResponse is the yt_comments payload.
type commentsResponse struct {
	VideoID       string    `json:"video_id"`
	Title         string    `json:"title"`
	ChannelID     string    `json:"channel_id"`
	CommentsCount int       `json:"comments_count"`
	Count         int       `json:"count"`
	Comments      []comment `json:"data"`
}

// comment is one YouTube comment. likes is a string and published is relative
// prose; see relativeTime for what that costs.
type comment struct {
	CommentID  string `json:"comment_id"`
	Author     string `json:"author"`
	Text       string `json:"text"`
	Likes      string `json:"likes"`
	Published  string `json:"published"`
	ReplyCount int    `json:"reply_count"`
	Depth      int    `json:"depth"`
}

// Fetch collects YouTube comment mentions for p between start and end.
//
// p.Hashtags are not searched: YouTube search treats "#boAt" as the literal
// token and the yield is a strict subset of the plain keyword's.
func Fetch(ctx context.Context, c anakin.Client, p models.BrandProfile, start, end time.Time) ([]models.Mention, error) {
	var drafts []models.Mention
	var problems []error

	for _, keyword := range p.Keywords {
		raw, err := c.Wire(ctx, "youtube", keyword, anakin.WireOpt{
			Action: "yt_search",
			Limit:  commentVideosPerQuery,
		})
		if err != nil {
			if errors.Is(err, anakin.ErrBudgetExceeded) {
				return finalize(drafts, p, start, end, problems, err)
			}
			problems = append(problems, fmt.Errorf("youtube: yt_search %q: %w", keyword, err))
			continue
		}

		var search searchResponse
		if err := json.Unmarshal(raw, &search); err != nil {
			problems = append(problems, fmt.Errorf("youtube: decoding yt_search %q: %w", keyword, err))
			continue
		}

		videos := search.Videos
		if len(videos) > commentVideosPerQuery {
			videos = videos[:commentVideosPerQuery]
		}
		for _, v := range videos {
			found, stop, errs := commentsFor(ctx, c, v, keyword, end)
			drafts = append(drafts, found...)
			problems = append(problems, errs...)
			if stop != nil {
				return finalize(drafts, p, start, end, problems, stop)
			}
		}
	}

	return finalize(drafts, p, start, end, problems, nil)
}

// commentsFor asks for one video's comments and maps them. stop is non-nil
// only for a budget ceiling, which ends the whole run rather than this video.
func commentsFor(ctx context.Context, c anakin.Client, v video, keyword string, reference time.Time) (drafts []models.Mention, stop error, problems []error) {
	raw, err := c.Wire(ctx, "youtube", v.VideoID, anakin.WireOpt{
		Action: "yt_comments",
		Limit:  commentLimit,
		Sort:   "top",
		Params: map[string]any{"video_id": v.VideoID, "include_replies": true},
	})
	if err != nil {
		if errors.Is(err, anakin.ErrBudgetExceeded) {
			return nil, err, nil
		}
		return nil, nil, []error{fmt.Errorf("youtube: yt_comments %s: %w", v.VideoID, err)}
	}

	var resp commentsResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, nil, []error{fmt.Errorf("youtube: decoding yt_comments %s: %w", v.VideoID, err)}
	}

	for _, cm := range resp.Comments {
		draft, err := toDraft(cm, v, keyword, reference)
		if err != nil {
			problems = append(problems, err)
			continue
		}
		drafts = append(drafts, draft)
	}
	return drafts, nil, problems
}

// finalize applies the shared close and folds the collected problems into one
// error, keeping stop at the head of the chain so errors.Is finds it.
func finalize(drafts []models.Mention, p models.BrandProfile, start, end time.Time, problems []error, stop error) ([]models.Mention, error) {
	out, dropped := mentions.Finalize(drafts, p, start, end)
	if len(dropped.Invalid) > 0 {
		problems = append(problems, fmt.Errorf("youtube: %d mentions failed validation: %s",
			len(dropped.Invalid), strings.Join(dropped.Invalid, "; ")))
	}
	if stop != nil {
		return out, errors.Join(append([]error{stop}, problems...)...)
	}
	return out, errors.Join(problems...)
}

// toDraft maps one comment onto a mention with everything except its identity.
func toDraft(cm comment, v video, keyword string, reference time.Time) (models.Mention, error) {
	postedAt, precision, err := relativeTime(cm.Published, reference)
	if err != nil {
		return models.Mention{}, fmt.Errorf("youtube: comment %s on %s: %w", cm.CommentID, v.VideoID, err)
	}

	return models.Mention{
		Source:     models.SourceYoutube,
		ExternalID: cm.CommentID,
		URL:        fmt.Sprintf("https://www.youtube.com/watch?v=%s&lc=%s", v.VideoID, cm.CommentID),
		Author:     strings.TrimPrefix(cm.Author, "@"),
		// YouTube exposes no subscriber count on a comment, so
		// influencer_mention cannot fire here. 0 means unknown, not zero
		// followers, and inventing a number would fire a real alert rule.
		AuthorFollowers: 0,
		Text:            cm.Text,
		Lang:            "en",
		PostedAt:        postedAt,
		Engagement: models.Engagement{
			Likes:   count(cm.Likes),
			Replies: cm.ReplyCount,
		},
		MatchedKeyword: keyword,
		Raw: map[string]any{
			"video_id":            v.VideoID,
			"video_title":         v.Title,
			"channel":             v.Channel,
			"published_relative":  cm.Published,
			"posted_at_precision": precision,
			"likes_raw":           cm.Likes,
		},
	}, nil
}

// relativeRE matches YouTube's only timestamp format, "10 months ago" with an
// optional " (edited)" suffix.
var relativeRE = regexp.MustCompile(`^(\d+)\s+(second|minute|hour|day|week|month|year)s?\s+ago(\s+\(edited\))?$`)

// relativeTime resolves YouTube's relative prose against reference, returning
// the resolved instant and the precision it was stated at.
//
// Nothing in either YouTube payload carries an absolute timestamp, and
// Mention.PostedAt must be exact, so this conversion is the only way the
// source can produce a valid mention at all. Two things make it safe enough:
//
// The reference is the collection window's end rather than time.Now(), so a
// replayed fixture resolves against the window it is replayed into and the
// same fixture produces the same corpus every run.
//
// The error is worst where it matters least. A comment stated in days or weeks
// lands inside a 14-day window with at most a few days of drift; one stated in
// months or years is outside that window and gets dropped by the filter
// regardless. The stated precision is recorded in Mention.Raw so nothing
// downstream reads a resolved month as an exact date.
//
// An unrecognised format is an error, never a fallback to reference. Dating an
// old comment as today would poison the baseline the detector runs on.
func relativeTime(published string, reference time.Time) (time.Time, string, error) {
	match := relativeRE.FindStringSubmatch(strings.TrimSpace(published))
	if match == nil {
		return time.Time{}, "", fmt.Errorf("unrecognised published format %q", published)
	}

	n, err := strconv.Atoi(match[1])
	if err != nil {
		return time.Time{}, "", fmt.Errorf("unparseable count in %q: %w", published, err)
	}
	unit := match[2]

	switch unit {
	case "second":
		return reference.Add(-time.Duration(n) * time.Second).UTC(), unit, nil
	case "minute":
		return reference.Add(-time.Duration(n) * time.Minute).UTC(), unit, nil
	case "hour":
		return reference.Add(-time.Duration(n) * time.Hour).UTC(), unit, nil
	case "day":
		return reference.AddDate(0, 0, -n).UTC(), unit, nil
	case "week":
		return reference.AddDate(0, 0, -7*n).UTC(), unit, nil
	case "month":
		return reference.AddDate(0, -n, 0).UTC(), unit, nil
	default:
		return reference.AddDate(-n, 0, 0).UTC(), unit, nil
	}
}

// count parses a counter YouTube returns as a string, such as likes "6".
//
// Only a plain integer, with or without thousands commas, is converted.
// Anything else returns 0, which Mention.Engagement already means as
// "unknown", and the original string is kept in Mention.Raw. Guessing at an
// abbreviation this adapter has never seen returned would be inventing a
// number that the influencer and spike rules read.
func count(s string) int {
	n, err := strconv.Atoi(strings.ReplaceAll(strings.TrimSpace(s), ",", ""))
	if err != nil {
		return 0
	}
	return n
}
