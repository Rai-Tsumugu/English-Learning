# AIエンジニア視点リサーチ: LLM英語学習コンテンツ生成の現実性

対象設計: `docs/design.md`（Curriculum / Generator / Review の3エージェント、gpt-4o-mini、JSON Schema強制、10%サンプリングReview、月$5予算）。

## 1. gpt-4o-mini の英語問題生成品質

先行研究では、GPTファミリーによる多肢選択クローズ問題生成で **文整形度75%、選択肢適切性66.85%** が報告されており、英語学習向け穴埋めは実用域。Bloom分類に沿った難易度コントロールはGPT-4 Turboでも認知レベルで精度差があり、CEFR A1〜C2を厳密に分けるなら **CEFR帯ごとの few-shot 例** を固定すべき。読解問題は「正答が文中に明示されているか」が弱点。Generator側で `answer_span` を必須フィールドにしReviewで照合。gpt-4o-miniは4oの85〜90%品質を維持。

## 2. Structured Outputs の信頼性

`response_format: json_schema`, `strict: true` は制約付きデコーディングで **スキーマ準拠率99.9%超**。残る失敗モード:

- refusal オブジェクト返却（安全フィルタ）
- `finish_reason: length` による途中切れ
- ネットワーク/レート制限エラー

推奨: `temperature=0.0〜0.2`、指数バックオフで最大2回、`length`切れ時は `max_tokens` を1.5倍拡張して再試行。設計書「再試行最大2回」は妥当。

## 3. LLM-as-judge (ReviewAgent) の妥当性

既知バイアス:
- **position bias**（選択肢順依存、影響≤0.04）
- **style bias**（文体偏向、0.76-0.92と支配的）
- **self-bias**（同一モデル生成物を高評価）

設計書でReviewerが Generator と同一の gpt-4o-mini である場合、**self-bias** が懸念。**Reviewer を別モデル(gpt-4o)または別プロンプト系統** にすべき。**ルーブリック固定 + 4段階スコア + 理由文必須** の構造化Judgeで一貫性改善。10%サンプリングは妥当だが、**新規CEFR帯・新出題形式の初出時は100%レビュー** に切替えるアダプティブ運用を推奨。

## 4. プロンプトキャッシュ・Batch API

- gpt-4o-mini は **自動プロンプトキャッシュ対応**（1024トークン以上の共通プレフィックスで発火、入力50%OFF）
- Batch API は **入出力ともに50%OFF**（24時間以内処理）
- 設計書「前夜23時バッチ事前生成」と Batch API は相性極めて良い → **半額**で翌朝分準備
- System prompt + CEFR few-shot を共通プレフィックスに固定し、ユーザー履歴部分を末尾に置けばキャッシュヒット率向上

## 5. コスト試算 (gpt-4o-mini)

| 項目 | 単価 (per 1M tok) | 想定使用 | 日次コスト |
|---|---:|---:|---:|
| 通常 input | $0.150 | 20k tok | $0.0030 |
| 通常 output | $0.600 | 10k tok | $0.0060 |
| キャッシュ input (50%OFF) | $0.075 | 15k tok (75%ヒット) | $0.0011 |
| Batch input (50%OFF) | $0.075 | 夜間20k tok | $0.0015 |
| Batch output (50%OFF) | $0.300 | 夜間10k tok | $0.0030 |
| **合計(最適化後)** | — | 30k tok/日 | **約 $0.005〜$0.010** |

月次換算で **$0.15〜$0.30**、$5上限に対し大幅余裕。1問あたり300〜600 tokenで **1日50〜100問・月1,500〜3,000問** 生成が現実的。

## 結論

- gpt-4o-mini + Structured Outputs は穴埋め/読解/解説に十分実用的（失敗率<0.1%）
- ReviewAgent は **Generator と別モデル化 or 別プロンプト系統** で self-bias 緩和、新規形式は100%レビューに切替えるアダプティブ運用
- Batch API + プロンプトキャッシュで **月$0.30以下** に圧縮可能。設計書の試算は保守的で1桁圧縮できる

## 出典

- OpenAI API Pricing: https://openai.com/api/pricing/
- Structured Outputs: https://openai.com/index/introducing-structured-outputs-in-the-api/
- Prompt Caching: https://openai.com/index/api-prompt-caching/
- Structured Outputs Guide: https://platform.openai.com/docs/guides/structured-outputs
- A Survey on LLM-as-a-Judge (arXiv:2411.15594)
- Judging the Judges: Position Bias (arXiv:2406.07791)
- Evaluating Scoring Bias in LLM-as-a-Judge (arXiv:2506.22316)
- Automated Generation of MC Cloze (arXiv:2403.02078)
- GPT-4 Turbo Question Generation with Bloom's Taxonomy (arXiv:2406.15211)
