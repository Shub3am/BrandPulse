// cost.go is the one place a token count becomes money. eval/cost reads this
// table, the dashboard reads eval/cost, and the pitch deck reads the dashboard,
// so a second copy of these numbers anywhere in the repo is a bug.
//
// It must not price the model an agent asked for. The Nasiko router discards
// the request's model field and resolves the provider server-side, so the only
// model we ever paid for is the one the response reports back.
package llm

import "strings"

// usdPerMTok is the published list price of one million tokens, in US dollars.
//
// Sources, read 2026-09-20:
//   - gpt-4o $2.50 / $10.00 and gpt-4o-mini $0.15 / $0.60, developers.openai.com/api/docs/pricing
//   - claude-3-5-sonnet $3.00 / $15.00, platform.claude.com/docs/en/about-claude/pricing
//   - gemini-1.5-pro $3.50 / $10.50, llmpricecheck.com/google/gemini-1.5-pro
//
// The Gemini entry is the weak one: Google has retired 1.5 Pro from its own
// published price list, so this is an aggregator's figure and it is the highest
// in circulation. That direction is deliberate. Over-reporting a rupee is a
// number we defend; under-reporting one is a number that collapses on stage.
type usdPerMTok struct {
	in  float64
	out float64
}

// modelPrices is keyed by the model string the router returns, catalog form
// first. Lookup falls back to the bare name, because a proxy that strips its
// own prefix on the way back is a thing proxies do.
var modelPrices = map[string]usdPerMTok{
	"openai/gpt-4o":                        {in: 2.50, out: 10.00},
	"openai/gpt-4o-mini":                   {in: 0.15, out: 0.60},
	"anthropic/claude-3-5-sonnet-20241022": {in: 3.00, out: 15.00},
	"gemini/gemini-1.5-pro":                {in: 3.50, out: 10.50},
}

// rupeesPerUSD is the RBI reference rate on 2026-09-17, 1 USD = 95.885 INR,
// from federalreserve.gov/releases/h10. A hackathon does not hedge currency;
// this is a constant we re-read on the morning of the demo, not a live quote.
const rupeesPerUSD = 95.885

// costPaise prices one completion. An unrecognised model is charged at the most
// expensive row in the table rather than at zero: a model we have never seen is
// a model whose price we do not know, and a silent 0.00 would make the cost
// dashboard confidently wrong.
func costPaise(model string, promptTokens, completionTokens int) float64 {
	price, known := priceFor(model)
	if !known {
		price = mostExpensivePrice()
	}
	usd := float64(promptTokens)/1e6*price.in + float64(completionTokens)/1e6*price.out
	return usd * rupeesPerUSD * 100
}

func priceFor(model string) (usdPerMTok, bool) {
	if price, ok := modelPrices[model]; ok {
		return price, true
	}
	bare := model[strings.LastIndex(model, "/")+1:]
	for catalogName, price := range modelPrices {
		if catalogName[strings.LastIndex(catalogName, "/")+1:] == bare {
			return price, true
		}
	}
	return usdPerMTok{}, false
}

func mostExpensivePrice() usdPerMTok {
	var worst usdPerMTok
	for _, price := range modelPrices {
		if price.in+price.out > worst.in+worst.out {
			worst = price
		}
	}
	return worst
}
