-- RaidHub Services - Definitions Schema Migration
-- Game definitions and catalog data

-- =============================================================================
-- DEFINITION TABLES
-- =============================================================================

-- Activity definitions
CREATE TABLE "definitions"."activity_definition" (
    "id" INTEGER NOT NULL PRIMARY KEY,
    "name" TEXT NOT NULL,
    "is_sunset" BOOLEAN NOT NULL DEFAULT false,
    "is_raid" BOOLEAN NOT NULL DEFAULT true,
    "path" TEXT NOT NULL,
    "release_date" TIMESTAMP(0) WITH TIME ZONE NOT NULL,
    "day_one_end" TIMESTAMP(0) WITH TIME ZONE GENERATED ALWAYS AS ("release_date" AT TIME ZONE 'UTC' + INTERVAL '1 day') STORED,
    "contest_end" TIMESTAMP(0) WITH TIME ZONE,
    "week_one_end" TIMESTAMP(0) WITH TIME ZONE,
    "milestone_hash" BIGINT,
    "splash_path" TEXT NOT NULL
);

-- Version definitions
CREATE TABLE "definitions"."version_definition" (
    "id" INTEGER NOT NULL PRIMARY KEY,
    "name" TEXT NOT NULL,
    "associated_activity_id" INTEGER,
    "path" TEXT NOT NULL,
    "is_challenge_mode" BOOLEAN NOT NULL DEFAULT false,
    CONSTRAINT "version_definition_associated_activity_id_fkey" FOREIGN KEY ("associated_activity_id") REFERENCES "definitions"."activity_definition"("id") ON DELETE SET NULL ON UPDATE CASCADE
);

-- Activity versions
CREATE TABLE "definitions"."activity_version" (
    "hash" BIGINT NOT NULL PRIMARY KEY,
    "activity_id" INTEGER NOT NULL,
    "version_id" INTEGER NOT NULL,
    "is_world_first" BOOLEAN NOT NULL DEFAULT false,
    "release_date_override" TIMESTAMP(0),
    CONSTRAINT "activity_version_activity_id_fkey" FOREIGN KEY ("activity_id") REFERENCES "definitions"."activity_definition"("id") ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT "activity_version_version_id_fkey" FOREIGN KEY ("version_id") REFERENCES "definitions"."version_definition"("id") ON DELETE RESTRICT ON UPDATE CASCADE
);
CREATE INDEX "idx_activity_version_activity_id" ON "definitions"."activity_version"("activity_id");
CREATE INDEX "idx_activity_version_version_id" ON "definitions"."activity_version"("version_id");

-- Seasons
CREATE TABLE "definitions"."season" (
    "id" INTEGER NOT NULL PRIMARY KEY,
    "short_name" TEXT NOT NULL,
    "long_name" TEXT NOT NULL,
    "dlc" TEXT NOT NULL,
    "start_date" TIMESTAMP(0) WITH TIME ZONE NOT NULL
);
CREATE INDEX "season_idx_date" ON "definitions"."season"(start_date DESC);

-- Season function
CREATE OR REPLACE FUNCTION "definitions".get_season(start_date_utc TIMESTAMP WITH TIME ZONE)
RETURNS INTEGER AS $$
DECLARE
    season_id INTEGER;
BEGIN
    SELECT ("id") INTO season_id FROM "definitions"."season"
    WHERE "definitions"."season"."start_date" < start_date_utc
    ORDER BY "definitions"."season"."start_date" DESC
    LIMIT 1;

    RETURN season_id;
END;
$$ LANGUAGE plpgsql VOLATILE;

-- Weapon enums
CREATE TYPE "WeaponElement" AS ENUM ('Kinetic', 'Arc', 'Solar', 'Void', 'Stasis', 'Strand');
CREATE TYPE "WeaponSlot" AS ENUM ('Kinetic', 'Energy', 'Power');
CREATE TYPE "WeaponAmmoType" AS ENUM ('Primary', 'Special', 'Heavy');
CREATE TYPE "WeaponRarity" AS ENUM ('Common', 'Uncommon', 'Rare', 'Legendary', 'Exotic');
CREATE TYPE "WeaponType" AS ENUM (
    'Auto Rifle',
    'Shotgun',
    'Machine Gun',
    'Hand Cannon',
    'Rocket Launcher',
    'Fusion Rifle',
    'Sniper Rifle',
    'Pulse Rifle',
    'Scout Rifle',
    'Sidearm',
    'Sword',
    'Linear Fusion Rifle',
    'Grenade Launcher',
    'Submachine Gun',
    'Trace Rifle',
    'Bow',
    'Glaive'
);

-- Weapon definitions table
CREATE TABLE "definitions"."weapon_definition" (
    "hash" BIGINT NOT NULL PRIMARY KEY,
    "name" TEXT NOT NULL,
    "icon_path" TEXT NOT NULL,
    "weapon_type" "WeaponType" NOT NULL,
    "element" "WeaponElement" NOT NULL,
    "slot" "WeaponSlot" NOT NULL,
    "ammo_type" "WeaponAmmoType" NOT NULL,
    "rarity" "WeaponRarity" NOT NULL
);

-- Weapon helper functions
CREATE OR REPLACE FUNCTION get_element(defaultDamageType SMALLINT)
RETURNS "WeaponElement" AS $$
DECLARE
    element "WeaponElement";
BEGIN
    CASE defaultDamageType
        WHEN 1 THEN element := 'Kinetic';
        WHEN 2 THEN element := 'Arc';
        WHEN 3 THEN element := 'Solar';
        WHEN 4 THEN element := 'Void';
        WHEN 6 THEN element := 'Stasis';
        WHEN 7 THEN element := 'Strand';
        ELSE RAISE EXCEPTION 'Invalid defaultDamageType, %', defaultDamageType;
    END CASE;
    
    RETURN element;
END;
$$ LANGUAGE plpgsql IMMUTABLE;

CREATE OR REPLACE FUNCTION get_ammo_type(ammoType SMALLINT)
RETURNS "WeaponAmmoType" AS $$
DECLARE
    ammo "WeaponAmmoType";
BEGIN
    CASE ammoType
        WHEN 1 THEN ammo := 'Primary';
        WHEN 2 THEN ammo := 'Special';
        WHEN 3 THEN ammo := 'Heavy';
        ELSE RAISE EXCEPTION 'Invalid ammoType, %', ammoType;
    END CASE;
    
    RETURN ammo;
END;
$$ LANGUAGE plpgsql IMMUTABLE;

CREATE OR REPLACE FUNCTION get_slot(equipmentSlotTypeHash BIGINT)
RETURNS "WeaponSlot" AS $$
DECLARE
    slot "WeaponSlot";
BEGIN
    CASE equipmentSlotTypeHash
        WHEN 1498876634 THEN slot := 'Kinetic';
        WHEN 2465295065 THEN slot := 'Energy';
        WHEN 953998645 THEN slot := 'Power';
        ELSE RAISE EXCEPTION 'Invalid equipmentSlotTypeHash, %', equipmentSlotTypeHash;
    END CASE;
    
    RETURN slot;
END;
$$ LANGUAGE plpgsql IMMUTABLE;

CREATE OR REPLACE FUNCTION get_weapon_type(itemSubType INT)
RETURNS "WeaponType" AS $$
DECLARE
    weapon_type "WeaponType";
BEGIN
    CASE itemSubType
        WHEN 6 THEN weapon_type := 'Auto Rifle';
        WHEN 7 THEN weapon_type := 'Shotgun';
        WHEN 8 THEN weapon_type := 'Machine Gun';
        WHEN 9 THEN weapon_type := 'Hand Cannon';
        WHEN 10 THEN weapon_type := 'Rocket Launcher';
        WHEN 11 THEN weapon_type := 'Fusion Rifle';
        WHEN 12 THEN weapon_type := 'Sniper Rifle';
        WHEN 13 THEN weapon_type := 'Pulse Rifle';
        WHEN 14 THEN weapon_type := 'Scout Rifle';
        WHEN 17 THEN weapon_type := 'Sidearm';
        WHEN 18 THEN weapon_type := 'Sword';
        WHEN 22 THEN weapon_type := 'Linear Fusion Rifle';
        WHEN 23 THEN weapon_type := 'Grenade Launcher';
        WHEN 24 THEN weapon_type := 'Submachine Gun';
        WHEN 25 THEN weapon_type := 'Trace Rifle';
        WHEN 31 THEN weapon_type := 'Bow';
        WHEN 33 THEN weapon_type := 'Glaive';
        ELSE
            RAISE EXCEPTION 'Invalid itemSubType: %', itemSubType;
    END CASE;
    
    RETURN weapon_type;
END;
$$ LANGUAGE plpgsql IMMUTABLE;

-- Class definitions
CREATE TABLE "definitions"."class_definition" (
    "hash" BIGINT NOT NULL PRIMARY KEY,
    "name" TEXT NOT NULL
);

-- Activity feat definitions
CREATE TABLE "definitions"."activity_feat_definition" (
    "hash" BIGINT NOT NULL PRIMARY KEY,
    "skull_hash" BIGINT NOT NULL,
    "name" TEXT NOT NULL,
    "name_short" TEXT GENERATED ALWAYS AS (
        trim(both from regexp_replace(name, '^Feat:\s*', '', 'i'))
    ) STORED,
    "description" TEXT NOT NULL,
    "icon" TEXT NOT NULL,
    "description_short" TEXT NOT NULL,
    "modifier_power_contribution" INT NOT NULL,
    "created_at" TIMESTAMP(0) NOT NULL DEFAULT NOW()
);

