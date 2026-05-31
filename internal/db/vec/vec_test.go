package vec

import "testing"

func TestCosine(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{1, 0, 0}
	if s := Cosine(a, b); s < 0.999 {
		t.Fatalf("identical vectors should score ~1, got %v", s)
	}
	c := []float32{0, 1, 0}
	if s := Cosine(a, c); s != 0 {
		t.Fatalf("orthogonal vectors should score 0, got %v", s)
	}
}

func TestTopK(t *testing.T) {
	items := map[int64][]float32{
		1: {1, 0, 0, 0, 0},
		2: {0.9, 0.1, 0, 0, 0},
		3: {0, 1, 0, 0, 0},
	}
	got := TopK([]float32{1, 0, 0, 0, 0}, items, 2)
	if len(got) != 2 {
		t.Fatalf("want 2 hits, got %d", len(got))
	}
	if got[0].ID != 1 {
		t.Errorf("top result should be ID=1, got %d", got[0].ID)
	}
	if got[1].ID != 2 {
		t.Errorf("second result should be ID=2, got %d", got[1].ID)
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	v := []float32{0.1, -0.2, 3.14, 42, -0.0001}
	got, err := Decode(Encode(v))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != len(v) {
		t.Fatalf("len mismatch: got %d want %d", len(got), len(v))
	}
	for i := range v {
		if got[i] != v[i] {
			t.Errorf("at %d: got %v want %v", i, got[i], v[i])
		}
	}
}
