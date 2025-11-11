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

    CONSTRAINT "flag_instance_pkey" PRIMARY KEY ("instance_id", "cheat_check_version"),
    CONSTRAINT "flag_instance_instance_id_fkey" FOREIGN KEY ("instance_id") REFERENCES "core"."instance"("instance_id") ON DELETE RESTRICT ON UPDATE NO ACTION
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

    CONSTRAINT "flag_instance_player_pkey" PRIMARY KEY ("instance_id", "membership_id", "cheat_check_version"),
    CONSTRAINT "flag_instance_player_instance_id_membership_id_fkey" FOREIGN KEY ("instance_id", "membership_id") REFERENCES "core"."instance_player"("instance_id", "membership_id") ON DELETE RESTRICT ON UPDATE NO ACTION
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
    "created_at" TIMESTAMPTZ DEFAULT NOW(),
    CONSTRAINT "blacklist_instance_instance_id_fkey" FOREIGN KEY ("instance_id") REFERENCES "core"."instance"("instance_id") ON DELETE RESTRICT ON UPDATE NO ACTION
);

-- Blacklist instance players
CREATE TABLE "flagging"."blacklist_instance_player" (
    "instance_id" BIGINT NOT NULL,
    "membership_id" BIGINT NOT NULL,
    "reason" TEXT NOT NULL,

    CONSTRAINT "blacklist_instance_player_pkey" PRIMARY KEY ("instance_id", "membership_id"),
    CONSTRAINT "blacklist_fkey" FOREIGN KEY ("instance_id") REFERENCES "flagging"."blacklist_instance"("instance_id") ON DELETE CASCADE ON UPDATE NO ACTION,
    CONSTRAINT "blacklist_instance_player_instance_id_membership_id_fkey" FOREIGN KEY ("instance_id", "membership_id") REFERENCES "core"."instance_player"("instance_id", "membership_id") ON DELETE RESTRICT ON UPDATE NO ACTION
);
CREATE INDEX "blacklist_instance_player_membership_id" ON "flagging"."blacklist_instance_player"("membership_id");
