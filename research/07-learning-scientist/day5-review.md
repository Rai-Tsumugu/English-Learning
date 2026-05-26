# Day5 学習科学レビュー: design.md v0.2

## 1. 目標達成可能性スコア: 7/10

KPI（週6/7継続、週30語、コスト$5）はPhase1構成で達成見込み。retrieval practice・spacing・elaborationの土台が揃うが、CAT精度とinterleavingの明示化が未整備で「地力向上」の上限を抑える。

## 2. 強み3点
- **Testing Effect ◎**: 4択+latency記録+ReviewAgent解説が即時フィードバック付き想起練習に合致（g≈0.5–0.6）。
- **Elaboration ◎**: 例文3つ・ネットマップ近傍語・ツールチップ訳が深い意味処理を誘導。
- **動機設計の autonomy 配慮**: リカバリーチケットがSDTのundermining effectを緩和（Duolingo Streak Freeze同型）。

## 3. 弱み3点
- **SRSがSM-2**: FSRS-5比で同保持率にレビュー20–30%増、ease hell残存。
- **CATの較正欠落**: IRTパラメータ・SEM停止規準・自己申告統合方針が未定義。
- **Interleaving/semantic interference未制御**: Curriculum Agentに混在制約なし、近傍語同時出題で30–50%効率低下リスク（Tinkham, Nakata）。

## 4. 改訂提案3点
1. **attemptsにstability/difficulty/lapses列を先行追加**しPhase2でFSRS-5移行可能に。desired retentionをprofile化。
2. **CurriculumAgentプロンプトに「品詞/意味カテゴリ/CEFR混在」「類義語同時出題禁止」制約を明記**、A1–A2は初回blocked→2回目interleavedのハイブリッド。
3. **CATをIRT化**：項目バンクにa/bパラメータ事前付与、SEM≤0.3 or 20問の二重停止規準、CEFR自己申告をベイズ事前分布として活用。併せて産出型タスク（タイピング/和→英）を一部混在。
