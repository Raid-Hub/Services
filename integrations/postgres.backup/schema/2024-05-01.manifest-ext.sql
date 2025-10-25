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

CREATE TABLE "weapon_definition" (
    "hash" BIGINT NOT NULL PRIMARY KEY,
    "name" TEXT NOT NULL,
    "icon_path" TEXT NOT NULL,
    "weapon_type" "WeaponType" NOT NULL,
    "element" "WeaponElement" NOT NULL,
    "slot" "WeaponSlot" NOT NULL,
    "ammo_type" "WeaponAmmoType" NOT NULL,
    "rarity" "WeaponRarity" NOT NULL
);

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

CREATE TABLE "class_definition" (
    "hash" BIGINT NOT NULL PRIMARY KEY,
    "name" TEXT NOT NULL
);