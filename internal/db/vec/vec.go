// Package vec provides a small, pure-Go vector utility for KNN search.
//
// Plan C of the T02 spike: sqlite-vec has no working pure-Go implementation,
// so we ship a small in-memory cosine-similarity scorer instead. Vectors are
// stored as []float32 (and serialized as little-endian BLOBs by the repo
// layer) and matched against a query with a brute-force top-K scan.
package vec

import (
	"encoding/binary"
	"errors"
	"math"
	"sort"
)

// Hit is a single TopK result.
type Hit struct {
	ID    int64
	Score float32
}

// Cosine returns the cosine similarity of two equal-length float32 vectors.
// If either vector has zero magnitude or lengths differ, it returns 0.
func Cosine(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		av := float64(a[i])
		bv := float64(b[i])
		dot += av * bv
		na += av * av
		nb += bv * bv
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return float32(dot / (math.Sqrt(na) * math.Sqrt(nb)))
}

// TopK returns the k items in `items` most similar (by cosine) to `query`.
// Results are sorted descending by score. If k <= 0 it returns nil. If there
// are fewer items than k, all items are returned.
func TopK(query []float32, items map[int64][]float32, k int) []Hit {
	if k <= 0 || len(items) == 0 {
		return nil
	}
	hits := make([]Hit, 0, len(items))
	for id, v := range items {
		hits = append(hits, Hit{ID: id, Score: Cosine(query, v)})
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Score == hits[j].Score {
			return hits[i].ID < hits[j].ID
		}
		return hits[i].Score > hits[j].Score
	})
	if len(hits) > k {
		hits = hits[:k]
	}
	return hits
}

// Encode serializes a []float32 vector as little-endian bytes, suitable for
// storing in a SQLite BLOB column.
func Encode(v []float32) []byte {
	out := make([]byte, 4*len(v))
	for i, f := range v {
		binary.LittleEndian.PutUint32(out[i*4:], math.Float32bits(f))
	}
	return out
}

// Decode reverses Encode. It returns an error if len(b) is not a multiple of 4.
func Decode(b []byte) ([]float32, error) {
	if len(b)%4 != 0 {
		return nil, errors.New("vec.Decode: byte length not a multiple of 4")
	}
	out := make([]float32, len(b)/4)
	for i := range out {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return out, nil
}
