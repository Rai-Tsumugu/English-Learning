# UXリサーチャー報告書: 個人用英語学習アプリの「毎日続ける」設計

## 1. 学習アプリ継続率ベンチマーク

| アプリ / 指標 | D1 | D7 | D30 | 補足 |
|---|---|---|---|---|
| Duolingo（全体） | 約55% | D7までに約50%が脱落 | 5〜40%（セグメント差） | D3以降が崖 |
| Duolingo Streak Masters | — | — | 約40% | 7日継続者は翌日復帰2.4倍 |
| Duolingo Casual | — | — | 約15% | — |
| Duolingo Inactive | — | — | 約5% | — |
| Anki（最適設定） | — | — | 知識保持85〜90% | 1日20枚が長期持続限界、50枚は約2か月で破綻 |
| 教育アプリ業界平均 | 約25〜30% | 約10% | 4〜8% | 通知opt-out率が他カテゴリより高い |

## 2. ストリーク機能の継続寄与エビデンス
- 7日以上のストリーク保持者は翌日復帰率 2.3〜2.4倍、D30維持率は非保持者比 +約40pt。
- 心理機序は **損失回避**。ただし1日途切れた瞬間にall-or-nothing思考で完全離脱する副作用が観測されている。
- Streak Freeze / 週末アミュレット型の救済機構が burnout 離脱を有意に抑制（設計書のリカバリーチケットと合致）。

## 3. 通知/リマインダーの効果と個人プロジェクトでの代替
- Pham & Nguyen (2016, IEEE): 15〜30分遅延通知が即時通知より反応率・セッション時間で有意。
- 教育アプリの通知 opt-out 率 ≈60%、過剰通知は逆効果。
- **個人ローカル代替**:
  1. macOS `launchd`/cron + `osascript -e 'display notification'` で1日1回ambient cue
  2. 前夜バッチ完了をターミナル banner 化
  3. 朝のブラウザ起動時に `localhost:5173` をデフォルトタブ化
  4. Slack/Discord webhook へ自送信（ストリーク残数を投稿）

## 4. 達成感の即時化 / マイクロ報酬
- 即時フィードバックはドーパミン放出を誘発し、マイクロラーニング形式で保持率を25〜80%押し上げ。
- コア4原則: incremental progress / immediate feedback / balanced difficulty / sense of achievement。
- 安価で効果大: 「正解時の即アニメーション+音」「セッション終了画面で本日獲得語のネットマップ点灯」。

## 5. 個人プロジェクトで継続が破綻する典型パターン

| パターン | メカニズム |
|---|---|
| 熱量減衰 | 初動の興奮が2〜4週で消える |
| Scope Creep | ゴール曖昧→進捗感喪失 |
| 自作バイアス (dogfooding fail) | 欠陥に慣れて気付かない、UI改善が止まる |
| 過負荷キュー | SRSレビュー山積みが脅迫感を生む（Anki 50枚問題と同型） |
| All-or-nothing | 1日途切れで放棄 |

## 6. §4.6 への具体的提言（5件）

1. **ストリーク表示を「週7マス埋まり可視化」へ**。週6/7のKPIと整合し、all-or-nothing心理を抑制。
2. **リカバリーチケットは月2枚を自動付与**し、使用時のコピーを「賢く休んだ」というポジティブ表現に。
3. **5分ノルマ達成画面に即時マイクロ報酬**: 本日獲得語をネットマップ上で点灯アニメ＋効果音、SRS「次に会える日」明示。
4. **通知は macOS `launchd` の1日1回ambient cue として同梱**。前夜バッチ完了を翌朝メニューバーに残ストリーク数で表示。
5. **Dogfooding盲点対策＋過負荷サーキットブレーカー**: 週次レポートに「今週UIで詰まった瞬間」自由記述欄。未消化SRSが3日分超で新規語投入を自動停止。

## 出典
- Lenny's Newsletter: How Duolingo reignited user growth
- Gitnux: Duolingo User Statistics 2026
- Just Another PM: The Psychology Behind Duolingo's Streak
- StriveCloud: Duolingo gamification explained
- UX Magazine: Psychology of Hot Streak Game Design
- DocendoCards: 10 Anki Settings That Will Double Your Retention Rate
- Migaku: Spaced Repetition in 2026
- IEEE Xplore: Pham & Nguyen 2016
- Winsome Marketing: Push Notification Strategy for Learning Apps
- Cards Microlearning: Gamification, Engagement & Motivation
- Medium/Bootcamp: Neuroscience & Responsible Gamification
- Pavley: When Dogfooding Fails
- Web Highlights: 5 Reasons Side Projects Never Finish
- Enable3: App Retention Benchmarks 2026
