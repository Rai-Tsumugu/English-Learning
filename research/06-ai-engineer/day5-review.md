# Day5レビュー: AIエンジニア視点

## 目標達成可能性スコア: 8/10

gpt-4o-mini + Structured Outputs で穴埋め/読解生成は実用域。月$5予算はBatch API+キャッシュで1桁余裕あり、技術的実現性は高い。

## 強み

1. **JSON Schema強制 + 再試行2回**: Structured Outputsの99.9%準拠率と整合、現実的設計。
2. **3エージェント分業 + 10%サンプリングReview**: コストと品質のバランスが妥当。
3. **生成キャッシュ(hash key) + 前夜23時バッチ**: Batch API(50%OFF)とプロンプトキャッシュ活用余地が大きく、コスト試算は保守的。

## 弱み

1. **Review self-bias**: Generator/Reviewerが同一gpt-4o-miniだと自己評価バイアス(style bias 0.76-0.92)で品質検出力低下。
2. **CEFR厳密制御の根拠不足**: A1〜C2帯ごとのfew-shot固定方針が未記載で、難易度ブレが起きやすい。
3. **読解の正答根拠検証なし**: 文中明示性チェック(answer_span照合)が設計に欠落。

## 改訂提案

1. **Reviewer別系統化**: gpt-4o or 別プロンプト+ルーブリック4段階+理由必須。新形式初出時は100%レビューに切替えるアダプティブ運用を明記。
2. **Batch API + プロンプトキャッシュを正式採用**: 共通プレフィックス(system+CEFR few-shot)を固定し、月予算を$1以下に再設定。浮いた予算をReview強化へ。
3. **Generator出力に`answer_span`/`cefr_evidence`必須化**: スキーマレベルで根拠提示を強制、Reviewで機械照合可能に。
