# OSS / 公開データセット棚卸し（英単語ネットマップ事前構築）

対象: `docs/design.md` §4.2「単語ネットマップ」(NGSL/COCA取り込み、CEFR付与、例文最低3、text-embedding-3-smallで埋め込み、sqlite-vec近傍探索) の事前データ構築用ソース選定。個人用 (`127.0.0.1` バインド、非商用) だが、後日公開・配布を見越して **ライセンス遵守と出典明記** を必須要件とする。

## 1. データセット別 比較表

### 1.1 CEFR付き語彙リスト

| 名前 | 規模 | ライセンス | 入手URL | 推奨用途 |
|---|---|---|---|---|
| **NGSL** | 2,809 lemma (一般英語92%カバー) | **CC BY-SA 4.0** | https://www.newgeneralservicelist.com/ | `words` テーブル主軸。EVPマッピング済(97.2%) |
| **English Vocabulary Profile (EVP)** | 約7,000 headword | Cambridge独自（閲覧無料・**再配布不可**） | https://englishprofile.org/ | CEFR確認の参照のみ（DB組込NG） |
| **Octanove C1/C2** | C1/C2拡張 | **CC BY-SA 4.0** | https://github.com/openlanguageprofiles/olp-en-cefrj | 上位レベル補完 |
| **CEFR-J Wordlist** | 約7,800語 | CC BY-SA 4.0 | https://github.com/openlanguageprofiles/olp-en-cefrj | 日本人学習者向けCEFR、NGSL補完に最適 |
| **Oxford 3000 / 5000** | 3,000 / 5,000 | OUP著作権（**DB組込NG**） | https://www.oxfordlearnersdictionaries.com/wordlists/oxford3000-5000 | 参照比較のみ |
| **COCA top 5000** | 5,000 | **再配布不可** | https://www.wordfrequency.info/ | 頻度スコア付与（個人利用のみ） |

### 1.2 例文コーパス

| 名前 | 規模 | ライセンス | 入手URL | 推奨用途 |
|---|---|---|---|---|
| **Tatoeba** | 1,200万文超(400言語) | **CC BY 2.0 FR**（一部CC0） | https://tatoeba.org/en/downloads | `examples.text` のシード。EN-JA対あり |
| **Tanaka Corpus** | 約15万 EN-JA対 | **Public Domain** | https://www.edrdg.org/wiki/index.php/Tanaka_Corpus | `examples.ja` の即戦力（Tatoebaに統合済） |
| **OPUS (OpenSubtitles)** | 数百万文 EN-JA | サブコーパス依存（**非商用主体**） | https://opus.nlpl.eu/ | 口語例文、要ライセンス精査 |
| **Project Gutenberg** | 7万書籍 | Public Domain (US) | https://www.gutenberg.org/ | Phase2 長文読解素材 |
| **JParaCrawl v3.0** | 2,100万 EN-JA対 | NTT独自（**非商用・研究目的**） | https://www.kecl.ntt.co.jp/icl/lirg/jparacrawl/ | Phase2対訳補強、localhost限定 |

### 1.3 単語埋め込み比較

| 名前 | 次元 | ライセンス | URL | 推奨用途 |
|---|---|---|---|---|
| **GloVe (Stanford)** | 50–300 | **Public Domain (PDDL)** | https://nlp.stanford.edu/projects/glove/ | OpenAIコスト超過時のオフラインfallback |
| **fastText (Meta)** | 300 | **CC BY-SA 3.0** | https://fasttext.cc/ | サブワード対応、未登録語に強い |
| **OpenAI text-embedding-3-small** | 1,536 | 商用OK (API規約) | https://platform.openai.com/docs/guides/embeddings | **採用済**。$0.02/1M tokens |

## 2. 推奨スタック構成図

```
[NGSL CSV (CC BY-SA 4.0)] ─┐
[CEFR-J Wordlist (CC BY-SA)] ┼─→ words(lemma, cefr, difficulty, source)
[Octanove C1/C2 (CC BY-SA)] ─┘
                              │
[Tatoeba EN-JA (CC BY 2.0 FR)] ┐
[Tanaka Corpus (PD)] ──────────┼─→ examples(text, ja, cefr, source, attribution)
不足分: GeneratorAgent + Review ┘
                              │
[OpenAI text-embedding-3-small] (~$0.30 for 5k語一括)
                              │
                              ▼
                        sqlite-vec word_vec → 関連語近傍探索
```

CC BY-SA 4.0 のSA伝播を避けたい場合は、語彙データを別SQLite (`vocab.db`) に分離し `LICENSES/` ディレクトリで個別管理。

## 3. 推奨採用組合せ（個人利用・非商用前提）

- **語彙コア**: NGSL + CEFR-J + Octanove C1/C2（全てCC BY-SA 4.0）
- **例文コア**: Tatoeba EN-JA + Tanaka Corpus（CC BY / PD）→ 不足はOpenAI生成 + Review
- **埋め込み**: OpenAI text-embedding-3-small（初回バッチ $1未満）
- **長文 (Phase2)**: Project Gutenberg。OPUS/JParaCrawlはlocalhost利用限定
- **避けるべき**: Oxford 3000/5000（DB組込NG）、COCA 20k+（再配布禁止）、EVP直接コピー

## 4. 実装メモ（設計書への提案）

1. `data/sources/` 配下にデータセットを展開し `LICENSE.txt` 同梱
2. **§5 スキーマ拡張**: `words.source TEXT`、`examples.source TEXT`、`examples.attribution TEXT` を追加
3. NGSL CSV→SQLiteは Go `encoding/csv`。lemma正規化は `golang.org/x/text/cases` で lowercase + NFKC
4. ライセンス分離: 語彙DB (`vocab.db`, CC BY-SA) と運用DB (`app.db`) を分離

## 出典

- NGSL: https://www.newgeneralservicelist.com/
- EVP: https://englishprofile.org/
- CEFR-J/Octanove: https://github.com/openlanguageprofiles/olp-en-cefrj
- Oxford 3000/5000: https://www.oxfordlearnersdictionaries.com/wordlists/oxford3000-5000
- COCA: https://www.wordfrequency.info/
- Tatoeba: https://tatoeba.org/en/downloads
- Tanaka Corpus: https://www.edrdg.org/wiki/index.php/Tanaka_Corpus
- OPUS: https://opus.nlpl.eu/
- JParaCrawl: https://www.kecl.ntt.co.jp/icl/lirg/jparacrawl/
- Project Gutenberg: https://www.gutenberg.org/
- GloVe: https://nlp.stanford.edu/projects/glove/
- fastText: https://fasttext.cc/
- OpenAI Embeddings: https://platform.openai.com/docs/guides/embeddings
