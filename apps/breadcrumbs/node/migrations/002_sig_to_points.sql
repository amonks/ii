-- Move significance from the rtree into the points table for faster bulk updates.
-- The rtree keeps only spatial dimensions (lat/lon); significance is filtered
-- via a regular column with an index.

ALTER TABLE points ADD COLUMN significance REAL NOT NULL DEFAULT 0;

-- Backfill significance from the rtree.
UPDATE points SET significance = (
    SELECT max_sig FROM points_idx WHERE points_idx.id = points.id
);

-- Rebuild rtree without sig dimensions.
DROP TABLE points_idx;
CREATE VIRTUAL TABLE points_idx USING rtree(
    id,
    min_lat, max_lat,
    min_lon, max_lon
);
INSERT INTO points_idx (id, min_lat, max_lat, min_lon, max_lon)
    SELECT id, lat, lat, lon, lon FROM points;

CREATE INDEX idx_points_significance ON points(significance);
