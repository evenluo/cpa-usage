#!/usr/bin/env sh
set -eu

db_path="${1:-data/app.db}"

if ! command -v sqlite3 >/dev/null 2>&1; then
  echo "sqlite3 is required" >&2
  exit 127
fi

if [ ! -f "$db_path" ]; then
  echo "SQLite database not found: $db_path" >&2
  exit 1
fi

has_table="$(sqlite3 "$db_path" "SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'model_price_settings';")"
if [ "$has_table" != "1" ]; then
  echo "model_price_settings table is missing in $db_path" >&2
  exit 1
fi

sqlite3 "$db_path" <<'SQL'
BEGIN;

-- OpenAI API pricing, Standard tier, short context, USD per 1M tokens.
-- Sources verified on 2026-05-17:
-- - https://openai.com/api/pricing/
-- - https://developers.openai.com/api/docs/pricing
INSERT INTO model_price_settings (
  model,
  prompt_price_per1_m,
  completion_price_per1_m,
  cache_price_per1_m,
  created_at,
  updated_at
)
VALUES
  ('gpt-5.5', 5.00, 30.00, 0.50, datetime('now'), datetime('now')),
  ('gpt-5.4', 2.50, 15.00, 0.25, datetime('now'), datetime('now')),
  ('gpt-5.4-mini', 0.75, 4.50, 0.075, datetime('now'), datetime('now')),
  ('gpt-5.4-nano', 0.20, 1.25, 0.02, datetime('now'), datetime('now')),
  ('gpt-5.3-codex', 1.75, 14.00, 0.175, datetime('now'), datetime('now'))
ON CONFLICT(model) DO UPDATE SET
  prompt_price_per1_m = excluded.prompt_price_per1_m,
  completion_price_per1_m = excluded.completion_price_per1_m,
  cache_price_per1_m = excluded.cache_price_per1_m,
  updated_at = datetime('now');

COMMIT;
SQL

sqlite3 "$db_path" <<'SQL'
.headers on
.mode column
SELECT
  model,
  prompt_price_per1_m,
  completion_price_per1_m,
  cache_price_per1_m
FROM model_price_settings
WHERE model IN ('gpt-5.5', 'gpt-5.4', 'gpt-5.4-mini', 'gpt-5.4-nano', 'gpt-5.3-codex')
ORDER BY model;
SQL
