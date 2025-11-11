-- RaidHub Services - Extended Schema Migration
-- Character-level data for instances (can be stored on separate disk)

-- =============================================================================
-- SCHEMA CREATION
-- =============================================================================

-- Create extended schema
CREATE SCHEMA IF NOT EXISTS "extended";

-- =============================================================================
-- EXTENDED TABLES
-- =============================================================================

-- Instance characters (detailed character data for performance optimization)
CREATE TABLE "extended"."instance_character" (
    "instance_id" BIGINT NOT NULL,
    "membership_id" BIGINT NOT NULL,
    "character_id" BIGINT NOT NULL,
    "class_hash" BIGINT,
    "emblem_hash" BIGINT,
    "completed" BOOLEAN NOT NULL,
    "score" INTEGER NOT NULL DEFAULT 0,
    "kills" INTEGER NOT NULL DEFAULT 0,
    "assists" INTEGER NOT NULL DEFAULT 0,
    "deaths" INTEGER NOT NULL DEFAULT 0,
    "precision_kills" INTEGER NOT NULL DEFAULT 0,
    "super_kills" INTEGER NOT NULL DEFAULT 0,
    "grenade_kills" INTEGER NOT NULL DEFAULT 0,
    "melee_kills" INTEGER NOT NULL DEFAULT 0,
    "time_played_seconds" INTEGER NOT NULL,
    "start_seconds" INTEGER NOT NULL,
    CONSTRAINT "instance_character_pkey" PRIMARY KEY ("instance_id", "membership_id", "character_id"),
    CONSTRAINT "instance_character_instance_id_membership_id_fkey" FOREIGN KEY ("instance_id", "membership_id") REFERENCES "core"."instance_player"("instance_id", "membership_id") ON DELETE RESTRICT ON UPDATE NO ACTION
);
CREATE INDEX "instance_character_idx_membership_id" ON "extended"."instance_character"("membership_id");

-- Instance character weapons (detailed weapon usage data)
CREATE TABLE "extended"."instance_character_weapon" (
    "instance_id" BIGINT NOT NULL,
    "membership_id" BIGINT NOT NULL,
    "character_id" BIGINT NOT NULL,
    "weapon_hash" BIGINT NOT NULL,
    "kills" INTEGER NOT NULL DEFAULT 0,
    "precision_kills" INTEGER NOT NULL DEFAULT 0,
    CONSTRAINT "instance_character_weapon_pkey" PRIMARY KEY ("instance_id","membership_id","character_id","weapon_hash"),
    CONSTRAINT "instance_character_weapon_fkey" FOREIGN KEY ("instance_id","membership_id","character_id") REFERENCES "extended"."instance_character"("instance_id","membership_id","character_id") ON DELETE RESTRICT ON UPDATE NO ACTION
);
