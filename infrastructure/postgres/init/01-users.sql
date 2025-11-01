-- PostgreSQL database initialization
-- Creates database, schemas, and users

-- Create database (ignore if already exists)
SELECT 'CREATE DATABASE raidhub'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'raidhub')\gexec

-- Grant permissions and all
GRANT ALL PRIVILEGES ON DATABASE raidhub TO postgres;

-- Grant schema usage
\c raidhub

-- Create public schemas
CREATE SCHEMA IF NOT EXISTS "core";
CREATE SCHEMA IF NOT EXISTS "definitions";
CREATE SCHEMA IF NOT EXISTS "clan";
CREATE SCHEMA IF NOT EXISTS "leaderboard";
CREATE SCHEMA IF NOT EXISTS "extended";
CREATE SCHEMA IF NOT EXISTS "raw";

-- Cache other schemas
CREATE SCHEMA IF NOT EXISTS "flagging";
CREATE SCHEMA IF NOT EXISTS "cache";

-- Create readonly user (ignore if already exists) - hardcoded as readonly with password 'password'
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'readonly') THEN
        CREATE USER readonly WITH PASSWORD 'password';
    END IF;
END
$$;

-- Grant readonly permissions to readonly user
GRANT CONNECT ON DATABASE raidhub TO readonly;
GRANT USAGE ON SCHEMA "core" TO readonly;
GRANT USAGE ON SCHEMA "definitions" TO readonly;
GRANT USAGE ON SCHEMA "clan" TO readonly;
GRANT USAGE ON SCHEMA "leaderboard" TO readonly;
GRANT USAGE ON SCHEMA "extended" TO readonly;
GRANT USAGE ON SCHEMA "raw" TO readonly;

-- Grant SELECT on all existing and future tables in all schemas (for ClickHouse PostgreSQL engine)
GRANT SELECT ON ALL TABLES IN SCHEMA "core" TO readonly;
GRANT SELECT ON ALL TABLES IN SCHEMA "definitions" TO readonly;
GRANT SELECT ON ALL TABLES IN SCHEMA "clan" TO readonly;
GRANT SELECT ON ALL TABLES IN SCHEMA "leaderboard" TO readonly;
GRANT SELECT ON ALL TABLES IN SCHEMA "extended" TO readonly;
GRANT SELECT ON ALL TABLES IN SCHEMA "raw" TO readonly;

-- Set default privileges for future tables
ALTER DEFAULT PRIVILEGES IN SCHEMA "core" GRANT SELECT ON TABLES TO readonly;
ALTER DEFAULT PRIVILEGES IN SCHEMA "definitions" GRANT SELECT ON TABLES TO readonly;
ALTER DEFAULT PRIVILEGES IN SCHEMA "clan" GRANT SELECT ON TABLES TO readonly;
ALTER DEFAULT PRIVILEGES IN SCHEMA "leaderboard" GRANT SELECT ON TABLES TO readonly;
ALTER DEFAULT PRIVILEGES IN SCHEMA "extended" GRANT SELECT ON TABLES TO readonly;
ALTER DEFAULT PRIVILEGES IN SCHEMA "raw" GRANT SELECT ON TABLES TO readonly;

-- =============================================================================
-- MIGRATION TRACKING
-- =============================================================================

-- Create migration tracking table
CREATE TABLE IF NOT EXISTS _migrations (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    applied_at TIMESTAMP DEFAULT NOW()
);

