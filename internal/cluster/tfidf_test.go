package cluster

import (
	"math"
	"sort"
	"testing"
)

func TestTokenize(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{
			name: "lowercases and splits on punctuation",
			in:   "Delivery DELAYED, again!! Order #4471.",
			want: []string{"delivery", "delayed", "order", "4471"},
		},
		{
			name: "drops one-rune tokens",
			in:   "a b delivery c 9 order",
			want: []string{"delivery", "order"},
		},
		{
			name: "drops english stopwords",
			in:   "the order is with the courier and it was late",
			want: []string{"order", "courier", "late"},
		},
		{
			name: "drops hinglish stopwords but keeps hinglish content words",
			in:   "product bahut bakwas hai yaar delivery bhi nahi aayi",
			want: []string{"product", "bakwas", "delivery", "aayi"},
		},
		{
			name: "keeps devanagari, splitting on script boundaries only",
			in:   "डिलीवरी बहुत late hai",
			want: []string{"डिलीवरी", "बहुत", "late"},
		},
		{
			name: "digits survive as tokens",
			in:   "rated 2 stars, 15 days late",
			want: []string{"rated", "stars", "15", "days", "late"},
		},
		{
			name: "empty input yields no tokens",
			in:   "   !!! ??? ",
			want: []string{},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Tokenize(c.in)
			if len(got) != len(c.want) {
				t.Fatalf("Tokenize(%q) = %v, want %v", c.in, got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Fatalf("Tokenize(%q) = %v, want %v", c.in, got, c.want)
				}
			}
		})
	}
}

func TestTFIDFShapeAndOrder(t *testing.T) {
	m := TFIDF(toyCorpus)

	if len(m.Rows) != len(toyCorpus) {
		t.Fatalf("got %d rows for %d docs", len(m.Rows), len(toyCorpus))
	}
	if !sort.StringsAreSorted(m.Terms) {
		t.Errorf("Terms is not sorted, so the column layout is not reproducible: %v", m.Terms)
	}
	for i, row := range m.Rows {
		if len(row) != len(m.Terms) {
			t.Fatalf("row %d has width %d, vocabulary is %d", i, len(row), len(m.Terms))
		}
	}
}

func TestTFIDFRowsAreL2Normalised(t *testing.T) {
	m := TFIDF(toyCorpus)
	for i, row := range m.Rows {
		sumOfSquares := 0.0
		for _, v := range row {
			sumOfSquares += v * v
		}
		if math.Abs(sumOfSquares-1.0) > 1e-9 {
			t.Errorf("row %d has squared length %v, want 1.0", i, sumOfSquares)
		}
	}
}

// A term present in exactly N-1 documents gets idf log(N/N) = 0. A document
// built only from such terms therefore vectorises to all zeros, and the
// normalise step must leave it alone rather than divide by zero. A NaN here
// does not fail loudly: it marshals as "json: unsupported value" three agents
// downstream.
func TestTFIDFZeroRowDoesNotProduceNaN(t *testing.T) {
	m := TFIDF([]string{"alpha beta", "alpha beta", "gamma delta"})

	for i, row := range m.Rows {
		for column, v := range row {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				t.Fatalf("row %d column %d is %v", i, column, v)
			}
		}
	}
	for _, i := range []int{0, 1} {
		for column, v := range m.Rows[i] {
			if v != 0 {
				t.Errorf("row %d column %d (%q) = %v, want 0: every term is corpus-wide",
					i, column, m.Terms[column], v)
			}
		}
	}
}

func TestTFIDFIsDeterministic(t *testing.T) {
	first := TFIDF(toyCorpus)
	for run := 0; run < 20; run++ {
		again := TFIDF(toyCorpus)
		if len(again.Terms) != len(first.Terms) {
			t.Fatalf("run %d: vocabulary size changed", run)
		}
		for i := range first.Terms {
			if again.Terms[i] != first.Terms[i] {
				t.Fatalf("run %d: term %d is %q, first run had %q",
					run, i, again.Terms[i], first.Terms[i])
			}
		}
		for i := range first.Rows {
			for j := range first.Rows[i] {
				if again.Rows[i][j] != first.Rows[i][j] {
					t.Fatalf("run %d: cell (%d,%d) is %v, first run had %v",
						run, i, j, again.Rows[i][j], first.Rows[i][j])
				}
			}
		}
	}
}

func TestTFIDFEmptyCorpus(t *testing.T) {
	m := TFIDF(nil)
	if len(m.Rows) != 0 || len(m.Terms) != 0 {
		t.Fatalf("empty corpus gave %d rows and %d terms", len(m.Rows), len(m.Terms))
	}
}
