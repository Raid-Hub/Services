# Pantheon 2.0 production deploy plan

PR [#46](https://github.com/Raid-Hub/Services/pull/46) ships **schema and service code only** (migration `010_pantheon_2_activity.sql` recreates the pantheon version leaderboard MV for `activity_id IN (101, 102)`). Definition rows and hash mappings are applied manually in production using the steps below.

## Deploy order

1. Merge and deploy Services PR #46 (code + schema migration).
2. Run `make migrate-postgres` (applies migration `010` — MV recreate only).
3. Run the **data SQL** below against production Postgres.
4. Refresh the materialized view (see step 5).
5. Deploy API / Website if required for Pantheon 2.0 routing (separate repos).

## Data SQL (production)

Run in a transaction where possible. Idempotent `ON CONFLICT` / `WHERE` clauses support re-runs.

```sql
BEGIN;

-- 1. Sunset historical The Pantheon (activity 101)
UPDATE "definitions"."activity_definition"
SET "is_sunset" = true
WHERE "id" = 101;

-- 2. Add permanent Pantheon (activity 102)
INSERT INTO "definitions"."activity_definition" (
    "id", "name", "is_sunset", "is_raid", "path", "release_date", "contest_end", "week_one_end", "milestone_hash", "splash_path"
) VALUES (
    102, 'Pantheon', false, false, 'pantheon', '2026-06-09T17:00:00Z', NULL, NULL, NULL, 'pantheon'
) ON CONFLICT ("id") DO UPDATE SET
    "name" = EXCLUDED."name",
    "path" = EXCLUDED."path",
    "release_date" = EXCLUDED."release_date",
    "splash_path" = EXCLUDED."splash_path";

-- 3. Pantheon 2.0 versions (Calus, Morgeth, Gauntlet placeholder)
INSERT INTO "definitions"."version_definition" (
    "id", "name", "associated_activity_id", "path", "is_challenge_mode"
) VALUES
    (132, 'Calus Resplendent', 102, 'calus', false),
    (133, 'Morgeth Surpassing', 102, 'morgeth', false),
    (134, 'Pantheon Gauntlet', 102, 'gauntlet', false)
ON CONFLICT ("id") DO UPDATE SET
    "name" = EXCLUDED."name",
    "associated_activity_id" = EXCLUDED."associated_activity_id",
    "path" = EXCLUDED."path";

-- If versions 132/133 already exist under activity 101, re-associate them:
UPDATE "definitions"."version_definition"
SET "associated_activity_id" = 102
WHERE "id" IN (132, 133) AND "associated_activity_id" = 101;

-- 4. Bungie activity hash mappings
UPDATE "definitions"."activity_version"
SET "activity_id" = 102
WHERE "hash" IN (2530656885, 1516551982);

INSERT INTO "definitions"."activity_version" (
    "hash", "activity_id", "version_id", "is_world_first"
) VALUES
    (2530656885, 102, 132, false),
    (1516551982, 102, 133, false)
ON CONFLICT ("hash") DO UPDATE SET
    "activity_id" = EXCLUDED."activity_id",
    "version_id" = EXCLUDED."version_id";

-- 5. Gauntlet placeholder — replace hash 0 when Bungie publishes the activity hash
INSERT INTO "definitions"."activity_version" (
    "hash", "activity_id", "version_id", "is_world_first"
) VALUES (
    0, 102, 134, false
) ON CONFLICT ("hash") DO UPDATE SET
    "activity_id" = EXCLUDED."activity_id",
    "version_id" = EXCLUDED."version_id";

COMMIT;
```

### Gauntlet hash follow-up

When the Bungie gauntlet activity hash is known, update the placeholder row:

```sql
-- Option A: update placeholder in place (only if hash 0 was never used for real instances)
UPDATE "definitions"."activity_version"
SET "hash" = <BUNGIE_GAUNTLET_HASH>
WHERE "hash" = 0 AND "version_id" = 134;

-- Option B: insert real hash and delete placeholder
INSERT INTO "definitions"."activity_version" ("hash", "activity_id", "version_id", "is_world_first")
VALUES (<BUNGIE_GAUNTLET_HASH>, 102, 134, false)
ON CONFLICT ("hash") DO UPDATE SET
    "activity_id" = EXCLUDED."activity_id",
    "version_id" = EXCLUDED."version_id";

DELETE FROM "definitions"."activity_version" WHERE "hash" = 0 AND "version_id" = 134;
```

## Refresh materialized view

After data steps, populate the recreated MV:

```sql
REFRESH MATERIALIZED VIEW "leaderboard"."individual_pantheon_version_leaderboard" WITH DATA;
```

Or via the refresh tool:

```bash
./bin/refresh-view individual_pantheon_version_leaderboard
```

## Local / fresh dev environments

After merging PR #46, developers can either run the production data SQL above against a local database, or temporarily add the equivalent rows to `infrastructure/postgres/seeds/default.json` and run `make seed`. Seed changes are **not** part of the PR; keep them local or apply via this deploy plan.

### Seed reference (for local `make seed`)

| Entity | Values |
|--------|--------|
| Activity 101 | `is_sunset: true` |
| Activity 102 | `Pantheon`, path `pantheon`, release `2026-06-09T17:00:00Z` |
| Version 132 | Calus Resplendent → activity 102, path `calus` |
| Version 133 | Morgeth Surpassing → activity 102, path `morgeth` |
| Version 134 | Pantheon Gauntlet → activity 102, path `gauntlet` |
| Hashes | `2530656885` → 102/132, `1516551982` → 102/133, `0` → 102/134 (placeholder) |
