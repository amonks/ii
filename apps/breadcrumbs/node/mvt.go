package node

import (
	pb "monks.co/apps/breadcrumbs/proto"
	"google.golang.org/protobuf/proto"
)

const mvtExtent = 4096

// EncodeMVT encodes points as a Mapbox Vector Tile with a single "track"
// layer containing one LineString feature.
func EncodeMVT(points []*pb.Point, z, x, y int) ([]byte, error) {
	south, north, west, east, err := TileBBox(z, x, y)
	if err != nil {
		return nil, err
	}

	feature := &pb.Tile_Feature{
		Type: pb.Tile_LINESTRING.Enum(),
	}

	if len(points) > 0 {
		feature.Geometry = encodeLineString(points, south, north, west, east)
	}

	version := uint32(2)
	extent := uint32(mvtExtent)
	tile := &pb.Tile{
		Layers: []*pb.Tile_Layer{{
			Version:  &version,
			Name:     proto.String("track"),
			Extent:   &extent,
			Features: []*pb.Tile_Feature{feature},
		}},
	}

	return proto.Marshal(tile)
}

func encodeLineString(points []*pb.Point, south, north, west, east float64) []uint32 {
	if len(points) == 0 {
		return nil
	}

	width := east - west
	height := north - south

	// MoveTo(1) + first point
	cmds := make([]uint32, 0, 3+2*(len(points)-1))

	prevX, prevY := 0, 0
	for i, p := range points {
		tileX := int(((p.Longitude - west) / width) * mvtExtent)
		tileY := int(((north - p.Latitude) / height) * mvtExtent)

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

// commandInt encodes an MVT command integer: (id & 0x7) | (count << 3)
func commandInt(id, count uint32) uint32 {
	return (id & 0x7) | (count << 3)
}

// zigzag encodes a signed integer as a zigzag-encoded unsigned integer.
func zigzag(n int) uint32 {
	return uint32((n << 1) ^ (n >> 31))
}
