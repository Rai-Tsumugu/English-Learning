# 英語講師視点レポート：1日5分設計の教育学的妥当性

対象設計書: `docs/design.md` (v0.2) / 評価者: 指導歴15年・CEFR準拠カリキュラム設計経験ありの英語講師

---

## 1. 1日5分の学習時間で地力はつくか

**エビデンス**: Beck, Perfetti & McKeown (1982) の濃密語彙介入研究は1日30分×5か月を基準とし、Web学習研究でも「利用分数」が語彙獲得の独立予測因子であることが示されている（PMC8753998）。すなわち時間と成果は概ね線形であり、5分単体では新規導入＋運用の両立は困難。一方、モバイル支援学習研究（PMC10108716）は「短時間×高頻度×自己調整」がdrop-outを抑え累積時間で同等以上の効果を出すことを示す。

**教育学的評価**: 地力＝保持×想起×運用のうち、5分設計は保持と想起に集中投資する設計として合理的。新規導入数を抑え既習語のSRS復習に偏重させれば retention は十分機能する。

**設計への提言**: KPIに「累積学習分数/週」を追加。週1回だけ15分の「拡張セッション（長文・コロケーション）」を許容するハイブリッドにすると運用力の壁を越えやすい。

出典:
- https://pmc.ncbi.nlm.nih.gov/articles/PMC4122318/
- https://pmc.ncbi.nlm.nih.gov/articles/PMC8753998/
- https://www.ncbi.nlm.nih.gov/pmc/articles/PMC10108716/

---

## 2. CEFRレベル別の必要語彙数・到達時間

| CEFR | 語彙(word family) | ゼロから累積時間 |
|---|---|---|
| A1 | ~700 | ~100h |
| A2 | 1,500–2,500 | ~200h |
| B1 | 2,750–3,250 | ~450h |
| B2 | ~4,000 | ~650h |
| C1 | ~8,000 | 700–800h+ |
| C2 | 16,000+ | 1,000h+ |

**評価**: 1日5分=年30時間。A2→B1へ200時間必要なため5分のみでは約7年。設計KPI「週30新語」は野心的だが、Nation基準の「90%テキストカバレッジ」までの距離をユーザーに可視化する必要がある。

**設計への提言**: ダッシュボードに「現在語彙サイズ推定→目標CEFRまでの残語数」を表示し進捗の実感を演出。NGSL/COCA取り込みは適切で、頻度上位2,000語の確実な定着を最優先するロジックをCurriculum Agentに明示すべき。

出典:
- https://www.wgtn.ac.nz/lals/resources/paul-nations-resources/vocabulary-lists/vocabulary-cefr-and-word-family-size/vocabulary-and-the-cefr-docx
- https://www.researchgate.net/publication/312063998_Vocabulary_size_and_the_common_European_framework_of_reference_for_languages

---

## 3. SRS (SM-2) の実証効果

**エビデンス**: SM-2はWozniak自身の長期試験で1年継続・1日41分・10,255項目・保持率92%を達成。Anki利用日数とスペイン語成績の正相関（Seibert Hanson & Brown 2020）、SM-2は非SRS比で200–300%の保持改善との報告。新興FSRSはSM-2比で90日保持で+3.2%程度（誤差範囲内）。

**評価**: SM-2選択は枯れた・実証十分・実装容易の三拍子で妥当。ease_factorとnext_review_atのみで動くシンプルさが個人用localhostに合致。

**設計への提言**: (a) lapse時のease下限を1.3で固定（Anki標準）、(b) 1日上限新規語10–15を導入し雪だるま式破綻を防ぐ、(c) `latency_ms`を用いた自信度自動推定でSM-2のquality値(0-5)を生成し1タップ運用化。

出典:
- https://andymatuschak.org/files/papers/Seibert%20Hanson%20and%20Brown%20-%202020%20-%20Enhancing%20L2%20learning%20through%20a%20mobile%20assisted%20sp.pdf
- https://tegaru.app/en/blog/sm2-algorithm-explained

---

## 4. 単語ネットマップ（意味ネットワーク）学習の有効性

**エビデンス**: Tinkham (1993, 1997)、Waring (1997) は意味的に近い語を同時導入すると干渉が生じ学習試行が約50%増と報告。後続研究（Frontiers 2022 / PMC9556891）でも再現。一方、既習語のネットワーク化（精緻化リハーサル）は長期保持を促進する。

**評価**: 「導入時に近接語を同時提示する」設計は危険。設計書の「関連語＝コサイン類似度上位」をそのまま新規導入に使うとTinkham効果に抵触する。

**設計への提言**: ネットマップは**復習・可視化フェーズ専用**とし、新規導入時は意味的・音韻的に離れた語を選ぶ distinctiveness optimizer をCurriculum Agentに組み込む。Phase3の可視化（習得済み点灯）は精緻化学習として有効。

出典:
- https://www.cambridge.org/core/journals/studies-in-second-language-acquisition/article/effects-of-massing-and-spacing-on-the-learning-of-semantically-related-and-unrelated-words/F58BA8D70385603B9C42E408BFCB8A10
- https://www.frontiersin.org/journals/psychology/articles/10.3389/fpsyg.2022.997951/full

---

## 5. 日本人学習者特有の弱点への対応

**エビデンス**: 日本人EFL学習者の主要弱点は (a) 音韻的L1干渉によるaural decoding誤り、(b) コロケーション知識不足、(c) 不一致(incongruent)コロケーションでの反応遅延（Yamashita & Jiang 2010; deGruyter 2022）。

**評価**: 現設計はテキスト中心でリスニング・コロケーション・語法への手当が薄い。Phase2の長文読解だけでは日本人学習者の頭打ちポイントを超えにくい。

**設計への提言**:
1. **コロケーション最優先**: Generatorに「ターゲット語＋高頻度コロケート2つ」を必須出力させる。`examples` に `collocation` カラム追加。
2. **TTS最低限導入**: OpenAI TTSは安価。1日1文の音声シャドウイング枠で音韻L1干渉に対処（残課題のTTSは費用対効果◎）。
3. **L1干渉アラート**: 和製英語/不一致コロケーション辞書を保持しヒット時に注意喚起。
4. **ディクテーション**: Phase2で5分中1分をミニディクテーションに割当。

出典:
- https://www.degruyterbrill.com/document/doi/10.1515/iral-2020-0050/html
- https://onlinelibrary.wiley.com/doi/abs/10.5054/tq.2010.235998
- https://www.degruyterbrill.com/document/doi/10.1515/iral-2022-0234/html

---

## 総合所見

「1日5分」は**保持装置としては科学的に妥当**だが、**運用力獲得には不足**。SRS(SM-2)とNGSLベース選定は強固な土台。一方、(1)意味ネット導入時の干渉、(2)コロケーション・音韻面の日本人弱点、(3)CEFR進捗の可視化、の3点は設計修正が必要。週1拡張セッション＋TTS＋コロケーション必須化を加えれば、年単位で確実にB1→B2に押し上げる設計に昇格できる。
