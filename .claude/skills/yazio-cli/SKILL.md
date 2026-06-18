---
name: yazio-cli
description: Use when an agent needs to read, write, export, or pipe YAZIO nutrition data via the yazio CLI. Covers both directions — reading (daily summaries, diary entries, product search, multi-day backfills with the today-UTC default, bulk CSV exports, JSON for piping into other APIs) and writing (logging meals from chat/agentic flows via product search → add, removing entries, backdated logging) — plus the auth/token lifecycle and idempotency caveats for replay-safe writes. Trigger whenever a task mentions YAZIO, food logging, calorie tracking, diary backfills, "log this meal for me", "pull my YAZIO data into X", or any agentic nutrition workflow.
---

# yazio-cli

A scriptable CLI for the unofficial YAZIO API. This skill is how an agent drives it end-to-end — both reading data out (summaries, diaries, exports) and writing data in (logging meals, removing entries) from scripts or chat-driven flows.

## Mental model

- **One binary, one config.** `yazio` reads `~/Library/Application Support/yazio-go-cli/config.yaml` (macOS) or `$XDG_CONFIG_HOME/yazio-go-cli/config.yaml` (Linux). Override per-invocation with `--config <path>` or `$YAZIO_CONFIG`.
- **Date-centric.** Every read command (`summary`, `consumed`, `export diary`) is scoped to one calendar date in **UTC**. Format is always `YYYY-MM-DD`. If you omit the date, the CLI uses **today (UTC)** — this is the single biggest footgun for backfills, see below.
- **Two output modes.** `--output table` (default, human) or `--output json` (machine-readable, stable shape). For any automation, **always pass `--output json`** or use `export diary` which emits CSV.
- **Token lifecycle is automatic.** After `auth login` the token sits in the config file at mode `0600`. Refresh happens transparently before each call if expired. You only re-login if the refresh token is gone or revoked.

## Preflight

```bash
yazio auth status     # exits 0 either way; prints "not logged in" or "status: valid|expired"
```

If not logged in:

```bash
export YAZIO_EMAIL='user@example.com'
export YAZIO_PASSWORD='…'
yazio auth login      # writes token to config
```

Never persist credentials in scripts; use env vars or read from a secrets manager and `unset` after.

## Command map

| Command | Purpose | Default scope |
|---|---|---|
| `yazio user profile` | Account/profile (units, goal type, premium) | n/a |
| `yazio summary [DATE]` | Calorie/macros/water/steps totals + goals | today UTC |
| `yazio consumed [DATE]` | Diary line items (entry_id, meal, product_id, amount) | today UTC |
| `yazio export diary [DATE]` | CSV export of diary entries (incl. recipes + simple products) | today UTC |
| `yazio export diary --from A --to B --file F` | Inclusive CSV range, max **366 days** | range |
| `yazio search QUERY` | Product lookup → product_id needed for `add` | n/a |
| `yazio add --product-id … --meal … --amount … [--date …]` | Append diary entry; mints a UUID | today UTC |
| `yazio remove <entry-id>` | Delete a diary entry by UUID | n/a |

## Backfilling — the core pattern

The CLI defaults to today on every read, so naïve scripts only capture one day. Use these patterns:

### 1. Diary CSV over a date range (one CLI call)

```bash
yazio export diary --from 2026-01-01 --to 2026-03-31 --file ~/data/yazio-q1.csv
```

- Inclusive on both ends.
- Hard limit: **366 days**. For longer windows, slice into chunks (see below).
- CSV header: `date,category,meal,entry_id,type,product_id,name,producer,amount,serving,serving_quantity,raw_json`.
- `category` is one of `product`, `recipe_portion`, `simple_product`. For the latter two, fields YAZIO doesn't expose as columns land in `raw_json` — don't drop that column when downstream-importing.

### 2. JSON summaries over a date range (shell loop)

`summary` and `consumed` have **no** `--from/--to`. Loop them:

```bash
# bash/zsh; macOS uses `date -j -v` instead of `date -d`.
start=2026-01-01
end=2026-03-31
mkdir -p ~/data/yazio-summaries
cursor="$start"
while [[ "$cursor" < "$end" || "$cursor" == "$end" ]]; do
  yazio --output json summary "$cursor" > "$HOME/data/yazio-summaries/$cursor.json"
  cursor=$(date -j -f %Y-%m-%d -v +1d "$cursor" +%Y-%m-%d)   # macOS
  # cursor=$(date -I -d "$cursor + 1 day")                    # GNU/Linux
done
```

Throttle if you're pulling many days — the client has a 30s timeout and retries transient GET failures up to 3× with exponential backoff, but YAZIO can still rate-limit. Sleep 200–500 ms between requests for ranges > 30 days.

### 3. Window slicing for > 366 days

```bash
yazio export diary --from 2024-01-01 --to 2024-12-31 --file ~/data/yazio-2024.csv
yazio export diary --from 2025-01-01 --to 2025-12-31 --file ~/data/yazio-2025.csv
```

Concatenate CSVs with `awk 'FNR==1 && NR!=1 {next} {print}' *.csv > merged.csv` to keep one header.

## Piping into other tools / APIs

Always emit JSON, then transform with `jq`:

```bash
# Total kcal for a day
yazio --output json summary 2026-06-02 \
  | jq '[.meals.breakfast.nutrients.energy, .meals.lunch.nutrients.energy,
         .meals.dinner.nutrients.energy, .meals.snack.nutrients.energy] | add'

# Flatten today's diary into one row per item for an external HTTP sink
yazio --output json consumed 2026-06-02 \
  | jq -c '.products[] | {entry: .id, meal: .daytime, product: .product_id, grams: .amount}' \
  | while read -r row; do
      curl -fsS -XPOST -H "Content-Type: application/json" -d "$row" \
        "$SINK_BASE_URL/yazio-entries"
    done
```

CSV → other systems: stream `export diary` without `--file` and pipe into `psql`, `duckdb`, `sqlite3 .import`, or `csvkit` directly.

## Adding entries (write path)

`yazio add` requires a `--product-id` from `yazio search`. The CLI mints a fresh UUID per `add`, so re-running the same command **creates duplicates** — there is no idempotency key. For replay-safe imports:

1. Search → resolve product_id once, cache in your dataset.
2. On import, query `consumed --output json` for the target date and dedupe on `(date, meal, product_id, amount)` before calling `add`.
3. If you must retry a failed `add`, immediately `consumed --output json` on that date to find the entry that did or did not land.

`--date` accepts `YYYY-MM-DD`, so backdated logging is fine. `--meal` is one of `breakfast`, `lunch`, `dinner`, `snack`.

## Removing entries

```bash
yazio remove <entry-id>
```

The entry ID is the `id` / `entry_id` column from `consumed --output json` or `export diary` CSV. Validates that the argument is a UUID before sending.

## Failure modes to know

| Symptom | Likely cause | Fix |
|---|---|---|
| `not logged in` | No token in config | `yazio auth login` |
| `stored token expired and no refresh token is available` | Refresh token revoked | `yazio auth login` again |
| `diary export range cannot exceed 366 days` | `--from`/`--to` span too wide | Slice and concatenate |
| Empty `products` array for a date | Nothing was logged that day | Not an error |
| HTTP 5xx during a long backfill | YAZIO upstream blip | Built-in 3× retry; if still failing, sleep and resume from the failed date |
| `invalid date "…"` | Wrong format | Always `YYYY-MM-DD`, no times, no timezones |

## Verification checklist

Before reporting a backfill complete:

1. **Coverage**: row/file count matches `(to - from + 1)` for daily JSON or sum of CSV rows per day matches `consumed` for spot-checked dates.
2. **No silent zeros**: a day with `0 kcal` and `0 products` in YAZIO is real; don't paper over it. Distinguish from API failures by checking `auth status` and re-running the single date.
3. **No duplicate writes**: after any `add`, run `consumed` for that date and confirm exactly one matching entry.
4. **Token still valid**: `yazio auth status` reports `valid` at the end of the run — confirms the refresh path worked.

## What NOT to do

- Don't parse the human `table` output. It's whitespace-aligned and meant for terminals — use `--output json` or `export diary`.
- Don't loop `export diary` per-day when a range works in one call; the range form is one CLI invocation but multiple API calls anyway, and it produces a single deduplicated CSV.
- Don't try to fetch a date in the local timezone and assume YAZIO agrees. Everything is UTC; if the user lives in `Europe/Berlin` and logged a meal at 23:30 local on June 1, it lands on `2026-06-01` UTC — fetch that date.
- Don't hardcode the config path in scripts; respect `$YAZIO_CONFIG` so multiple accounts work.
