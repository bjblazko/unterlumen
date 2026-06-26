# ADR-0021: Database Schema Migration Strategy

*Last modified: 2026-06-26*

## Status
Accepted

## Context
Unterlumen stores per-library data in SQLite databases at `{root}/libraries/{id}/library.db`. As the application evolves, the schema needs to change (new columns, new indexes, data backfills). Existing installations must self-migrate without data loss and without manual intervention.

## Decision
All schema migrations are implemented as **idempotent SQL statements** appended directly to the `openDB()` function in `src/internal/library/store.go`. The function is called once per library on first access and applies all migrations in sequence.

### Rules for Adding a Migration
1. Add a Go comment `// Migration: <short description>` before the SQL
2. Use `db.Exec(...)` — errors are intentionally ignored (idempotency means re-running is safe)
3. Use idempotent SQL patterns:
   - `CREATE INDEX IF NOT EXISTS ...`
   - `ALTER TABLE ... ADD COLUMN ...` (SQLite silently ignores duplicate column errors)
   - `UPDATE ... WHERE <column> IS NULL` for backfills (only affects un-migrated rows)
   - `CREATE TABLE IF NOT EXISTS ...` for new tables
4. Never use `DROP`, `RENAME`, or destructive DDL — these cannot be made safely idempotent

### New Library-Level Properties
New key/value data (e.g. `last_new_photos`, `sort_order`) can be added as `library_props` entries without any schema migration — the `library_props` table already exists and uses open-ended keys.

### When Versioning Becomes Necessary
This approach is sufficient as long as all migrations are idempotent. If a future migration cannot be made idempotent (e.g. a destructive rename, a data transformation that must run exactly once), introduce a `schema_version` key in `library_props` at that time and gate migrations on the version number.

## Consequences
**Positive:**
- Zero-configuration upgrades: existing installations self-migrate on first start
- No migration files to manage; migrations are co-located with the schema
- Simple to audit: all migrations are in one function

**Negative:**
- No explicit version tracking — cannot easily report "which migrations have run"
- The `openDB()` function grows over time as migrations accumulate
- Non-idempotent migrations require a more complex strategy (deferred to when needed)
