// Package irt は Item Response Theory (2PL) ベースの適応出題ロジックを提供する。
//
// 2PL モデル:
//
//	P(correct | theta, a, b) = 1 / (1 + exp(-a*(theta - b)))
//	I(theta) = a^2 * p * (1-p)
//	SEM(theta) = 1 / sqrt(sum I_i(theta))
package irt

import "math"

// Item は出題候補となるアイテム。
type Item struct {
	ID string
	A  float64 // 識別力
	B  float64 // 難易度
}

// Response は受験者の解答履歴 1 件。
type Response struct {
	A       float64
	B       float64
	Correct bool
}

// Prob は 2PL モデルにおける正答確率。
func Prob(theta, a, b float64) float64 {
	return 1.0 / (1.0 + math.Exp(-a*(theta-b)))
}

// Info は theta における 2PL のアイテム情報量。
func Info(theta, a, b float64) float64 {
	p := Prob(theta, a, b)
	return a * a * p * (1.0 - p)
}

// UpdateTheta は Newton-Raphson 1ステップで MLE 推定値を更新する。
// items が 0 件の場合は初期 theta をそのまま返す。
// theta は範囲外発散を避けるため [-4, 4] にクリップする。
func UpdateTheta(theta float64, items []Response) float64 {
	if len(items) == 0 {
		return theta
	}
	// 1 ステップで十分収束するが、安定のため最大 5 回までニュートン法を回す。
	for i := 0; i < 5; i++ {
		var grad, hess float64
		for _, r := range items {
			p := Prob(theta, r.A, r.B)
			u := 0.0
			if r.Correct {
				u = 1.0
			}
			grad += r.A * (u - p)
			hess += -r.A * r.A * p * (1.0 - p)
		}
		if hess == 0 {
			break
		}
		delta := grad / hess
		theta -= delta
		if math.Abs(delta) < 1e-4 {
			break
		}
	}
	if theta > 4.0 {
		theta = 4.0
	} else if theta < -4.0 {
		theta = -4.0
	}
	return theta
}

// SEM は theta における推定標準誤差。
// items が 0 件 / 情報量 0 のときは +Inf を返す。
func SEM(theta float64, items []Response) float64 {
	var sum float64
	for _, r := range items {
		sum += Info(theta, r.A, r.B)
	}
	if sum <= 0 {
		return math.Inf(1)
	}
	return 1.0 / math.Sqrt(sum)
}

// SelectNext は pool から asked に含まれない最大 Info(theta) のアイテムを返す。
// 候補が無い場合は nil。
func SelectNext(theta float64, pool []Item, asked []string) *Item {
	askedSet := make(map[string]struct{}, len(asked))
	for _, id := range asked {
		askedSet[id] = struct{}{}
	}
	var best *Item
	var bestInfo float64 = -1
	for i := range pool {
		it := &pool[i]
		if _, ok := askedSet[it.ID]; ok {
			continue
		}
		info := Info(theta, it.A, it.B)
		if info > bestInfo {
			bestInfo = info
			best = it
		}
	}
	return best
}

// ThetaToCEFR は theta を CEFR レベル文字列に変換する。
func ThetaToCEFR(theta float64) string {
	switch {
	case theta < -1.5:
		return "A1"
	case theta < -0.5:
		return "A2"
	case theta < 0.5:
		return "B1"
	case theta < 1.5:
		return "B2"
	case theta < 2.5:
		return "C1"
	default:
		return "C2"
	}
}
