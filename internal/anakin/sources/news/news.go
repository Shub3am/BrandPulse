// Package news collects press coverage through the Search API.
//
// It is searchapi with one prompt suffix and one Source. There is no
// server-side source filter to ask for news specifically, verified live on
// 2026-09-20, so the prompt text is the whole mechanism and this package is
// deliberately thin rather than pretending to more control than exists.
//
// It must not know what a sentiment or a topic is. It emits raw mentions.
package news

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"brandpulse/internal/anakin"
	"brandpulse/internal/anakin/sources/mentions"
	"brandpulse/internal/anakin/sources/searchapi"
	"brandpulse/internal/models"
)

// Fetch collects news mentions for p between start and end.
//
// Results are filtered to the window client-side and paid for either way: the
// Search API accepts freshness and date_range parameters with a 200 and
// ignores them.
func Fetch(ctx context.Context, c anakin.Client, p models.BrandProfile, start, end time.Time) ([]models.Mention, error) {
	drafts, stop, problems := searchapi.Collect(ctx, c, models.SourceNews, p.Keywords, prompt, end)

	out, dropped := mentions.Finalize(drafts, p, start, end)
	if len(dropped.Invalid) > 0 {
		problems = append(problems, fmt.Errorf("news: %d mentions failed validation: %s",
			len(dropped.Invalid), strings.Join(dropped.Invalid, "; ")))
	}
	if stop != nil {
		return out, errors.Join(append([]error{stop}, problems...)...)
	}
	return out, errors.Join(problems...)
}

// prompt steers Search towards press coverage. It is a hint, not a filter.
func prompt(keyword string) string { return keyword + " news" }
