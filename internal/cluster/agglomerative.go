// Average-linkage agglomerative merging over a TF-IDF Matrix.
//
// This file owns grouping. It must not tokenise, must not pick a cutoff, and
// must not know what a topic is.
//
// # Why the naive implementation is the right one
//
// It precomputes the full pairwise distance matrix and rescans every active
// pair on every merge. At the 400 mentions a brand-day produces that is
// 160,000 floats and at most 400 scans of a shrinking triangle, which runs in
// single-digit milliseconds. A nearest-neighbour chain or a priority queue
// would be asymptotically better and materially harder to read, and this is
// the one algorithm in the repo nobody can check against a library. The
// reading cost is the cost that matters here.
//
// Cluster-to-cluster distances are kept in the same matrix and updated with
// the Lance-Williams rule, which is exact for average linkage: merging A and B
// gives D(A+B, C) = (|A|*D(A,C) + |B|*D(B,C)) / (|A| + |B|). That avoids
// re-summing every member pair on every merge without changing the answer.
package cluster

import "sort"

// CosineDistance is 1 minus the dot product of two L2-normalised vectors.
//
// It panics on a length mismatch. Two rows of the same Matrix always match, so
// a mismatch is a caller bug, and silently truncating to the shorter vector
// would return a plausible wrong number instead of a stack trace.
func CosineDistance(a, b []float64) float64 {
	if len(a) != len(b) {
		panic("cluster: CosineDistance on vectors of different length")
	}
	dot := 0.0
	for i := range a {
		dot += a[i] * b[i]
	}
	return 1 - dot
}

// Agglomerative merges rows of m by average linkage until the closest pair of
// clusters exceeds cutoff cosine distance, then returns only the clusters with
// at least minSize members. Member indices index into m.Rows. Members are
// sorted ascending; clusters are sorted by size descending, ties broken by
// smallest member index. Two runs over the same Matrix return the identical
// slice of slices.
func Agglomerative(m Matrix, cutoff float64, minSize int) [][]int {
	rowCount := len(m.Rows)
	if rowCount == 0 {
		return [][]int{}
	}

	distance := make([][]float64, rowCount)
	for i := range distance {
		distance[i] = make([]float64, rowCount)
	}
	for i := 0; i < rowCount; i++ {
		for j := i + 1; j < rowCount; j++ {
			d := CosineDistance(m.Rows[i], m.Rows[j])
			distance[i][j] = d
			distance[j][i] = d
		}
	}

	// Slot i holds the cluster that started as row i. Merging folds the higher
	// slot into the lower and deactivates it, so slot indices stay stable and
	// "lowest (i, j)" is a well-defined tie-break for the whole run.
	members := make([][]int, rowCount)
	active := make([]bool, rowCount)
	for i := range members {
		members[i] = []int{i}
		active[i] = true
	}

	for {
		closestI, closestJ, closestDistance := closestPair(distance, active)
		if closestI < 0 || closestDistance > cutoff {
			break
		}
		mergeInto(closestI, closestJ, members, active, distance)
	}

	return collect(members, active, minSize)
}

// closestPair scans the upper triangle in index order, so the first pair at
// the minimum wins and ties resolve to the lowest (i, j). A strict less-than
// is what makes that true; >= would take the last tie instead. It returns
// i = -1 when fewer than two clusters remain.
func closestPair(distance [][]float64, active []bool) (int, int, float64) {
	bestI, bestJ := -1, -1
	best := 0.0
	for i := range active {
		if !active[i] {
			continue
		}
		for j := i + 1; j < len(active); j++ {
			if !active[j] {
				continue
			}
			if bestI < 0 || distance[i][j] < best {
				bestI, bestJ, best = i, j, distance[i][j]
			}
		}
	}
	return bestI, bestJ, best
}

// mergeInto folds cluster j into cluster i and updates i's distance to every
// other active cluster by the Lance-Williams average-linkage rule. Sizes are
// read before the member lists change, because the rule weights by the sizes
// of the two clusters as they were.
func mergeInto(i, j int, members [][]int, active []bool, distance [][]float64) {
	sizeI := float64(len(members[i]))
	sizeJ := float64(len(members[j]))

	for other := range active {
		if !active[other] || other == i || other == j {
			continue
		}
		merged := (sizeI*distance[i][other] + sizeJ*distance[j][other]) / (sizeI + sizeJ)
		distance[i][other] = merged
		distance[other][i] = merged
	}

	members[i] = append(members[i], members[j]...)
	sort.Ints(members[i])
	members[j] = nil
	active[j] = false
}

// collect returns the surviving clusters that meet minSize, largest first,
// ties broken by smallest member index. Both the filter and the sort run off
// slices rather than a map walk, so the output order is fixed.
func collect(members [][]int, active []bool, minSize int) [][]int {
	out := [][]int{}
	for i := range active {
		if active[i] && len(members[i]) >= minSize {
			out = append(out, members[i])
		}
	}
	sort.SliceStable(out, func(a, b int) bool {
		if len(out[a]) != len(out[b]) {
			return len(out[a]) > len(out[b])
		}
		return out[a][0] < out[b][0]
	})
	return out
}
