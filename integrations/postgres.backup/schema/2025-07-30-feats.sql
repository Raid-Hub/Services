CREATE TABLE "activity_feat_definition" (
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

GRANT UPDATE, INSERT ON "activity_feat_definition" TO "raidhub_user";

ALTER TABLE "instance" ADD COLUMN "skull_hashes" BIGINT[] DEFAULT ARRAY[]::BIGINT[] NOT NULL;