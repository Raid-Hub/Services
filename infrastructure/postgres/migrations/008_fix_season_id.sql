-- Recalculates season_id for existing instances that may have NULL values
-- This is needed because season_id is a STORED generated column that only
-- calculates at INSERT time. If instances were inserted before seasons were
-- added to the database, they would have NULL season_id values.

-- Fix the function to ensure it NEVER returns NULL - raise error if no season found
-- Keep it IMMUTABLE as required for generated columns
CREATE OR REPLACE FUNCTION "definitions".get_season(start_date_utc TIMESTAMPTZ)
RETURNS INTEGER AS $$
DECLARE
    season_id INTEGER;
BEGIN
    SELECT ("id") INTO season_id FROM "definitions"."season"
    WHERE "definitions"."season"."start_date" < start_date_utc
    ORDER BY "definitions"."season"."start_date" DESC
    LIMIT 1;

    -- If no season found, raise an error - this indicates missing season data
    IF season_id IS NULL THEN
        RAISE EXCEPTION 'No season found for date %', start_date_utc;
    END IF;

    RETURN season_id;
END;
$$ LANGUAGE plpgsql IMMUTABLE;

-- Update the date_started column to itself, which triggers recalculation
-- of all generated columns that depend on it (including season_id)
-- We update all rows to ensure they use the updated function
UPDATE "core"."instance" 
SET "date_started" = "date_started";

-- Now add NOT NULL constraint to season_id
ALTER TABLE "core"."instance" 
ALTER COLUMN "season_id" SET NOT NULL;
