ALTER TABLE "activity_definition"
    ADD COLUMN IF NOT EXISTS "splash_path" TEXT;

ALTER TABLE "activity_definition"
    ALTER COLUMN "splash_path" SET NOT NULL;