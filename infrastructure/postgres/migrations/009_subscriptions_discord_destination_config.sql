-- Discord-specific metadata for subscriptions.destination rows.
-- Keeps subscriptions.destination generic while allowing Discord config and policy checks.
--
-- This table is canonical for Discord destination details.

-- Legacy cleanup for environments that still have URL-based destination records.

CREATE TABLE "subscriptions"."discord_destination_config" (
    "destination_id" BIGINT PRIMARY KEY,
    "guild_id" TEXT,
    "channel_id" TEXT,
    "webhook_id" TEXT NOT NULL,
    "webhook_token" TEXT NOT NULL,
    "created_at" TIMESTAMPTZ(3) NOT NULL DEFAULT NOW(),
    "updated_at" TIMESTAMPTZ(3) NOT NULL DEFAULT NOW(),
    CONSTRAINT "discord_destination_config_destination_fkey"
        FOREIGN KEY ("destination_id")
        REFERENCES "subscriptions"."destination" ("id")
        ON DELETE CASCADE
);

-- Backfill from legacy subscriptions.destination.webhook_url rows.
-- Parse and store webhook id/token in plaintext for now.
-- Use a scalar subquery for regexp_match (portable vs. LATERAL on a scalar function).
INSERT INTO "subscriptions"."discord_destination_config" (
    "destination_id",
    "webhook_id",
    "webhook_token"
)
SELECT
    d.id,
    match_row.m[1],
    match_row.m[2]
FROM "subscriptions"."destination" d
CROSS JOIN LATERAL (
    SELECT regexp_match(
        d.webhook_url,
        '^https://discord(?:app)?\.com/api/webhooks/([^/]+)/([^/?#]+)'
    ) AS m
) AS match_row
WHERE d.channel_type = 'discord_webhook'
  AND match_row.m IS NOT NULL
ON CONFLICT ("destination_id")
DO UPDATE SET
    "webhook_id" = EXCLUDED."webhook_id",
    "webhook_token" = EXCLUDED."webhook_token",
    "updated_at" = NOW();

CREATE UNIQUE INDEX "idx_discord_destination_config_webhook_id"
    ON "subscriptions"."discord_destination_config" ("webhook_id");

-- Enforce one Discord destination per channel.
-- Remove duplicate rows that may exist across guilds for the same channel_id.
-- Keep the most recently updated destination for each channel.
WITH ranked AS (
    SELECT
        destination_id,
        channel_id,
        ROW_NUMBER() OVER (
            PARTITION BY channel_id
            ORDER BY updated_at DESC, created_at DESC, destination_id DESC
        ) AS row_num
    FROM "subscriptions"."discord_destination_config"
    WHERE channel_id IS NOT NULL
),
to_delete AS (
    SELECT destination_id
    FROM ranked
    WHERE row_num > 1
)
DELETE FROM "subscriptions"."discord_destination_config" d
USING to_delete td
WHERE d.destination_id = td.destination_id;

CREATE UNIQUE INDEX "idx_discord_destination_config_channel_id"
    ON "subscriptions"."discord_destination_config" ("channel_id")
    WHERE "channel_id" IS NOT NULL;

-- HTTPS JSON callback URL for subscriptions.destination rows (channel_type = http_callback).
-- Canonical URL storage; subscriptions.destination no longer carries webhook_url.

CREATE TABLE "subscriptions"."http_callback_destination_config" (
    "destination_id" BIGINT PRIMARY KEY,
    "callback_url" TEXT NOT NULL,
    "created_at" TIMESTAMPTZ(3) NOT NULL DEFAULT NOW(),
    "updated_at" TIMESTAMPTZ(3) NOT NULL DEFAULT NOW(),
    CONSTRAINT "http_callback_destination_config_destination_fkey"
        FOREIGN KEY ("destination_id")
        REFERENCES "subscriptions"."destination" ("id")
        ON DELETE CASCADE
);

CREATE UNIQUE INDEX "idx_http_callback_destination_config_callback_url"
    ON "subscriptions"."http_callback_destination_config" ("callback_url");

INSERT INTO "subscriptions"."http_callback_destination_config" (
    "destination_id",
    "callback_url"
)
SELECT
    d.id,
    d.webhook_url
FROM "subscriptions"."destination" d
WHERE d.channel_type = 'http_callback'
ON CONFLICT ("destination_id")
DO UPDATE SET
    "callback_url" = EXCLUDED."callback_url",
    "updated_at" = NOW();

DROP INDEX IF EXISTS "idx_destination_channel_type_webhook_url";
ALTER TABLE "subscriptions"."destination" DROP COLUMN IF EXISTS "webhook_url";

GRANT SELECT ON "subscriptions"."discord_destination_config" TO readonly;
GRANT SELECT ON "subscriptions"."http_callback_destination_config" TO readonly;
