-- Revert courts.location back to geography(Point, 4326)

DROP INDEX IF EXISTS idx_courts_location;

ALTER TABLE courts
    ALTER COLUMN location TYPE geography(Point, 4326)
    USING location::geography;

CREATE INDEX idx_courts_location ON courts USING GIST (location);
