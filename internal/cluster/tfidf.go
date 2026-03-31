// Package cluster is hand-rolled TF-IDF and average-linkage agglomerative
// clustering over short documents. Go has no scikit-learn and the one
// agglomerative package on pkg.go.dev has had no release since 2020, so the
// demo's centrepiece is written out here instead of depended upon.
//
// It imports only the standard library, and it knows about []string and
// nothing else: no models, no context, no I/O, no logging. That is what makes
// it testable on a fixed toy corpus without a fixture, a database or a key.
//
// # Determinism is the requirement, not a nicety
//
// Go randomises map iteration order on purpose. Every map in this package has
// its keys sorted before they influence output: the vocabulary, the
// document-frequency walk and the final cluster ordering. Two runs over the
// same corpus must produce the same clusters in the same order, or the demo is
// not reproducible and eval/ is measuring noise. Treat a `for k := range m`
// that feeds output as a bug.
//
// This file owns vectorisation. It must not decide how many clusters there
// are, and it must not hold the distance cutoff: that is the caller's choice.
package cluster

import (
	"math"
	"sort"
	"strings"
	"unicode"
)

// Matrix is one corpus as L2-normalised TF-IDF row vectors. Rows[i] is the
// vector for docs[i] and keeps the caller's input order. Terms is the
// vocabulary in column order, sorted, so the column layout is reproducible.
type Matrix struct {
	Rows  [][]float64
	Terms []string
}

// Tokenize lowercases, splits on non-letter non-digit runes, drops stopwords
// and tokens shorter than two runes.
//
// Combining marks count as part of a token alongside letters and digits.
// Devanagari vowel signs are nonspacing marks rather than letters, so a
// letter-and-digit-only split tears "डिलीवरी" into "वर" and drops three of its
// runes on the floor. Keeping unicode.IsMark is what makes the brief's
// "Devanagari survives" true in practice.
func Tokenize(text string) []string {
	fields := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && !unicode.IsMark(r)
	})

	tokens := make([]string, 0, len(fields))
	for _, f := range fields {
		// Rune count, not byte length: a two-rune Devanagari token is six
		// bytes and must not be dropped as if it were one character.
		if len([]rune(f)) < 2 {
			continue
		}
		if _, isStop := stopwords[f]; isStop {
			continue
		}
		tokens = append(tokens, f)
	}
	return tokens
}

// TFIDF tokenises, drops stopwords, and builds the normalised term-frequency
// times inverse-document-frequency matrix for docs.
//
// The IDF is log(N / (1 + df)), which is zero for a term appearing in exactly
// N-1 documents and negative for one appearing in all N. That is intended: a
// term every document shares separates nothing. A document whose every term is
// corpus-wide therefore vectorises to all zeros, and is left at zero rather
// than normalised into NaN. CosineDistance then reports it as maximally
// distant from everything, which is the honest answer.
func TFIDF(docs []string) Matrix {
	tokenized := make([][]string, len(docs))
	documentFrequency := map[string]int{}

	for i, doc := range docs {
		tokenized[i] = Tokenize(doc)

		seen := map[string]struct{}{}
		for _, term := range tokenized[i] {
			if _, already := seen[term]; already {
				continue
			}
			seen[term] = struct{}{}
			documentFrequency[term]++
		}
	}

	terms := make([]string, 0, len(documentFrequency))
	for term := range documentFrequency {
		terms = append(terms, term)
	}
	sort.Strings(terms)

	columnOf := make(map[string]int, len(terms))
	inverseDocumentFrequency := make([]float64, len(terms))
	corpusSize := float64(len(docs))
	for column, term := range terms {
		columnOf[term] = column
		inverseDocumentFrequency[column] = math.Log(corpusSize / (1 + float64(documentFrequency[term])))
	}

	rows := make([][]float64, len(docs))
	for i, tokens := range tokenized {
		row := make([]float64, len(terms))
		for _, term := range tokens {
			row[columnOf[term]]++
		}
		for column := range row {
			row[column] *= inverseDocumentFrequency[column]
		}
		normalise(row)
		rows[i] = row
	}

	return Matrix{Rows: rows, Terms: terms}
}

// normalise scales row to unit L2 length in place, so cosine similarity is a
// plain dot product. A zero row stays zero: there is no direction to preserve
// and dividing would produce NaN, which marshals as a JSON error rather than a
// number.
func normalise(row []float64) {
	sumOfSquares := 0.0
	for _, v := range row {
		sumOfSquares += v * v
	}
	if sumOfSquares == 0 {
		return
	}
	length := math.Sqrt(sumOfSquares)
	for i := range row {
		row[i] /= length
	}
}
