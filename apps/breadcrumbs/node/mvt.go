package node

import (
	"math"

	"google.golang.org/protobuf/proto"
	pb "monks.co/apps/breadcrumbs/proto"
)

const mvtExtent = 4096

// maxPointsPerTile is the maximum number of points we encode into a single
// MVT tile. MapLibre's WebGL vertex buffer limit is 65,535 (uint16 index),
// and line rendering generates ~10-18 vertices per coordinate (for extruded
// width, miter joins, caps). We cap at 5,000 to stay safely under the limit.
const maxPointsPerTile = 5000

// EncodeMVT encodes point segments as a Mapbox Vector Tile with a "track"
// layer. Each segment becomes a separate LineString feature. If the total
// point count exceeds MapLibre's per-tile vertex limit, the budget is
// distributed proportionally across segments.
func EncodeMVT(segments [][]*pb.Point, z, x, y int) ([]byte, error) {
	south, north, west, east, err := TileBBox(z, x, y)
	if err != nil {
		return nil, err
	}

	// Downsample if needed, distributing budget proportionally.
	total := 0
	for _, seg := range segments {
		total += len(seg)
	}
	if total > maxPointsPerTile {
		for i, seg := range segments {
			budget := max(2, len(seg)*maxPointsPerTile/total)
			segments[i] = downsample(seg, budget)
		}
	}

	var features []*pb.Tile_Feature
	for _, seg := range segments {
		if len(seg) >= 2 {
			features = append(features, &pb.Tile_Feature{
				Type:     pb.Tile_LINESTRING.Enum(),
				Geometry: encodeLineString(seg, south, north, west, east),
			})
		}
	}

	version := uint32(2)
	extent := uint32(mvtExtent)
	tile := &pb.Tile{
		Layers: []*pb.Tile_Layer{{
			Version:  &version,
			Name:     new("track"),
			Extent:   &extent,
			Features: features,
		}},
	}

	return proto.Marshal(tile)
}

// downsample uniformly selects n points from pts, always including the
// first and last point. Points are evenly spaced by index.
func downsample(pts []*pb.Point, n int) []*pb.Point {
	if len(pts) <= n {
		return pts
	}
	out := make([]*pb.Point, n)
	out[0] = pts[0]
	out[n-1] = pts[len(pts)-1]
	for i := 1; i < n-1; i++ {
		idx := int(float64(i) * float64(len(pts)-1) / float64(n-1))
		out[i] = pts[idx]
	}
	return out
}

func encodeLineString(points []*pb.Point, south, north, west, east float64) []uint32 {
	if len(points) == 0 {
		return nil
	}

	width := east - west
	// Convert tile lat bounds to Mercator Y for correct projection.
	mercNorth := latToMercatorY(north)
	mercSouth := latToMercatorY(south)
	mercHeight := mercNorth - mercSouth

	// MoveTo(1) + first point
	cmds := make([]uint32, 0, 3+2*(len(points)-1))

	prevX, prevY := 0, 0
	for i, p := range points {
		tileX := int(((p.Longitude - west) / width) * mvtExtent)
		mercY := latToMercatorY(p.Latitude)
		tileY := int(((mercNorth - mercY) / mercHeight) * mvtExtent)

		dx := tileX - prevX
		dy := tileY - prevY

		if i == 0 {
			cmds = append(cmds, commandInt(1, 1)) // MoveTo, count=1
		} else if i == 1 {
			cmds = append(cmds, commandInt(2, uint32(len(points)-1))) // LineTo, count=n-1
		}

		cmds = append(cmds, zigzag(dx), zigzag(dy))
		prevX = tileX
		prevY = tileY
	}

	return cmds
}

// latToMercatorY converts a latitude in degrees to Web Mercator Y
// (projected coordinate in the range used by slippy map tiles).
func latToMercatorY(lat float64) float64 {
	latRad := lat * math.Pi / 180.0
	return math.Log(math.Tan(latRad/2 + math.Pi/4))
}

// commandInt encodes an MVT command integer: (id & 0x7) | (count << 3)
func commandInt(id, count uint32) uint32 {
	return (id & 0x7) | (count << 3)
}

// zigzag encodes a signed integer as a zigzag-encoded unsigned integer.
func zigzag(n int) uint32 {
	return uint32((n << 1) ^ (n >> 31))
}
