-- RaidHub Services - Clan Schema Migration
-- Clan information and membership

-- =============================================================================
-- CLAN TABLES
-- =============================================================================

-- Clan table
CREATE TABLE "clan"."clan" (
    "group_id" BIGINT NOT NULL PRIMARY KEY,
    "name" TEXT NOT NULL,
    "motto" TEXT NOT NULL,
    "call_sign" TEXT NOT NULL,
    "clan_banner_data" JSONB NOT NULL,
    "updated_at" TIMESTAMPTZ(3) NOT NULL DEFAULT NOW()
);

-- Clan members
CREATE TABLE "clan"."clan_members" (
    "group_id" BIGINT NOT NULL,
    "membership_id" BIGINT NOT NULL,
    PRIMARY KEY ("group_id", "membership_id"),
    CONSTRAINT "fk_clan_members_group_id" FOREIGN KEY ("group_id") REFERENCES "clan"."clan" ("group_id"),
    CONSTRAINT "fk_clan_members_membership_id" FOREIGN KEY ("membership_id") REFERENCES "core"."player" ("membership_id")
);
