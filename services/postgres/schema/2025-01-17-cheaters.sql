CREATE TABLE "flag_instance" (
    "instance_id" BIGINT NOT NULL,
    "cheat_check_version" TEXT,
    "cheat_check_bitmask" BIGINT,
    "flagger_id" BIGINT,
    "flagged_at" TIMESTAMP DEFAULT now(),
    "cheat_probability" NUMERIC CHECK ("cheat_probability" >= 0 AND "cheat_probability" <= 1),
    "is_override" BOOLEAN CHECK ("is_override" IS TRUE OR "is_override" IS NULL),

    CONSTRAINT "flag_instance_src_check" CHECK (
        "cheat_check_version" IS NOT NULL OR "flagger_id" IS NOT NULL
    ),

    CONSTRAINT "flag_instance_pkey" PRIMARY KEY ("instance_id", "flagged_at"),
    CONSTRAINT "flag_instance_instance_id_fkey" FOREIGN KEY ("instance_id") REFERENCES "instance"("instance_id") ON DELETE RESTRICT ON UPDATE NO ACTION,
    CONSTRAINT "flag_instance_flagger_fkey" FOREIGN KEY ("flagger_id") REFERENCES "player"("membership_id") ON DELETE RESTRICT ON UPDATE NO ACTION,

    CONSTRAINT "flag_instance_unique_cheat_check" UNIQUE ("instance_id", "cheat_check_version"),
    CONSTRAINT "flag_instance_unique_flagger" UNIQUE ("instance_id", "flagger_id"),
    CONSTRAINT "flag_instance_unique_override" UNIQUE ("instance_id", "is_override")
);
CREATE INDEX "flag_instance_flagged_at" ON "flag_instance"("flagged_at" DESC);

CREATE TABLE "flag_instance_player" (
    "instance_id" BIGINT NOT NULL,
    "membership_id" BIGINT NOT NULL,
    "cheat_check_version" TEXT,
    "cheat_check_bitmask" BIGINT,
    "flagger_id" BIGINT,
    "flagged_at" TIMESTAMP DEFAULT now(),
    "cheat_probability" NUMERIC CHECK ("cheat_probability" >= 0 AND "cheat_probability" <= 1),
    "is_override" CHECK ("is_override" IS TRUE OR "is_override" IS NULL),

    CONSTRAINT "flag_instance_player_src_check" CHECK (
        "cheat_check_version" IS NOT NULL OR "flagger_id" IS NOT NULL
    ),

    CONSTRAINT "flag_instance_player_pkey" PRIMARY KEY ("instance_id", "membership_id", "flagged_at"),
    CONSTRAINT "flag_instance_player_fkey" FOREIGN KEY ("instance_id", "membership_id") REFERENCES "instance_player"("instance_id", "membership_id") ON DELETE RESTRICT ON UPDATE NO ACTION,
    CONSTRAINT "flag_instance_player_flagger_fkey" FOREIGN KEY ("flagger_id") REFERENCES "player"("membership_id") ON DELETE RESTRICT ON UPDATE NO ACTION,
    
    CONSTRAINT "flag_instance_player_unique_cheat_check" UNIQUE ("instance_id", "membership_id", "cheat_check_version"),
    CONSTRAINT "flag_instance_player_unique_flagger" UNIQUE ("instance_id", "membership_id", "flagger_id"),
    CONSTRAINT "flag_instance_player_unique_override" UNIQUE ( "instance_id", "membership_id", "is_override")
);
CREATE INDEX "flag_instance_player_membership_id" ON "flag_instance_player"("membership_id");
CREATE INDEX "flag_instance_player_flagged_at" ON "flag_instance_player"("flagged_at" DESC);