#!/usr/bin/env bash
# age 暗号化バックアップスクリプト (T31)
#
# 使い方:
#   ./scripts/backup.sh <age-recipient>
# 例:
#   ./scripts/backup.sh age1xyz...
#
# data/app.db と data/vocab.db を tar.gz にまとめ、
# 指定された age 公開鍵 (recipient) で暗号化して
# backups/YYYYMMDD-HHMMSS.tar.gz.age に保存する。

set -euo pipefail

if [[ $# -lt 1 || -z "${1:-}" ]]; then
  echo "usage: $0 <age-recipient>" >&2
  echo "  例: $0 age1xyz..." >&2
  exit 2
fi

RECIPIENT="$1"

if ! command -v age >/dev/null 2>&1; then
  echo "error: 'age' コマンドが見つかりません。https://github.com/FiloSottile/age からインストールしてください。" >&2
  exit 1
fi

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT_DIR"

mkdir -p backups

TS="$(date -u +%Y%m%d-%H%M%S)"
OUT="backups/${TS}.tar.gz.age"

# data/ 配下に存在するファイルのみ tar に含める
FILES=()
for f in data/app.db data/vocab.db; do
  if [[ -f "$f" ]]; then
    FILES+=("$f")
  fi
done

if [[ ${#FILES[@]} -eq 0 ]]; then
  echo "error: バックアップ対象 (data/app.db, data/vocab.db) が見つかりません。" >&2
  exit 1
fi

tar -czf - "${FILES[@]}" | age -r "$RECIPIENT" -o "$OUT"

echo "backup created: $OUT"
