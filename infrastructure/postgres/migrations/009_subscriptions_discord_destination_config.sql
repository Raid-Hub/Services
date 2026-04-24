-- Split channel-specific fields out of subscriptions.destination: Discord (guild/channel/webhook
-- ids) and http_callback (callback URL). Legacy discord_webhook rows cannot be migrated (URL
-- only), so drop them and their rules (CASCADE).

DELETE FROM "subscriptions"."destination"
WHERE "channel_type" = 'discord_webhook';

CREATE TABLE "subscriptions"."discord_destination_config" (
    "destination_id" BIGINT PRIMARY KEY,
    "guild_id" TEXT NOT NULL,
    "channel_id" TEXT NOT NULL,
    "webhook_id" TEXT NOT NULL,
    "webhook_token" TEXT NOT NULL,
    "created_at" TIMESTAMPTZ(3) NOT NULL DEFAULT NOW(),
    "updated_at" TIMESTAMPTZ(3) NOT NULL DEFAULT NOW(),
    CONSTRAINT "discord_destination_config_destination_fkey"
        FOREIGN KEY ("destination_id")
        REFERENCES "subscriptions"."destination" ("id")
        ON DELETE CASCADE
);

CREATE UNIQUE INDEX "idx_discord_destination_config_webhook_id"
    ON "subscriptions"."discord_destination_config" ("webhook_id");

-- One Discord destination per channel (table is new; legacy discord_webhook rows were dropped above).
CREATE UNIQUE INDEX "idx_discord_destination_config_channel_id"
    ON "subscriptions"."discord_destination_config" ("channel_id");

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

DROP INDEX IF EXISTS "idx_destination_channel_type_webhook_url";
ALTER TABLE "subscriptions"."destination" DROP COLUMN IF EXISTS "webhook_url";

-- subscriptions.rule: track last mutation (API / Hermes upserts and deactivations).
-- Idempotent for databases that already include updated_at from a revised 008 apply.

ALTER TABLE "subscriptions"."rule"
    ADD COLUMN IF NOT EXISTS "updated_at" TIMESTAMPTZ(3) NOT NULL DEFAULT NOW();

GRANT SELECT ON "subscriptions"."discord_destination_config" TO readonly;
GRANT SELECT ON "subscriptions"."http_callback_destination_config" TO readonly;
