-- MotT difficulty tier (Adventure / Standard / Custom) for tier-collection activities.

ALTER TABLE "core"."instance"
    ADD COLUMN "difficulty_tier" TEXT;

ALTER TABLE "core"."instance"
    ADD CONSTRAINT "instance_difficulty_tier_check"
    CHECK ("difficulty_tier" IS NULL OR "difficulty_tier" IN ('adventure', 'standard', 'custom'));
