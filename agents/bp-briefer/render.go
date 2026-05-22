// Every number in a brief is formatted here, by Go, from the values the
// orchestrator measured.
//
// The model writes the headline, the narrative and the suggested actions, and
// is told in the prompt never to write a number, because a model that restates
// a percentage will eventually restate it wrong and it will be wrong on a
// screen in front of the founder.
//
// Both renderers are pure functions, so their tests need no LLM stub. This file
// must not read a database and must not call a model.

package main

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"brandpulse/internal/models"
)

const (
	// whatsappTopics and whatsappActions cap what reaches a phone. The limit is
	// 600 bytes and a founder reading on the move wants the top of the list,
	// not a truncated tail of it.
	whatsappTopics  = 3
	whatsappActions = 2

	// whatsappAlerts is 1: the most severe alert. The rest are in the markdown.
	whatsappAlerts = 1

	// markdownTopics caps the full brief's topic list. Past this it stops being
	// a brief.
	markdownTopics = 8
)

// ---------------------------------------------------------------------------
// WhatsApp
// ---------------------------------------------------------------------------

// renderWhatsapp builds the short brief: no markdown tables, no links, and at
// most models.WhatsappShortLimit bytes.
//
// It is a pure function over models.DailyBrief so its length and formatting
// tests need no model. DailyBrief.Validate() rejects an over-long value, but
// relying on that alone means finding out on stage, so the limit is enforced
// here, at a sentence boundary, and what survives reads as sentences rather
// than as a cut-off word.
func renderWhatsapp(brief models.DailyBrief) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Brief for %s\n", formatDateRange(brief.PeriodStart, brief.PeriodEnd))
	if brief.Headline != "" {
		fmt.Fprintf(&b, "\n%s\n", plain(brief.Headline))
	}
	fmt.Fprintf(&b, "\n%s\n", formatNumbersLine(brief.Numbers))

	for _, alert := range mostSevere(brief.Alerts, whatsappAlerts) {
		fmt.Fprintf(&b, "\nAlert, %s: %s\n", alert.Severity, plain(alert.Why))
	}

	if topics := firstN(brief.TopTopics, whatsappTopics); len(topics) > 0 {
		labels := make([]string, 0, len(topics))
		for _, topic := range topics {
			labels = append(labels, fmt.Sprintf("%s (%d)", plain(topic.Label), topic.Size))
		}
		fmt.Fprintf(&b, "\nTalked about: %s.\n", strings.Join(labels, ", "))
	}

	if len(brief.SuggestedActions) > 0 {
		b.WriteString("\nThis week:\n")
		for i, action := range brief.SuggestedActions {
			if i >= whatsappActions {
				break
			}
			fmt.Fprintf(&b, "%d. %s\n", i+1, plain(action))
		}
	}

	return truncateAtSentence(strings.TrimSpace(b.String()), models.WhatsappShortLimit)
}

var (
	markdownLink       = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	collapseWhitespace = regexp.MustCompile(`\s+`)
)

// plain strips what WhatsApp renders badly. Topic labels and suggested actions
// are model-written, so either can arrive carrying a markdown link or a pipe.
// A link becomes its own text, a pipe becomes a slash, and nothing is silently
// deleted.
func plain(text string) string {
	text = markdownLink.ReplaceAllString(text, "$1")
	text = strings.ReplaceAll(text, "|", "/")
	text = strings.ReplaceAll(text, "*", "")
	text = strings.ReplaceAll(text, "`", "")
	return strings.TrimSpace(collapseWhitespace.ReplaceAllString(text, " "))
}

// truncateAtSentence cuts text to at most limit bytes, preferring the last
// sentence end and falling back to the last word.
//
// Bytes, not runes, because models.DailyBrief.Validate measures the field with
// len(). A Hinglish brief is multi-byte, so a cut that lands mid-rune is walked
// back before it is returned.
func truncateAtSentence(text string, limit int) string {
	if len(text) <= limit {
		return text
	}

	cut := text[:limit]
	if at := strings.LastIndexAny(cut, ".!?\n"); at > limit/2 {
		return strings.TrimSpace(cut[:at+1])
	}
	if at := strings.LastIndex(cut, " "); at > 0 {
		cut = cut[:at]
	}
	for len(cut) > 0 && !utf8.ValidString(cut) {
		cut = cut[:len(cut)-1]
	}
	return strings.TrimSpace(cut)
}

// ---------------------------------------------------------------------------
// Markdown
// ---------------------------------------------------------------------------

// renderMarkdown builds the full brief around the model's narrative. The
// narrative is the only part of this string a model wrote; period and brandName
// come from BriefInput because DailyBrief carries neither.
func renderMarkdown(brief models.DailyBrief, period, brandName, narrative string) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# %s: %s brief\n\n", brandName, period)
	fmt.Fprintf(&b, "%s\n\n", formatDateRange(brief.PeriodStart, brief.PeriodEnd))
	if brief.Headline != "" {
		fmt.Fprintf(&b, "**%s**\n\n", brief.Headline)
	}

	b.WriteString("## Numbers\n\n| Metric | Value |\n|---|---|\n")
	fmt.Fprintf(&b, "| Mentions | %s (%s) |\n", commas(brief.Numbers.Mentions), formatDelta(brief.Numbers.MentionsDeltaPct))
	fmt.Fprintf(&b, "| Average sentiment | %s |\n", formatSentiment(brief.Numbers.SentimentAvg))
	fmt.Fprintf(&b, "| Negative share | %s |\n", formatPercent(brief.Numbers.NegativeShare))
	fmt.Fprintf(&b, "| Share of voice | %s |\n\n", formatPercent(brief.Numbers.ShareOfVoice))

	if narrative != "" {
		fmt.Fprintf(&b, "%s\n\n", strings.TrimSpace(narrative))
	}

	if topics := firstN(brief.TopTopics, markdownTopics); len(topics) > 0 {
		b.WriteString("## What people talked about\n\n")
		for i, topic := range topics {
			fmt.Fprintf(&b, "%d. **%s**, %d mentions, %s. %s\n",
				i+1, topic.Label, topic.Size, formatTrend(topic.Trend), topic.Summary)
		}
		b.WriteString("\n")
	}

	if len(brief.Alerts) > 0 {
		b.WriteString("## Alerts\n\n")
		for _, alert := range mostSevere(brief.Alerts, len(brief.Alerts)) {
			fmt.Fprintf(&b, "- **%s** (%s): %s\n", alert.Kind, alert.Severity, alert.Why)
			for _, e := range alert.Evidence {
				fmt.Fprintf(&b, "  - %s: %s, fires at %s, over %s\n",
					e.Metric, trimFloat(e.Value), trimFloat(e.Threshold), e.Window)
			}
		}
		b.WriteString("\n")
	}

	if len(brief.CompetitorWatch) > 0 {
		fmt.Fprintf(&b, "## Competitor watch\n\n%s\n\n", strings.Join(brief.CompetitorWatch, ", "))
	}

	if len(brief.SuggestedActions) > 0 {
		b.WriteString("## Suggested actions\n\n")
		for _, action := range brief.SuggestedActions {
			fmt.Fprintf(&b, "- %s\n", action)
		}
	}

	return strings.TrimSpace(b.String()) + "\n"
}

// ---------------------------------------------------------------------------
// Number formatting. One place, so the markdown and the WhatsApp brief can
// never disagree about the same figure.
// ---------------------------------------------------------------------------

// formatNumbersLine is the whole of BriefNumbers as two sentences, which is
// what fits on a phone.
func formatNumbersLine(n models.BriefNumbers) string {
	return fmt.Sprintf("%s mentions, %s. %s negative, average sentiment %s, share of voice %s.",
		commas(n.Mentions),
		formatDelta(n.MentionsDeltaPct),
		formatPercent(n.NegativeShare),
		formatSentiment(n.SentimentAvg),
		formatPercent(n.ShareOfVoice),
	)
}

// formatDelta reads a period-over-period change. Exactly zero is "flat" rather
// than "up 0%", because a delta of exactly zero is almost always the first
// period rather than a genuinely unchanged one.
func formatDelta(pct float64) string {
	switch {
	case pct > 0:
		return "up " + formatPercent(pct)
	case pct < 0:
		return "down " + formatPercent(-pct)
	default:
		return "flat"
	}
}

// formatPercent takes a percentage, not a fraction: BriefNumbers.NegativeShare
// and ShareOfVoice are already on a 0 to 100 scale.
func formatPercent(pct float64) string {
	return strconv.FormatFloat(roundTo(pct, 1), 'f', -1, 64) + "%"
}

// formatSentiment keeps the sign and both decimals, which is the whole meaning
// of a figure that lives between -1 and 1.
func formatSentiment(avg float64) string {
	return strconv.FormatFloat(roundTo(avg, 2), 'f', 2, 64)
}

// formatTrend reads Topic.Trend, which is this window's size over the prior
// one. 1.0 means flat and 1.0 is also what a first window carries, so both read
// as "flat" on purpose rather than inventing a rise out of a first run. The 5%
// band keeps rounding noise from being reported as movement.
func formatTrend(trend float64) string {
	switch {
	case trend > 1.05:
		return "up " + formatPercent((trend-1)*100)
	case trend < 0.95:
		return "down " + formatPercent((1-trend)*100)
	default:
		return "flat"
	}
}

// commas groups an integer in thousands.
func commas(n int) string {
	digits := strconv.Itoa(n)
	sign := ""
	if strings.HasPrefix(digits, "-") {
		sign, digits = "-", digits[1:]
	}

	var out strings.Builder
	for i, r := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			out.WriteByte(',')
		}
		out.WriteRune(r)
	}
	return sign + out.String()
}

// trimFloat prints an evidence number without a trailing ".0", so a z-score of
// 4 reads as 4 and one of 4.2 reads as 4.2.
func trimFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// roundTo is half-up on the absolute value, so -0.125 and 0.125 round to the
// same magnitude. math.Round is banker's-neutral but this keeps the sign
// handling in one visible place.
func roundTo(v float64, places int) float64 {
	scale := 1.0
	for i := 0; i < places; i++ {
		scale *= 10
	}
	if v < 0 {
		return -float64(int(-v*scale+0.5)) / scale
	}
	return float64(int(v*scale+0.5)) / scale
}

// ---------------------------------------------------------------------------
// Small shared helpers
// ---------------------------------------------------------------------------

func formatDateRange(start, end time.Time) string {
	return fmt.Sprintf("%s to %s",
		start.UTC().Format("2 Jan"),
		end.UTC().Format("2 Jan 2006"),
	)
}

func firstN(topics []models.Topic, n int) []models.Topic {
	if len(topics) <= n {
		return topics
	}
	return topics[:n]
}

// mostSevere returns at most n alerts, worst first, so a phone-sized brief
// leads with the crisis rather than with whichever rule happened to run first.
// The sort is stable, so equal severities keep the detector's deterministic
// order and the same input renders the same brief every time.
func mostSevere(alerts []models.Alert, n int) []models.Alert {
	ranked := make([]models.Alert, len(alerts))
	copy(ranked, alerts)
	sort.SliceStable(ranked, func(i, j int) bool {
		return severityRank(ranked[i].Severity) > severityRank(ranked[j].Severity)
	})
	if len(ranked) > n {
		ranked = ranked[:n]
	}
	return ranked
}

func severityRank(s models.Severity) int {
	switch s {
	case models.SeverityCritical:
		return 4
	case models.SeverityHigh:
		return 3
	case models.SeverityMedium:
		return 2
	case models.SeverityLow:
		return 1
	}
	return 0
}
