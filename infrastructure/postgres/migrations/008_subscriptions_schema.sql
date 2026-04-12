-- RaidHub Services - Subscriptions (alert destinations and match rules).
-- Destinations store routing only (channel_type, webhook_url); no display/embed columns.

CREATE SCHEMA IF NOT EXISTS "subscriptions";

-- Outbound channel: Discord webhook URL, or HTTPS URL for JSON callbacks (dto.Instance body). Primary key is the stable destination id.
CREATE TABLE "subscriptions"."destination" (
    "id" BIGSERIAL PRIMARY KEY,
    "channel_type" TEXT NOT NULL,
    "webhook_url" TEXT NOT NULL,
    "is_active" BOOLEAN NOT NULL DEFAULT true,
    "created_at" TIMESTAMPTZ(3) NOT NULL DEFAULT NOW(),
    "updated_at" TIMESTAMPTZ(3) NOT NULL DEFAULT NOW(),
    CONSTRAINT "destination_channel_type_chk" CHECK ("channel_type" IN ('discord_webhook', 'http_callback'))
);

-- Match rule: watch a player membership id and/or a clan group id.
CREATE TABLE "subscriptions"."rule" (
    "id" BIGSERIAL PRIMARY KEY,
    "destination_id" BIGINT NOT NULL,
    "scope" TEXT NOT NULL,
    "membership_id" BIGINT,
    "group_id" BIGINT,
    "is_active" BOOLEAN NOT NULL DEFAULT true,
    "created_at" TIMESTAMPTZ(3) NOT NULL DEFAULT NOW(),
    CONSTRAINT "rule_destination_fkey" FOREIGN KEY ("destination_id") REFERENCES "subscriptions"."destination" ("id") ON DELETE CASCADE,
    CONSTRAINT "rule_scope_chk" CHECK ("scope" IN ('player', 'clan')),
    CONSTRAINT "rule_target_chk" CHECK (
        ("scope" = 'player' AND "membership_id" IS NOT NULL AND "group_id" IS NULL)
        OR ("scope" = 'clan' AND "group_id" IS NOT NULL AND "membership_id" IS NULL)
    )
);

-- One active (destination, player) and (destination, clan) pair each; uniqueness is unchanged if column order is swapped.
-- Leading membership_id / group_id matches how we query (ANY on those ids), so we do not need separate btree indexes.
CREATE UNIQUE INDEX "idx_rule_player_unique" ON "subscriptions"."rule" ("membership_id", "destination_id")
    WHERE "scope" = 'player' AND "is_active";
CREATE UNIQUE INDEX "idx_rule_clan_unique" ON "subscriptions"."rule" ("group_id", "destination_id")
    WHERE "scope" = 'clan' AND "is_active";

GRANT USAGE ON SCHEMA "subscriptions" TO readonly;
GRANT SELECT ON ALL TABLES IN SCHEMA "subscriptions" TO readonly;
ALTER DEFAULT PRIVILEGES IN SCHEMA "subscriptions" GRANT SELECT ON TABLES TO readonly;
