package cluster

import (
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"strings"
	"testing"
)

// toyCorpus is fifteen short documents in three obviously separate topics:
// delivery complaints (0-4), price praise (5-9), packaging damage (10-14).
// Three of them are Hinglish, because that is the register this clusters in
// production and a corpus of clean English would not exercise the stopword
// list at all.
//
// The right answer is visible to a reader, which is the point: this package
// cannot be checked against a library, so the test corpus has to be checkable
// by eye.
var toyCorpus = []string{
	"delivery delayed again order stuck with courier",
	"order delivery late courier never arrived",
	"delivery bahut late hai courier ne order nahi diya",
	"courier delayed my order delivery three days",
	"late delivery order courier problem again",

	"price is great value for money totally worth",
	"great price worth the money value product",
	"price kaafi accha hai value for money worth",
	"value for money price great worth buying",
	"worth the price great value money saved",

	"packaging damaged bottle leaked box crushed",
	"box arrived damaged packaging leaked bottle",
	"packaging bekaar hai bottle leaked box damaged",
	"damaged packaging bottle broken leaked box",
	"box crushed packaging damaged bottle leaked",
}

// toyCutoff sits in the middle of the band that separates this corpus
// correctly. Measured on it: within-topic distances run 0.44 to 0.77 and
// cross-topic distances are exactly 1.0, because the three topics share no
// vocabulary. Every cutoff from 0.80 to 0.98 gives the same three clusters.
const toyCutoff = 0.85

func TestCosineDistance(t *testing.T) {
	cases := []struct {
		name string
		a, b []float64
		want float64
	}{
		{"identical unit vectors are zero apart", []float64{1, 0}, []float64{1, 0}, 0},
		{"orthogonal unit vectors are one apart", []float64{1, 0}, []float64{0, 1}, 1},
		{"opposed unit vectors are two apart", []float64{1, 0}, []float64{-1, 0}, 2},
		{"a zero row is maximally distant", []float64{0, 0}, []float64{1, 0}, 1},
		{
			"forty-five degrees is one minus root-half",
			[]float64{1, 0},
			[]float64{math.Sqrt2 / 2, math.Sqrt2 / 2},
			1 - math.Sqrt2/2,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := CosineDistance(c.a, c.b)
			if math.Abs(got-c.want) > 1e-12 {
				t.Errorf("CosineDistance(%v, %v) = %v, want %v", c.a, c.b, got, c.want)
			}
		})
	}
}

func TestCosineDistancePanicsOnLengthMismatch(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("CosineDistance silently accepted vectors of different length")
		}
	}()
	CosineDistance([]float64{1, 0}, []float64{1, 0, 0})
}

func TestAgglomerativeSeparatesTheToyCorpus(t *testing.T) {
	got := Agglomerative(TFIDF(toyCorpus), toyCutoff, 3)
	want := [][]int{
		{0, 1, 2, 3, 4},
		{5, 6, 7, 8, 9},
		{10, 11, 12, 13, 14},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// The three topics survive anywhere in the band, so the cutoff bp-clusterer
// picks is not balanced on a knife edge. If this narrows, the constant in
// bp-clusterer needs re-measuring rather than nudging.
func TestAgglomerativeCutoffBandIsWide(t *testing.T) {
	m := TFIDF(toyCorpus)
	want := [][]int{{0, 1, 2, 3, 4}, {5, 6, 7, 8, 9}, {10, 11, 12, 13, 14}}

	for _, cutoff := range []float64{0.80, 0.85, 0.90, 0.95, 0.98} {
		if got := Agglomerative(m, cutoff, 3); !reflect.DeepEqual(got, want) {
			t.Errorf("cutoff %.2f gave %v, want %v", cutoff, got, want)
		}
	}
}

func TestAgglomerativeMinSizeFilters(t *testing.T) {
	m := TFIDF(toyCorpus)

	if got := Agglomerative(m, toyCutoff, 6); len(got) != 0 {
		t.Errorf("minSize 6 over three clusters of five returned %v, want nothing", got)
	}
	if got := Agglomerative(m, toyCutoff, 5); len(got) != 3 {
		t.Errorf("minSize 5 returned %d clusters, want 3", len(got))
	}
}

// Size descending, ties on the smallest member index. Built by hand rather
// than from the toy corpus so the ordering is asserted independently of
// whatever the corpus happens to produce.
func TestAgglomerativeOrdersBySizeThenFirstMember(t *testing.T) {
	// Six rows in three groups: one pair at columns 0-1 (rows 0,3), one
	// triple at column 2 (rows 1,2,5), one pair at column 3 (rows 4 and... )
	m := Matrix{
		Terms: []string{"a", "b", "c", "d"},
		Rows: [][]float64{
			{0, 0, 0, 1}, // 0  d-group
			{0, 0, 1, 0}, // 1  c-group
			{0, 0, 1, 0}, // 2  c-group
			{0, 0, 0, 1}, // 3  d-group
			{0, 0, 1, 0}, // 4  c-group
			{0, 0, 0, 1}, // 5  d-group
		},
	}

	got := Agglomerative(m, 0.5, 2)
	want := [][]int{{0, 3, 5}, {1, 2, 4}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v: equal sizes must break on the smallest member index", got, want)
	}
}

func TestAgglomerativeEdgeCases(t *testing.T) {
	if got := Agglomerative(Matrix{}, toyCutoff, 3); len(got) != 0 {
		t.Errorf("empty Matrix returned %v", got)
	}
	single := TFIDF([]string{"delivery delayed again"})
	if got := Agglomerative(single, toyCutoff, 1); !reflect.DeepEqual(got, [][]int{{0}}) {
		t.Errorf("one document returned %v, want [[0]]", got)
	}
	if got := Agglomerative(single, toyCutoff, 3); len(got) != 0 {
		t.Errorf("one document under minSize 3 returned %v", got)
	}
}

// TestDeterminism is the reproducibility guarantee the demo and eval/ both
// rest on. Go randomises map iteration, so a single lucky run proves nothing:
// this builds the Matrix once and clusters it twenty times. Run with -count=5.
func TestDeterminism(t *testing.T) {
	m := TFIDF(toyCorpus)
	first := Agglomerative(m, toyCutoff, 3)

	for run := 1; run < 20; run++ {
		if got := Agglomerative(m, toyCutoff, 3); !reflect.DeepEqual(got, first) {
			t.Fatalf("run %d returned %v, first run returned %v", run, got, first)
		}
	}

	// And again from a freshly built Matrix, which is where an unsorted
	// vocabulary would show up as a different column layout.
	for run := 0; run < 20; run++ {
		if got := Agglomerative(TFIDF(toyCorpus), toyCutoff, 3); !reflect.DeepEqual(got, first) {
			t.Fatalf("rebuilt run %d returned %v, first run returned %v", run, got, first)
		}
	}
}

// benchCorpus is 400 documents, the size of a busy brand-day, across eight
// topics with per-document variation so the vocabulary is realistic rather
// than eight repeated strings.
func benchCorpus() []string {
	templates := []string{
		"delivery %d delayed again courier stuck shipment",
		"price %d value money worth discount offer",
		"packaging %d damaged bottle leaked box crushed",
		"quality %d product ingredients formula texture",
		"support %d refund replacement ticket agent response",
		"shampoo %d hairfall scalp dandruff oily roots",
		"skin %d serum acne glow dryness patch",
		"order %d cancelled payment gateway failed refund",
	}
	docs := make([]string, 0, 400)
	for i := 0; i < 400; i++ {
		docs = append(docs, fmt.Sprintf(templates[i%len(templates)], i/len(templates)))
	}
	return docs
}

// wideVocabularyCorpus is also 400 documents, but each one is a topic core
// padded with words drawn from a 3,000-word background, giving a ~2,700-term
// vocabulary instead of benchCorpus's 85.
//
// Both corpora are here on purpose. Cost in this package scales with the
// vocabulary, not the document count, so benchCorpus alone reports a number
// that is true only of itself: it is what made the dense pairwise pass look
// like 10ms of work when a realistic window was closer to 180ms. Keep both, and
// distrust a speedup that only shows up on the narrow one.
//
// The generator is seeded, so the corpus is the same on every run.
func wideVocabularyCorpus() []string {
	cores := [][]string{
		{"delivery", "delayed", "courier", "shipment", "stuck"},
		{"price", "value", "money", "discount", "offer"},
		{"packaging", "damaged", "bottle", "leaked", "crushed"},
		{"quality", "ingredients", "formula", "texture", "product"},
		{"support", "refund", "replacement", "ticket", "agent"},
		{"shampoo", "hairfall", "scalp", "dandruff", "roots"},
		{"serum", "acne", "glow", "dryness", "patch"},
		{"order", "cancelled", "payment", "gateway", "failed"},
	}

	background := make([]string, 3000)
	for i := range background {
		background[i] = fmt.Sprintf("bg%dword", i)
	}

	random := rand.New(rand.NewSource(1))
	docs := make([]string, 0, 400)
	for i := 0; i < 400; i++ {
		tokens := append([]string{}, cores[i%len(cores)]...)
		for len(tokens) < 25 {
			tokens = append(tokens, background[random.Intn(len(background))])
		}
		docs = append(docs, strings.Join(tokens, " "))
	}
	return docs
}

func BenchmarkTFIDF400(b *testing.B) {
	docs := benchCorpus()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		TFIDF(docs)
	}
}

func BenchmarkAgglomerative400(b *testing.B) {
	m := TFIDF(benchCorpus())
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Agglomerative(m, toyCutoff, 3)
	}
}

func BenchmarkTFIDF400WideVocabulary(b *testing.B) {
	docs := wideVocabularyCorpus()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		TFIDF(docs)
	}
}

func BenchmarkAgglomerative400WideVocabulary(b *testing.B) {
	m := TFIDF(wideVocabularyCorpus())
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Agglomerative(m, toyCutoff, 3)
	}
}
