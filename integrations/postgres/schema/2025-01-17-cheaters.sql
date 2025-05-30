CREATE TABLE "flag_instance" (
    "instance_id" BIGINT NOT NULL,
    "cheat_check_version" TEXT NOT NULL,
    "cheat_check_bitmask" BIGINT NOT NULL,
    "flagged_at" TIMESTAMPTZ DEFAULT NOW(),
    "cheat_probability" NUMERIC CHECK ("cheat_probability" >= 0 AND "cheat_probability" <= 1),

    CONSTRAINT "flag_instance_pkey" PRIMARY KEY ("instance_id", "cheat_check_version"),
    CONSTRAINT "flag_instance_instance_id_fkey" FOREIGN KEY ("instance_id") REFERENCES "instance"("instance_id") ON DELETE RESTRICT ON UPDATE NO ACTION
);
CREATE INDEX "flag_instance_flagged_at" ON "flag_instance"("flagged_at" DESC);

CREATE TABLE "flag_instance_player" (
    "instance_id" BIGINT NOT NULL,
    "membership_id" BIGINT NOT NULL,
    "cheat_check_version" TEXT NOT NULL,
    "cheat_check_bitmask" BIGINT NOT NULL,
    "flagged_at" TIMESTAMPTZ DEFAULT NOW(),
    "cheat_probability" NUMERIC CHECK ("cheat_probability" >= 0 AND "cheat_probability" <= 1),

    CONSTRAINT "flag_instance_player_pkey" PRIMARY KEY ("instance_id", "membership_id", "cheat_check_version"),
    CONSTRAINT "flag_instance_player_fkey" FOREIGN KEY ("instance_id", "membership_id") REFERENCES "instance_player"("instance_id", "membership_id") ON DELETE RESTRICT ON UPDATE NO ACTION,
);
CREATE INDEX "flag_instance_player_membership_id" ON "flag_instance_player"("membership_id");
CREATE INDEX "flag_instance_player_flagged_at" ON "flag_instance_player"("flagged_at" DESC);

CREATE TYPE "BlacklistReportSource" AS ENUM (
    'Manual',
    'WebReport',
    'CheatCheck',
    'BlacklistedPlayerCascade'
);

CREATE TABLE "blacklist_instance" (
    "instance_id" BIGINT NOT NULL PRIMARY KEY,
    "report_source" "BlacklistReportSource" NOT NULL,
    "report_id" BIGINT,
    "cheat_check_version" TEXT,
    "reason" TEXT NOT NULL,
    "created_at" TIMESTAMPTZ DEFAULT NOW(),

    CONSTRAINT "blacklist_instance_id_fkey" FOREIGN KEY ("instance_id") REFERENCES "instance"("instance_id") ON DELETE RESTRICT ON UPDATE NO ACTION
);

CREATE TABLE "blacklist_instance_player" (
    "instance_id" BIGINT NOT NULL,
    "membership_id" BIGINT NOT NULL,
    "reason" TEXT NOT NULL,

    CONSTRAINT "blacklist_instance_player_pkey" PRIMARY KEY ("instance_id", "membership_id"),
    CONSTRAINT "blacklist_fkey" FOREIGN KEY ("instance_id") REFERENCES "blacklist_instance"("instance_id") ON DELETE CASCADE ON UPDATE NO ACTION,
    CONSTRAINT "blacklist_instance_player_fkey" FOREIGN KEY ("instance_id", "membership_id") REFERENCES "instance_player"("instance_id", "membership_id") ON DELETE RESTRICT ON UPDATE NO ACTION
);
CREATE INDEX "blacklist_instance_player_membership_id" ON "blacklist_instance_player"("membership_id");

INSERT INTO "blacklist_instance" (instance_id, reason, report_source)
SELECT instance_id, 'Legacy', 'Manual'
FROM instance
WHERE cheat_override IS TRUE;

ALTER TABLE "instance" DROP COLUMN IF EXISTS "cheat_override";

BEGIN;
ALTER TABLE "player" ADD CONSTRAINT "cheat_level_range" CHECK ("cheat_level" >= 0 AND "cheat_level" <= 4);

ALTER TABLE "player" ADD COLUMN "first_seen" TIMESTAMP(3) DEFAULT NOW();

UPDATE player
SET first_seen = sub.first_seen
FROM (
  SELECT ip.membership_id, MIN(i.date_completed) AS first_seen
  FROM instance_player ip
  JOIN instance i ON ip.instance_id = i.instance_id
  GROUP BY ip.membership_id
) sub
WHERE player.membership_id = sub.membership_id;

COMMIT;


ALTER TABLE "player" ADD COLUMN "is_whitelisted" BOOLEAN DEFAULT FALSE;

ALTER TABLE activity_definition ADD COLUMN r2_path TEXT NOT NULL DEFAULT '';
ALTER TABLE activity_definition ALTER COLUMN r2_path DROP DEFAULT;

ALTER TABLE "instance" ADD COLUMN "is_whitelisted" BOOLEAN DEFAULT FALSE;