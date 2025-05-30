-- Tables
CREATE TABLE "player" (
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
    "last_seen" TIMESTAMP(3) NOT NULL,
    "clears" INTEGER NOT NULL DEFAULT 0,
    "fresh_clears" INTEGER NOT NULL DEFAULT 0,
    "sherpas" INTEGER NOT NULL DEFAULT 0,
    "total_time_played_seconds" INTEGER NOT NULL DEFAULT 0,
    "sum_of_best" INTEGER,
    "last_crawled" TIMESTAMP(3),
    "_search_score" NUMERIC(14, 4) GENERATED ALWAYS AS (
        sqrt("clears" + 1) * power(2, GREATEST(0, EXTRACT(EPOCH FROM ("last_seen" AT TIME ZONE 'UTC' - TIMESTAMP '2017-08-27'))) / 20000000)
    ) STORED,
    "cheat_level" SMALLINT NOT NULL DEFAULT 0,
    "is_private" BOOLEAN NOT NULL DEFAULT false,
    "history_last_crawled" TIMESTAMP(3)
);
CREATE INDEX "primary_search_idx" ON "player"(lower("bungie_name") text_pattern_ops, "_search_score" DESC);
CREATE INDEX "secondary_search_idx" ON "player"(lower("display_name") text_pattern_ops, "_search_score" DESC);

CREATE TABLE "activity_definition" (
    "id" INTEGER NOT NULL PRIMARY KEY,
    "name" TEXT NOT NULL,
    "is_sunset" BOOLEAN NOT NULL DEFAULT false,
    "is_raid" BOOLEAN NOT NULL DEFAULT true,
    "path" TEXT NOT NULL,
    "release_date" TIMESTAMP(0) WITH TIME ZONE NOT NULL,
    "day_one_end" TIMESTAMP(0) WITH TIME ZONE GENERATED ALWAYS AS ("release_date" AT TIME ZONE 'UTC' + INTERVAL '1 day') STORED,
    "contest_end" TIMESTAMP(0) WITH TIME ZONE,
    "week_one_end" TIMESTAMP(0) WITH TIME ZONE,
    "milestone_hash" BIGINT
);

CREATE TABLE "version_definition" (
    "id" INTEGER NOT NULL PRIMARY KEY,
    "name" TEXT NOT NULL,
    "associated_activity_id" INTEGER,
    "path" TEXT NOT NULL,
    "is_challenge_mode" BOOLEAN NOT NULL DEFAULT false,
    CONSTRAINT "version_definition_associated_activity_id_fkey" FOREIGN KEY ("associated_activity_id") REFERENCES "activity_definition"("id") ON DELETE SET NULL ON UPDATE CASCADE
);

CREATE TABLE "activity_version" (
    "hash" BIGINT NOT NULL PRIMARY KEY,
    "activity_id" INTEGER NOT NULL,
    "version_id" INTEGER NOT NULL,
    "is_world_first" BOOLEAN NOT NULL DEFAULT false,
    "release_date_override" TIMESTAMP(0),
    CONSTRAINT "activity_version_activity_id_fkey" FOREIGN KEY ("activity_id") REFERENCES "activity_definition"("id") ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT "activity_version_version_id_fkey" FOREIGN KEY ("version_id") REFERENCES "version_definition"("id") ON DELETE RESTRICT ON UPDATE CASCADE
);
CREATE INDEX "idx_activity_version_activity_id" ON "activity_version"("activity_id");
CREATE INDEX "idx_activity_version_version_id" ON "activity_version"("version_id");

CREATE TABLE "season" (
    "id" INTEGER NOT NULL PRIMARY KEY,
    "short_name" TEXT NOT NULL,
    "long_name" TEXT NOT NULL,
    "dlc" TEXT NOT NULL,
    "start_date" TIMESTAMP(0) WITH TIME ZONE NOT NULL
);
CREATE INDEX "season_idx_date" ON "season"(start_date DESC);

CREATE OR REPLACE FUNCTION get_season(start_date_utc TIMESTAMP WITH TIME ZONE)
RETURNS INTEGER AS $$
DECLARE
    season_id INTEGER;
BEGIN
    SELECT ("id") INTO season_id FROM "season"
    WHERE "season"."start_date" < start_date_utc
    ORDER BY "season"."start_date" DESC
    LIMIT 1;

    RETURN season_id;
END;
$$ LANGUAGE plpgsql
IMMUTABLE;

CREATE TABLE "instance" (
    "instance_id" BIGINT NOT NULL PRIMARY KEY,
    "hash" BIGINT NOT NULL,
    "score" INT NOT NULL DEFAULT 0,
    "flawless" BOOLEAN,
    "completed" BOOLEAN NOT NULL,
    "fresh" BOOLEAN,
    "player_count" INTEGER NOT NULL,
    "date_started" TIMESTAMP(0) WITH TIME ZONE NOT NULL,
    "date_completed" TIMESTAMP(0) WITH TIME ZONE NOT NULL,
    "duration" INTEGER NOT NULL,
    "platform_type" INTEGER NOT NULL,
    "season_id" INTEGER GENERATED ALWAYS AS (get_season("date_started" AT TIME ZONE 'UTC')) STORED,
    "cheat_override" BOOLEAN NOT NULL DEFAULT False,
    CONSTRAINT "activity_version_fk" FOREIGN KEY ("hash") REFERENCES "activity_version"("hash")
);

CREATE INDEX "hash_date_completed_index_partial" ON "instance"("hash", "date_completed") WHERE "completed";
CREATE INDEX "tag_index_partial" ON "instance"("hash", "player_count", "fresh", "flawless") WHERE "completed" AND ("player_count" <= 3 OR "flawless");
CREATE INDEX "speedrun_index_partial" ON "instance"("hash", "duration") WHERE "completed" AND "fresh";
CREATE INDEX "score_idx_partial" ON "instance"("hash", "score" DESC) WHERE "completed" AND "fresh" AND "score" > 0;

CREATE TABLE "instance_player" (
    "instance_id" BIGINT NOT NULL,
    "membership_id" BIGINT NOT NULL,
    "completed" BOOLEAN NOT NULL,
    "time_played_seconds" INTEGER NOT NULL DEFAULT 0,
    "sherpas" INTEGER NOT NULL DEFAULT 0,
    "is_first_clear" BOOLEAN NOT NULL DEFAULT false,
    CONSTRAINT "instance_player_instance_id_membership_id_pkey" PRIMARY KEY ("instance_id","membership_id"),
    CONSTRAINT "instance_player_instance_id_fkey" FOREIGN KEY ("instance_id") REFERENCES "instance"("instance_id") ON DELETE RESTRICT ON UPDATE NO ACTION,
    CONSTRAINT "instance_player_membership_id_fkey" FOREIGN KEY ("membership_id") REFERENCES "player"("membership_id") ON DELETE RESTRICT ON UPDATE NO ACTION
);
CREATE INDEX "idx_instance_id" ON "instance_player"("instance_id");
CREATE INDEX "idx_membership_id" ON "instance_player"("membership_id");
CREATE INDEX "idx_instance_player_is_first_clear" ON "instance_player"("is_first_clear") INCLUDE (instance_id) WHERE is_first_clear;

CREATE TABLE "instance_character" (
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
    CONSTRAINT "instance_character_instance_id_membership_id_fkey" FOREIGN KEY ("instance_id", "membership_id") REFERENCES "instance_player"("instance_id", "membership_id") ON DELETE RESTRICT ON UPDATE NO ACTION
);
CREATE INDEX "instance_character_idx_membership_id" ON "instance_character"("membership_id");

CREATE TABLE "instance_character_weapon" (
    "instance_id" BIGINT NOT NULL,
    "membership_id" BIGINT NOT NULL,
    "character_id" BIGINT NOT NULL,
    "weapon_hash" BIGINT NOT NULL,
    "kills" INTEGER NOT NULL DEFAULT 0,
    "precision_kills" INTEGER NOT NULL DEFAULT 0,
    CONSTRAINT "instance_character_weapon_pkey" PRIMARY KEY ("instance_id","membership_id","character_id","weapon_hash"),
    CONSTRAINT "instance_character_weapon_fkey" FOREIGN KEY ("instance_id","membership_id","character_id") REFERENCES "instance_character"("instance_id","membership_id","character_id") ON DELETE RESTRICT ON UPDATE NO ACTION
);

CREATE TABLE "player_stats" (
    "membership_id" BIGINT NOT NULL,
    "activity_id" INTEGER NOT NULL,
    "clears" INTEGER NOT NULL DEFAULT 0,
    "fresh_clears" INTEGER NOT NULL DEFAULT 0,
    "sherpas" INTEGER NOT NULL DEFAULT 0,
    "fastest_instance_id" BIGINT,
    "total_time_played_seconds" INTEGER NOT NULL DEFAULT 0,
    CONSTRAINT "player_stats_pkey" PRIMARY KEY ("membership_id","activity_id"),
    CONSTRAINT "activity_id_fkey" FOREIGN KEY ("activity_id") REFERENCES "activity_definition"("id") ON DELETE RESTRICT ON UPDATE NO ACTION,
    CONSTRAINT "player_membership_id_fkey" FOREIGN KEY ("membership_id") REFERENCES "player"("membership_id") ON DELETE RESTRICT ON UPDATE NO ACTION,
    CONSTRAINT "player_stats_fastest_instance_id_fkey" FOREIGN KEY ("fastest_instance_id") REFERENCES "instance"("instance_id") ON DELETE SET NULL ON UPDATE CASCADE
);