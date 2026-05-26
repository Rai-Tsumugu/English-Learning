# Day5 レビュー（OSSデータアナリスト視点）

## 1. 目標達成可能性スコア: 7/10

NGSL+CEFR-J+Octanove+Tatoeba/Tanakaで語彙コア・例文シードは無償・適法に揃い、Phase1 KPI（週6/7継続、週30語、月$5）は技術的に到達可能。ただしライセンス管理とCOCA/EVP/Oxford非組込の運用規律が設計書に未反映で減点。

## 2. 強み
- gpt-4o-mini + sqlite-vec + キャッシュ60%目標でコスト$5/月の現実性が高い
- SRS+JSON Schema強制+Reviewサンプリングで品質と継続性の両立設計が堅実
- Phase分割が明快でPhase1の2週間スコープが妥当

## 3. 弱み
- §4.2が「NGSL/COCA取り込み」と記載しCOCA再配布不可リスクを内包
- `words/examples`に`source`/`attribution`列がなく出典追跡不能
- 例文最低3保証のシード（Tatoeba/Tanaka）と不足時のGenerator委譲ロジックが未定義

## 4. 改訂提案
1. §4.2の「COCA」を「CEFR-J + Octanove C1/C2」に置換、COCAは頻度スコア参照のみと明記
2. §5に`words.source`, `examples.source`, `examples.attribution`列追加、CC BY-SA伝播回避のため`vocab.db`/`app.db`分離
3. §2/§10に`data/sources/LICENSES/`同梱と起動時ライセンス整合性チェックを追加
