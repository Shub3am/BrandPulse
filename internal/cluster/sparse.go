// A read-only sparse view over a dense TF-IDF Matrix, for the one place that
// pays for density: the initial pairwise distance pass.
//
// This file owns the representation, not the arithmetic that uses it. It must
// not merge, must not hold a cutoff, and must not be exported: Matrix is the
// public shape and CosineDistance is the public operation, both of which stay
// dense so callers and tests are unaffected.
//
// # Why this exists
//
// A TF-IDF row is as wide as the vocabulary and holds one non-zero per distinct
// term in its document. A mention is ~25 tokens; a real brand-day's vocabulary
// is thousands of terms. The dense dot product therefore did |V| multiply-adds
// to consume ~25 of them, which made the pairwise pass scale with the
// vocabulary rather than with the text. Measured on 400 documents over a
// 2,773-term vocabulary the pass went from 179ms to 17ms; on the toy corpus in
// the tests, whose vocabulary is 85 terms, it is worth about 1.2x. The toy
// number is why this was not obvious.
package cluster

// sparseRow is one row's non-zero entries. columns is ascending, which is what
// lets dot walk two rows in one pass, and is a property of how sparsify builds
// it rather than something checked at use.
type sparseRow struct {
	columns []int32
	values  []float64
}

// dot is the inner product of two rows, in O(nnz(a) + nnz(b)).
//
// Both rows come from the same Matrix, so a column index means the same term in
// each. Columns present in one row and not the other contribute zero and are
// skipped rather than multiplied.
func (a sparseRow) dot(b sparseRow) float64 {
	dot := 0.0
	i, j := 0, 0
	for i < len(a.columns) && j < len(b.columns) {
		switch {
		case a.columns[i] < b.columns[j]:
			i++
		case a.columns[i] > b.columns[j]:
			j++
		default:
			dot += a.values[i] * b.values[j]
			i++
			j++
		}
	}
	return dot
}

// sparsify builds the sparse view of rows, backed by two slices for the whole
// matrix rather than two per row.
//
// It panics on a ragged matrix, matching CosineDistance: rows of one Matrix
// always agree, so a mismatch is a caller that hand-built a Matrix, and a stack
// trace beats a plausible wrong number.
func sparsify(rows [][]float64) []sparseRow {
	width, nonZero := -1, 0
	for _, row := range rows {
		if width < 0 {
			width = len(row)
		} else if len(row) != width {
			panic("cluster: Matrix rows of different length")
		}
		for _, v := range row {
			if v != 0 {
				nonZero++
			}
		}
	}

	// Exact capacity, so the appends below never reallocate and the per-row
	// subslices stay pointing at live memory.
	columns := make([]int32, 0, nonZero)
	values := make([]float64, 0, nonZero)

	sparse := make([]sparseRow, len(rows))
	for i, row := range rows {
		start := len(columns)
		for column, v := range row {
			if v != 0 {
				columns = append(columns, int32(column))
				values = append(values, v)
			}
		}
		end := len(columns)
		sparse[i] = sparseRow{
			columns: columns[start:end:end],
			values:  values[start:end:end],
		}
	}
	return sparse
}
