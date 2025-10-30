-- RaidHub Services - Core Schema Migration
-- Core game data: players, instances, participation

-- =============================================================================
-- SCHEMA CREATION
-- =============================================================================

-- Create schemas
CREATE SCHEMA IF NOT EXISTS "core";
CREATE SCHEMA IF NOT EXISTS "definitions";
CREATE SCHEMA IF NOT EXISTS "clan";
CREATE SCHEMA IF NOT EXISTS "flagging";
CREATE SCHEMA IF NOT EXISTS "leaderboard";
CREATE SCHEMA IF NOT EXISTS "extended";
CREATE SCHEMA IF NOT EXISTS "raw";

-- Set search path to include all schemas
SET search_path TO "core", "definitions", "clan", "flagging", "leaderboard", "extended", "raw", public;

-- =============================================================================
-- CORE TABLES
-- =============================================================================

-- Player table
CREATE TABLE "core"."player" (
    "membership_id" BIGINT NOT NULL PRIMARY KEY,
    "membership_type" INTEGER,
    "icon_path" TEXT,
    "display_name" TEXT,
    "bungie_global_display_name" TEXT,
    "bungie_global_display_name_code" TEXT,
    "bungie_name" TEXT GENERATED ALWAYS AS (
        CASE
            WHEN "bungie_global_display_name" IS NOT NULL AND "bungie_global_display_name_code" IS NOT NULL THEN
                "bungie_global_display_name" || '#' || "bungie_global_display_name_code"
            ELSE
                "display_name"
        END
    ) STORED, 
    "last_seen" TIMESTAMPTZ(3) NOT NULL,
    "first_seen" TIMESTAMPTZ(3) NOT NULL,
    "clears" INTEGER NOT NULL DEFAULT 0,
    "fresh_clears" INTEGER NOT NULL DEFAULT 0,
    "sherpas" INTEGER NOT NULL DEFAULT 0,
    "total_time_played_seconds" INTEGER NOT NULL DEFAULT 0,
    "sum_of_best" INTEGER,
    "last_crawled" TIMESTAMPTZ(3),
    "wfr_score" DOUBLE PRECISION NOT NULL DEFAULT 0,
    "cheat_level" SMALLINT NOT NULL DEFAULT 0,
    "is_private" BOOLEAN NOT NULL DEFAULT false,
    "history_last_crawled" TIMESTAMPTZ(3),
    "updated_at" TIMESTAMPTZ(3) NOT NULL DEFAULT NOW(),
    "_search_score" NUMERIC(14, 4) GENERATED ALWAYS AS (sqrt("clears" + 1) * power(2, GREATEST(0, EXTRACT(EPOCH FROM ("last_seen" - TIMESTAMPTZ '2017-08-27')) / 20000000)) * sqrt("wfr_score" + 1)) STORED
);
CREATE INDEX "primary_search_idx" ON "core"."player"(lower("bungie_name") text_pattern_ops, "_search_score" DESC);
CREATE INDEX "secondary_search_idx" ON "core"."player"(lower("display_name") text_pattern_ops, "_search_score" DESC);

-- Instances
CREATE TABLE "core"."instance" (
    "instance_id" BIGINT NOT NULL PRIMARY KEY,
    "hash" BIGINT NOT NULL,
    "score" INT NOT NULL DEFAULT 0,
    "flawless" BOOLEAN,
    "completed" BOOLEAN NOT NULL,
    "fresh" BOOLEAN,
    "player_count" INTEGER NOT NULL,
    "date_started" TIMESTAMPTZ(0) NOT NULL,
    "date_completed" TIMESTAMPTZ(0) NOT NULL,
    "duration" INTEGER NOT NULL,
    "platform_type" INTEGER NOT NULL,
    "season_id" INTEGER,
    "cheat_override" BOOLEAN NOT NULL DEFAULT False,
    "skull_hashes" BIGINT[]
);

CREATE INDEX "hash_date_completed_index_partial" ON "core"."instance"("hash", "date_completed") WHERE "completed";
CREATE INDEX "tag_index_partial" ON "core"."instance"("hash", "player_count", "fresh", "flawless") WHERE "completed" AND ("player_count" <= 3 OR "flawless");
CREATE INDEX "speedrun_index_partial" ON "core"."instance"("hash", "duration") WHERE "completed" AND "fresh";
CREATE INDEX "score_idx_partial" ON "core"."instance"("hash", "score" DESC) WHERE "completed" AND "fresh" AND "score" > 0;

-- Instance players
CREATE TABLE "core"."instance_player" (
    "instance_id" BIGINT NOT NULL,
    "membership_id" BIGINT NOT NULL,
    "completed" BOOLEAN NOT NULL,
    "time_played_seconds" INTEGER NOT NULL DEFAULT 0,
    "sherpas" INTEGER NOT NULL DEFAULT 0,
    "is_first_clear" BOOLEAN NOT NULL DEFAULT false,
    CONSTRAINT "instance_player_instance_id_membership_id_pkey" PRIMARY KEY ("instance_id","membership_id"),
    CONSTRAINT "instance_player_instance_id_fkey" FOREIGN KEY ("instance_id") REFERENCES "core"."instance"("instance_id") ON DELETE RESTRICT ON UPDATE NO ACTION,
    CONSTRAINT "instance_player_membership_id_fkey" FOREIGN KEY ("membership_id") REFERENCES "core"."player"("membership_id") ON DELETE RESTRICT ON UPDATE NO ACTION
);
CREATE INDEX "idx_instance_id" ON "core"."instance_player"("instance_id");
CREATE INDEX "idx_membership_id" ON "core"."instance_player"("membership_id");
CREATE INDEX "idx_instance_player_is_first_clear" ON "core"."instance_player"("is_first_clear") INCLUDE (instance_id) WHERE is_first_clear;

-- Player stats
CREATE TABLE "core"."player_stats" (
    "membership_id" BIGINT NOT NULL,
    "activity_id" INTEGER NOT NULL,
    "clears" INTEGER NOT NULL DEFAULT 0,
    "fresh_clears" INTEGER NOT NULL DEFAULT 0,
    "sherpas" INTEGER NOT NULL DEFAULT 0,
    "fastest_instance_id" BIGINT,
    "total_time_played_seconds" INTEGER NOT NULL DEFAULT 0,
    CONSTRAINT "player_stats_pkey" PRIMARY KEY ("membership_id","activity_id"),
    CONSTRAINT "player_membership_id_fkey" FOREIGN KEY ("membership_id") REFERENCES "core"."player"("membership_id") ON DELETE RESTRICT ON UPDATE NO ACTION,
    CONSTRAINT "player_stats_fastest_instance_id_fkey" FOREIGN KEY ("fastest_instance_id") REFERENCES "core"."instance"("instance_id") ON DELETE SET NULL ON UPDATE CASCADE
);
