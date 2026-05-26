# 英語学習アプリ市場・競合分析レポート（Marketer視点）

## サマリー
- 英語学習アプリ市場はグローバルで2025年$7.36B→2033年$24.39B、CAGR約16%と高成長。日本もTOEIC/受験需要で堅調。
- 主要競合（Duolingo/ELSA/Cake/abceed/スタサプENGLISH）はゲーミフィケーション・AI発音・動画教材で差別化、月額1,000〜3,000円のサブスクが主流。
- 2025-2026のトレンドはLLMによる教材自動生成×適応学習×SRSの融合。「個別最適化」が業界共通テーマ。
- OSS自作セグメントは存在するがニッチ（LinguaCafe, companion, anki-llm等）。完全統合型の「ネットマップ＋エージェント生成」は空白地帯。
- 本設計はB2C競合とは戦わない「個人用・低コスト・自律生成」ポジションで、技術検証＋自己学習の二兎を狙える独自性が強み。

## 1. 市場規模・成長率
- 英語学習市場（広義）: 2026年 $24.0B → 2035年 $147.1B、CAGR 22.5%。
- 言語学習アプリ市場（狭義）: 2024年 $6.34B → 2033年 $24.39B、CAGR 16.15%。
- 日本: 政府が2023年に公立校英語教育に$250M超を投資。TOEIC/IELTS対策アプリ需要が学生層で増加。
- アジア太平洋がスマホ普及と英語需要で最大成長エンジン。

## 2. 主要競合
| アプリ | 強み | 課金 | 想定継続率 |
|---|---|---|---|
| Duolingo | ゲーミフィケーション、ストリーク文化、世界5億DL超 | Free / Super ≈¥1,300/月 | DAU/MAU約25% |
| ELSA Speak | AI発音矯正特化、音素レベル評価 | ¥1,800/月相当 | 発音特化で高エンゲージ |
| Cake | YouTube/ドラマ動画でリスニング | 部分Free、Plus月額制 | 短尺動画で高頻度起動 |
| スタサプENGLISH | 関先生講義動画＋TOEIC対策、日本市場最強 | ¥2,178〜3,278/月 | 受験生中心、3か月集中型 |
| abceed | 市販教材デジタル化＋AI診断 | ¥2,000前後/月 | 問題量で受験層に支持 |

共通点: 短時間×毎日×ストリーク。差異: Duolingo=広く浅く / スタサプ=深い講義 / ELSA=スピーキング / abceed=問題演習。

## 3. パーソナライズAI学習トレンド（2024-2026）
- LLMによる教材オンデマンド生成が研究・実装で急増（ScienceDirect 2025レビュー）。
- 「Adaptive Learning × SRS × LLM」の三位一体が次世代標準。Duolingo Maxが先行（GPT-4ベース）。
- 課題: 学習変数の測定難、理論的指針不足、ハルシネーション。→ JSON Schema強制＋Reviewエージェントは正解パターン。
- 埋め込みベースの「概念グラフ/ネットマップ」可視化は学術寄りで商用実装は少数。

## 4. 「個人用OSS自作」セグメント
- LinguaCafe（多言語リーダー）、companion（GPT+Whisper家庭教師）、anki-llm（Anki自動生成CLI）、doc-to-anki-with-llm 等が存在。
- いずれも「リーダー」「フラッシュカード生成」「会話Bot」の単機能。本設計のように オンボーディング→SRS→生成→Review→ネットマップ を統合した個人用フルスタックは確認できず。
- Anki+LLMプラグイン群は強力だがUIは古典的。React+ネットマップ可視化は差別化余地大。

## 5. 設計差別化評価
**強み**
- 単一ユーザー固定で認証・スケール設計不要 → 開発速度。
- sqlite-vec採用で依存最小、月$5予算は個人OSSとして現実的。
- Curriculum/Generator/Review三段エージェント＋10%サンプリングReviewは商用にも通用する品質設計。
- ネットマップは学習動機を可視化する独自UX（Phase3）。

**弱み・リスク**
- スピーキング/発音評価がスコープ外 → 「英語の地力」のうち産出能力に弱い。
- 単一ユーザー前提のためコミュニティ要素ゼロ、外発的動機づけが弱い（ストリーク頼み）。
- OpenAI依存。$5上限は厳しく、Phase2長文導入でキャッシュヒット率60%未達なら破綻。
- ネットマップUIは未確定で、Phase3まで価値が見えにくい。

## 出典
- https://www.marketgrowthreports.com/market-reports/language-learning-market-114898
- https://www.marketgrowthreports.com/market-reports/english-language-learning-market-103052
- https://straitsresearch.com/report/language-learning-apps-market
- https://www.mordorintelligence.com/industry-reports/online-language-learning-market
- https://reskill.gakken.jp/4367
- https://blog.greeden.me/2025/12/04/
- https://myrtille-aya.com/application/recipy-studysapuri-duolingo-speakbuddy/
- https://810-suru.com/abceed-studysapuri-hikaku/
- https://www.sciencedirect.com/science/article/pii/S2590291125008447
- https://www.sciencedirect.com/science/article/pii/S2666920X25001699
- https://www.clarifai.com/blog/llms-and-ai-trends
- https://github.com/Vuizur/awesome-language-learning
- https://github.com/shakedzy/companion
- https://github.com/raine/anki-llm

## Day5評価へのインプット
- 商用競合と機能で戦う必要なし。「個人用・低コスト・自律生成」のポジショニングを明示すべき（KPIに継続率を据えるのは妥当）。
- スピーキング/発音をスコープ外とする方針は明文化推奨。「読む・語彙・文法」に絞った地力定義をdesign.md §1.1へ追記。
- $5/月予算はキャッシュヒット率60%が生命線。未達時のフォールバック（Reviewスキップ＋静的教材）を§8で再強化。
- ネットマップは差別化の核だがPhase3。Phase1のうちに簡易グラフを出して価値を早期検証すべき。
- 外発的動機が弱い → ストリーク＋週次レポート＋ネットマップ点灯の三位一体UXがKPI 6/7達成の鍵。リカバリーチケットの設計を具体化。
