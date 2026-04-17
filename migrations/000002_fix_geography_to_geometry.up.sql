-- Convert courts.location from geography(Point, 4326) to geometry(Point, 4326)
-- so that pg_tileserv can auto-discover the table as a publishable layer.

-- Drop the existing GIST index (built on geography type)
DROP INDEX IF EXISTS idx_courts_location;

-- Alter the column type from geography to geometry
ALTER TABLE courts
    ALTER COLUMN location TYPE geometry(Point, 4326)
    USING location::geometry;

-- Recreate the GIST index on the new geometry column
CREATE INDEX idx_courts_location ON courts USING GIST (location);
