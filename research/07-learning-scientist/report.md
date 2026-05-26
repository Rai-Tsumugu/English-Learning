# 学習科学エビデンス評価レポート: English-Learning 設計 v0.2

評価者: Learning Science / Cognitive Psychology 研究者視点
対象: `/docs/design.md` (v0.2)
評価日: 2026-05-21

---

## 1. Spaced Repetition (SRS): SM-2 vs FSRS

### エビデンス
分散学習効果（Spacing Effect）は Cepeda et al. (2006, 2008) のメタ分析で確立されており、間隔をあけた復習は集中学習に対し平均効果量 d ≈ 0.4〜0.6 で長期保持を向上させる。アルゴリズム面では、Expertium による約7億件の Anki レビューに基づく公開ベンチマーク（2023–2024）で、**FSRS-5 は目標保持率90%に対し誤差 ±5.3%、SM-2 は ±16.2%**。同一保持率を維持しつつレビュー数を 20〜30% 削減でき、**99%以上のユーザーで FSRS が SM-2 を上回る**。SM-2 は 1987 年のヒューリスティックで、連続誤答による間隔崩壊（"ease hell"）など既知の欠陥を抱える。

### 設計への適用度: ○
SM-2 採用は実装容易性で合理的だが、学習効率の観点では最良ではない。

### 推奨改善
- **Phase2 で FSRS-5 への移行を計画化**。`attempts` に `stability`, `difficulty`, `lapses`, `last_review_at` 列を追加できるようマイグレーション設計を準備。
- 目標保持率（desired retention）を `users.difficulty_profile` に格納し可変化（初学者 0.85 / 上級者 0.90）。
- 出典: [Expertium SR Benchmark](https://expertium.github.io/Benchmark.html) / [FSRS-5 vs SM-2](https://www.diane.app/en/guides/fsrs-vs-sm2)

---

## 2. Testing Effect / Retrieval Practice

### エビデンス
Rowland (2014) のメタ分析（159研究）で **retrieval practice vs restudy の効果量 g = 0.50**、81%の比較で retrieval 優位。Adesope et al. (2017) は **g = 0.61**（118研究）。Karpicke & Smith (2012) は語彙学習において想起練習が keyword mnemonic 等の elaborative strategy を上回ることを実証。効果は **(a) 想起努力が大きいほど、(b) フィードバックがあるほど、(c) 内容が複雑なほど** 強い。

### 設計への適用度: ◎
4択クイズで能動的想起を要求、`attempts` に latency_ms と result を記録、ReviewAgent が解説（フィードバック）を生成する設計は retrieval practice 理論と高度に整合。

### 推奨改善
- 4択（再認）に加え、**産出型タスク**（タイピング/和訳→英訳）を一部混在させ desirable difficulty を確保。再認のみでは効果量が約半減する。
- フィードバックは「解答直後の即時提示」を固定。
- 出典: [Rowland 2014 メタ分析](https://pdf.retrievalpractice.org/MetaAnalysisGuide.pdf) / [Schwieren et al. 2017](https://journals.sagepub.com/doi/10.1177/1475725717695149)

---

## 3. 適応型プレースメントテスト (CAT)

### エビデンス
CAT は IRT (Item Response Theory) ベースで、固定長テストの 1/3〜1/2 の項目数で同等精度を達成（Weiss & Kingsbury, 1984）。実証例として SLUPE は平均 22.7 項目で英語レベル判定。一般に **能力推定の標準誤差 SEM ≤ 0.3 logit** を停止規準とし、**最低 15〜25 項目** で安定推定が可能。ただし IRT パラメータ（識別力 a、困難度 b）の事前較正が必須で、ad hoc な難易度設定では精度が劣化する。

### 設計への適用度: △
「20問固定」は妥当な水準だが、設計書には **項目較正・停止規準・能力推定方式が明記されていない**。CEFR 自己申告との整合チェックも未定義。

### 推奨改善
- **項目バンクに難易度パラメータを事前付与**（NGSL頻度順位＋CEFRから初期推定、運用中の正答率で更新）。
- 停止規準を「SEM ≤ 0.3 もしくは 20問上限」のハイブリッド化。
- 自己申告 CEFR をベイズ事前分布として使い初期効率を上げる。
- 出典: [Wikipedia CAT](https://en.wikipedia.org/wiki/Computerized_adaptive_testing) / [SLUPE 事例](https://www.researchgate.net/publication/324314840)

---

## 4. 内発的動機 vs 外発的動機（ストリーク/バッジ）

### エビデンス
Self-Determination Theory のメタ分析（Deci, Koestner & Ryan, 1999）では、有形外発報酬は **内発的動機を平均 d = -0.36 で低下**させる（undermining effect）。一方 Duolingo の実運用データは、ストリーク7日継続ユーザーが長期継続率 3.6 倍、Streak Freeze 導入で離脱 21% 低下。Hamari et al. (2014) は「ゲーミフィケーションは短期効果は一貫、長期効果は文脈依存」と結論。鍵は **autonomy / competence / relatedness** の3欲求充足で、ストリークも「自分で目標設定」と組み合わせれば内発化しやすい。

### 設計への適用度: ○
「リカバリーチケット（罪悪感軽減）」は SDT の autonomy 支持と整合（Duolingo Streak Freeze 同型）。ただしストリーク以外の動機設計（mastery feedback, progress visualization）が薄い。

### 推奨改善
- ストリークを **唯一の継続指標にしない**。週次レポートで「習得語数」「正答率の伸び」など competence feedback を主役に。
- 1日の学習量を**ユーザー自身が設定可能**にし autonomy 確保（5/10/15分）。
- ネットマップの「点灯」（Phase3）は mastery 可視化として極めて有効、優先度を上げる価値あり。
- 出典: [Duolingo Habit Research](https://blog.duolingo.com/how-duolingo-streak-builds-habit/) / [Trophy: Duolingo Case 2026](https://trophy.so/blog/duolingo-gamification-case-study)

---

## 5. Interleaving / Spacing / Elaboration

### エビデンス
- **Spacing**: Nakata & Elgort (2021) は L2 文脈学習で spacing が明示的語彙知識を促進。Nakata (2023) は relearning 条件下でも spacing の正味効果は維持。
- **Interleaving**: Libersky et al. (2025) は L2 語彙で interleaving が blocking を上回ることを再現。ただし Hwang et al. (2025) は **低習熟層では初期 blocked practice が必要**（desirable difficulty の限度）。
- **Elaboration**: 意味処理深度（Craik & Lockhart 1972）、例文・関連語ネットワーク化は意味記憶定着に寄与。

### 設計への適用度
- Spacing: ◎ — SRS で自動実装
- Interleaving: △ — Curriculum Agent の出題プランで明示的に interleave されているか不明
- Elaboration: ◎ — 例文最低3つ、ネットマップ関連語、ツールチップ訳が深い処理を誘導

### 推奨改善
- Curriculum Agent のプロンプトに **「異なる意味カテゴリ/品詞/CEFR を混在させる」** 制約を追加し interleaving を明示化。
- CEFR A1-A2 層は **新出語は初回 blocked → 2回目以降 interleaved** のハイブリッド方針。
- ネットマップの近傍語提示時、**意味的に近すぎる語の同時出題は避ける**（Tinkham 1997, Nakata 2015: 同義語同時学習は効率を 30〜50% 低下）。
- 出典: [Libersky et al. 2025](https://journals.sagepub.com/doi/10.1177/02676583251338768) / [Nakata & Elgort 2021](https://journals.sagepub.com/doi/10.1177/0267658320927764) / [Hwang et al. 2025](https://onlinelibrary.wiley.com/doi/10.1111/lang.12659)

---

## 総合評価

| 項目 | 適用度 |
|---|---|
| 1. SRS (SM-2) | ○ |
| 2. Testing Effect | ◎ |
| 3. CAT プレースメント | △ |
| 4. 動機設計 | ○ |
| 5. Interleaving/Spacing/Elaboration | ○（Spacing/Elaboration ◎ / Interleaving △） |

**総合: ○+（学習科学的に妥当な土台）**。最大の伸びしろは (a) FSRS への将来移行、(b) CAT の IRT 化、(c) Curriculum Agent への interleaving 明示制約、(d) semantic interference 回避ロジック、(e) 産出型タスク混在による retrieval 強度向上 — Phase2/3 の優先課題として組み込むことを推奨。
