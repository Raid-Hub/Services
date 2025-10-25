-- RaidHub Services - Flagging Database Schema
-- Cheat detection, blacklists, and analytics

-- =============================================================================
-- FLAGGING TABLES
-- =============================================================================

-- Blacklist report source enum
CREATE TYPE "flagging"."BlacklistReportSource" AS ENUM (
    'Manual',
    'WebReport',
    'CheatCheck',
    'BlacklistedPlayerCascade'
);

-- Flag instances
CREATE TABLE "flagging"."flag_instance" (
    "instance_id" BIGINT NOT NULL,
    "cheat_check_version" TEXT NOT NULL,
    "cheat_check_bitmask" BIGINT NOT NULL,
    "flagged_at" TIMESTAMPTZ DEFAULT NOW(),
    "cheat_probability" NUMERIC CHECK ("cheat_probability" >= 0 AND "cheat_probability" <= 1),

    CONSTRAINT "flag_instance_pkey" PRIMARY KEY ("instance_id", "cheat_check_version")
    -- Note: instance_id references instance table in core database
    -- Cross-schema foreign keys not supported, handled at application level
);
CREATE INDEX "flag_instance_flagged_at" ON "flagging"."flag_instance"("flagged_at" DESC);

-- Flag instance players
CREATE TABLE "flagging"."flag_instance_player" (
    "instance_id" BIGINT NOT NULL,
    "membership_id" BIGINT NOT NULL,
    "cheat_check_version" TEXT NOT NULL,
    "cheat_check_bitmask" BIGINT NOT NULL,
    "flagged_at" TIMESTAMPTZ DEFAULT NOW(),
    "cheat_probability" NUMERIC CHECK ("cheat_probability" >= 0 AND "cheat_probability" <= 1),

    CONSTRAINT "flag_instance_player_pkey" PRIMARY KEY ("instance_id", "membership_id", "cheat_check_version")
    -- Note: instance_id, membership_id references instance_player table in core database
    -- Cross-schema foreign keys not supported, handled at application level
);
CREATE INDEX "flag_instance_player_membership_id" ON "flagging"."flag_instance_player"("membership_id");
CREATE INDEX "flag_instance_player_flagged_at" ON "flagging"."flag_instance_player"("flagged_at" DESC);

-- Blacklist instances
CREATE TABLE "flagging"."blacklist_instance" (
    "instance_id" BIGINT NOT NULL PRIMARY KEY,
    "report_source" "flagging"."BlacklistReportSource" NOT NULL,
    "report_id" BIGINT,
    "cheat_check_version" TEXT,
    "reason" TEXT NOT NULL,
    "created_at" TIMESTAMPTZ DEFAULT NOW()
    -- Note: instance_id references instance table in core database
    -- Cross-schema foreign keys not supported, handled at application level
);

-- Blacklist instance players
CREATE TABLE "flagging"."blacklist_instance_player" (
    "instance_id" BIGINT NOT NULL,
    "membership_id" BIGINT NOT NULL,
    "reason" TEXT NOT NULL,

    CONSTRAINT "blacklist_instance_player_pkey" PRIMARY KEY ("instance_id", "membership_id"),
    CONSTRAINT "blacklist_fkey" FOREIGN KEY ("instance_id") REFERENCES "flagging"."blacklist_instance"("instance_id") ON DELETE CASCADE ON UPDATE NO ACTION
    -- Note: instance_id, membership_id references instance_player table in core database
    -- Cross-schema foreign keys not supported, handled at application level
);
CREATE INDEX "blacklist_instance_player_membership_id" ON "flagging"."blacklist_instance_player"("membership_id");
