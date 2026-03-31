// Package sources is the lookup from a Source to the adapter that collects it.
//
// It holds the registry and nothing else. Every adapter lives in its own
// subdirectory because CONTRACTS §4 fixes the entry point as a function named
// Fetch, and eight files in one Go package cannot each declare Fetch. One
// package per source keeps the contract's signature exactly and makes the
// import path say which source a Fetch belongs to.
//
// bp-collector reads Adapters and nothing under it. Nothing here knows what a
// sentiment or a topic is.
package sources

import (
	"context"
	"time"

	"brandpulse/internal/anakin"
	"brandpulse/internal/anakin/sources/appstore"
	"brandpulse/internal/anakin/sources/news"
	"brandpulse/internal/anakin/sources/playstore"
	"brandpulse/internal/anakin/sources/reddit"
	"brandpulse/internal/anakin/sources/web"
	"brandpulse/internal/anakin/sources/youtube"
	"brandpulse/internal/models"
)

// FetchFunc is the adapter entry point fixed by CONTRACTS §4.
//
// The window is half-open, [start, end). An adapter returns the mentions it
// managed to collect and an error describing everything that went wrong,
// including when it returns both: a source that hit the budget ceiling halfway
// through still hands back the half it paid for.
type FetchFunc func(ctx context.Context, c anakin.Client, p models.BrandProfile, start, end time.Time) ([]models.Mention, error)

// Adapters is every source BrandPulse actually collects.
//
// Six, not seven and not ten. amazon is absent because its Wire action returns
// review metadata with empty review text, and x is absent because it was never
// probed. A source that is not in this map does not appear in a fixture, a
// count or a sentence.
var Adapters = map[models.Source]FetchFunc{
	models.SourceReddit:    reddit.Fetch,
	models.SourceYoutube:   youtube.Fetch,
	models.SourceNews:      news.Fetch,
	models.SourceWeb:       web.Fetch,
	models.SourceAppstore:  appstore.Fetch,
	models.SourcePlaystore: playstore.Fetch,
}

// For returns the adapter for a source, and whether one exists. A caller that
// ranges over a BrandProfile.Sources list needs to tell "not collected" apart
// from "collected nothing", and a nil FetchFunc out of the map does not.
func For(source models.Source) (FetchFunc, bool) {
	adapter, ok := Adapters[source]
	return adapter, ok
}
