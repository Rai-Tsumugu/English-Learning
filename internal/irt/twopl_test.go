package irt

import (
	"math"
	"testing"
)

func TestProb(t *testing.T) {
	// theta == b なら p = 0.5
	if p := Prob(0, 1, 0); math.Abs(p-0.5) > 1e-9 {
		t.Fatalf("Prob(0,1,0) = %v, want 0.5", p)
	}
	// theta >> b なら p -> 1
	if p := Prob(5, 1, 0); p < 0.99 {
		t.Fatalf("Prob(5,1,0)=%v expected >0.99", p)
	}
	// theta << b なら p -> 0
	if p := Prob(-5, 1, 0); p > 0.01 {
		t.Fatalf("Prob(-5,1,0)=%v expected <0.01", p)
	}
}

func TestInfo(t *testing.T) {
	// theta == b で最大、a^2 * 0.25
	got := Info(0, 1, 0)
	want := 0.25
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("Info(0,1,0)=%v want %v", got, want)
	}
	// a が大きいほど情報量も大きい
	if Info(0, 2, 0) <= Info(0, 1, 0) {
		t.Fatalf("expected Info increasing in a")
	}
	// 極端な theta では情報量小
	if Info(5, 1, 0) >= Info(0, 1, 0) {
		t.Fatalf("expected info to drop far from b")
	}
}

func TestUpdateTheta_HighDifficultyCorrectRaisesTheta(t *testing.T) {
	// 高難易度 (b=2.0) を連続正答 -> theta は 0 から上昇するはず
	items := []Response{
		{A: 1.2, B: 2.0, Correct: true},
		{A: 1.2, B: 2.0, Correct: true},
		{A: 1.2, B: 2.0, Correct: true},
		{A: 1.2, B: 1.5, Correct: true},
	}
	got := UpdateTheta(0, items)
	if got <= 0 {
		t.Fatalf("expected theta > 0 after correct on hard items, got %v", got)
	}
}

func TestUpdateTheta_EasyWrongLowersTheta(t *testing.T) {
	// 易しい問題 (b=-1.5) を連続誤答 -> theta は低下
	items := []Response{
		{A: 1.0, B: -1.5, Correct: false},
		{A: 1.0, B: -1.5, Correct: false},
		{A: 1.0, B: -1.0, Correct: false},
	}
	got := UpdateTheta(0, items)
	if got >= 0 {
		t.Fatalf("expected theta < 0, got %v", got)
	}
}

func TestUpdateTheta_Empty(t *testing.T) {
	if v := UpdateTheta(0.3, nil); v != 0.3 {
		t.Fatalf("expected unchanged theta, got %v", v)
	}
}

func TestSEM(t *testing.T) {
	if v := SEM(0, nil); !math.IsInf(v, 1) {
		t.Fatalf("expected +Inf for empty, got %v", v)
	}
	items := []Response{
		{A: 1, B: 0, Correct: true},
	}
	got := SEM(0, items)
	// Info=0.25, SEM=1/sqrt(0.25)=2
	if math.Abs(got-2.0) > 1e-9 {
		t.Fatalf("SEM=%v want 2", got)
	}
}

func TestSelectNext_MaxInfo(t *testing.T) {
	pool := []Item{
		{ID: "easy", A: 1, B: -3},
		{ID: "match", A: 1, B: 0},
		{ID: "hard", A: 1, B: 3},
	}
	got := SelectNext(0, pool, nil)
	if got == nil || got.ID != "match" {
		t.Fatalf("expected match, got %+v", got)
	}
}

func TestSelectNext_ExcludesAsked(t *testing.T) {
	pool := []Item{
		{ID: "match", A: 1, B: 0},
		{ID: "near", A: 1, B: 0.2},
	}
	got := SelectNext(0, pool, []string{"match"})
	if got == nil || got.ID != "near" {
		t.Fatalf("expected near, got %+v", got)
	}
	// すべて除外 -> nil
	if v := SelectNext(0, pool, []string{"match", "near"}); v != nil {
		t.Fatalf("expected nil, got %+v", v)
	}
}

func TestThetaToCEFR(t *testing.T) {
	cases := []struct {
		theta float64
		want  string
	}{
		{-2, "A1"}, {-1, "A2"}, {0, "B1"}, {1, "B2"}, {2, "C1"}, {3, "C2"},
	}
	for _, c := range cases {
		if got := ThetaToCEFR(c.theta); got != c.want {
			t.Fatalf("ThetaToCEFR(%v)=%s want %s", c.theta, got, c.want)
		}
	}
}
