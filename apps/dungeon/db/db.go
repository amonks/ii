package db

import (
	"fmt"
	"time"

	"monks.co/pkg/database"
)

type DB struct {
	*database.DB
}

type Map struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	Name      string    `gorm:"column:name" json:"name"`
	Type      string    `gorm:"column:type" json:"type"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

type Cell struct {
	ID         uint   `gorm:"primarykey" json:"id"`
	MapID      uint   `gorm:"column:map_id" json:"map_id"`
	X          int    `gorm:"column:x" json:"x"`
	Y          int    `gorm:"column:y" json:"y"`
	IsExplored bool   `gorm:"column:is_explored" json:"is_explored"`
	Text       string `gorm:"column:text" json:"text"`
	Hue        *int   `gorm:"column:hue" json:"hue"`
	RoomID     *uint  `gorm:"column:room_id" json:"room_id"`
}

type Wall struct {
	ID    uint   `gorm:"primarykey" json:"id"`
	MapID uint   `gorm:"column:map_id" json:"map_id"`
	X1    int    `gorm:"column:x1" json:"x1"`
	Y1    int    `gorm:"column:y1" json:"y1"`
	X2    int    `gorm:"column:x2" json:"x2"`
	Y2    int    `gorm:"column:y2" json:"y2"`
	Type  string `gorm:"column:type" json:"type"`
}

type Marker struct {
	ID     uint   `gorm:"primarykey" json:"id"`
	MapID  uint   `gorm:"column:map_id" json:"map_id"`
	X      int    `gorm:"column:x" json:"x"`
	Y      int    `gorm:"column:y" json:"y"`
	Letter string `gorm:"column:letter" json:"letter"`
}

const schema = `
CREATE TABLE IF NOT EXISTS maps (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL,
	type TEXT NOT NULL,
	created_at DATETIME
);

CREATE TABLE IF NOT EXISTS cells (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	map_id INTEGER NOT NULL REFERENCES maps(id),
	x INTEGER NOT NULL,
	y INTEGER NOT NULL,
	is_explored INTEGER NOT NULL DEFAULT 0,
	text TEXT NOT NULL DEFAULT '',
	hue INTEGER,
	room_id INTEGER,
	UNIQUE(map_id, x, y)
);
CREATE INDEX IF NOT EXISTS cells_by_map ON cells(map_id);

CREATE TABLE IF NOT EXISTS walls (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	map_id INTEGER NOT NULL REFERENCES maps(id),
	x1 INTEGER NOT NULL,
	y1 INTEGER NOT NULL,
	x2 INTEGER NOT NULL,
	y2 INTEGER NOT NULL,
	type TEXT NOT NULL,
	UNIQUE(map_id, x1, y1, x2, y2)
);
CREATE INDEX IF NOT EXISTS walls_by_map ON walls(map_id);

CREATE TABLE IF NOT EXISTS markers (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	map_id INTEGER NOT NULL REFERENCES maps(id),
	x INTEGER NOT NULL,
	y INTEGER NOT NULL,
	letter TEXT NOT NULL,
	UNIQUE(map_id, x, y)
);
CREATE INDEX IF NOT EXISTS markers_by_map ON markers(map_id);
`

func New() (*DB, error) {
	d, err := database.OpenFromDataFolder("dungeon")
	if err != nil {
		return nil, err
	}
	if err := d.Exec(schema).Error; err != nil {
		return nil, fmt.Errorf("schema: %w", err)
	}
	return &DB{d}, nil
}

func NewMemory() (*DB, error) {
	d, err := database.Open(":memory:")
	if err != nil {
		return nil, err
	}
	if err := d.Exec(schema).Error; err != nil {
		return nil, fmt.Errorf("schema: %w", err)
	}
	return &DB{d}, nil
}

// --- Map CRUD ---

func (db *DB) CreateMap(m *Map) error {
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now()
	}
	return db.Create(m).Error
}

func (db *DB) GetMap(id uint) (*Map, error) {
	var m Map
	if err := db.First(&m, id).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

func (db *DB) ListMaps() ([]Map, error) {
	var maps []Map
	if err := db.Order("created_at desc").Find(&maps).Error; err != nil {
		return nil, err
	}
	return maps, nil
}

func (db *DB) DeleteMap(id uint) error {
	db.Where("map_id = ?", id).Delete(&Cell{})
	db.Where("map_id = ?", id).Delete(&Wall{})
	db.Where("map_id = ?", id).Delete(&Marker{})
	return db.Delete(&Map{}, id).Error
}

// --- Cell CRUD ---

func (db *DB) UpsertCell(c *Cell) error {
	var existing Cell
	err := db.Where("map_id = ? AND x = ? AND y = ?", c.MapID, c.X, c.Y).First(&existing).Error
	if err == nil {
		c.ID = existing.ID
		return db.Save(c).Error
	}
	return db.Create(c).Error
}

func (db *DB) UpsertCells(cells []Cell) error {
	for i := range cells {
		if err := db.UpsertCell(&cells[i]); err != nil {
			return err
		}
	}
	return nil
}

func (db *DB) GetCell(mapID uint, x, y int) (*Cell, error) {
	var c Cell
	if err := db.Where("map_id = ? AND x = ? AND y = ?", mapID, x, y).First(&c).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

func (db *DB) ListCells(mapID uint) ([]Cell, error) {
	var cells []Cell
	if err := db.Where("map_id = ?", mapID).Find(&cells).Error; err != nil {
		return nil, err
	}
	return cells, nil
}

// NextRoomID returns the next available room_id for a map.
func (db *DB) NextRoomID(mapID uint) (uint, error) {
	var maxRoom *uint
	err := db.Model(&Cell{}).Where("map_id = ?", mapID).Select("MAX(room_id)").Scan(&maxRoom).Error
	if err != nil {
		return 0, err
	}
	if maxRoom == nil {
		return 1, nil
	}
	return *maxRoom + 1, nil
}

func (db *DB) DeleteCells(mapID uint, coords [][2]int) error {
	for _, c := range coords {
		if err := db.Where("map_id = ? AND x = ? AND y = ?", mapID, c[0], c[1]).Delete(&Cell{}).Error; err != nil {
			return err
		}
	}
	return nil
}

// DeleteCellsAndCleanWalls deletes cells and removes any walls where neither
// endpoint has a room cell anymore. Returns the walls that were cleaned up.
func (db *DB) DeleteCellsAndCleanWalls(mapID uint, coords [][2]int) ([]Wall, error) {
	if err := db.DeleteCells(mapID, coords); err != nil {
		return nil, err
	}

	// Find walls where neither endpoint has a room cell
	var orphaned []Wall
	err := db.Raw(`
		SELECT w.* FROM walls w
		WHERE w.map_id = ?
		AND NOT EXISTS (
			SELECT 1 FROM cells c
			WHERE c.map_id = w.map_id AND c.x = w.x1 AND c.y = w.y1 AND c.room_id IS NOT NULL
		)
		AND NOT EXISTS (
			SELECT 1 FROM cells c
			WHERE c.map_id = w.map_id AND c.x = w.x2 AND c.y = w.y2 AND c.room_id IS NOT NULL
		)
	`, mapID).Scan(&orphaned).Error
	if err != nil {
		return nil, err
	}

	if len(orphaned) > 0 {
		ids := make([]uint, len(orphaned))
		for i, w := range orphaned {
			ids[i] = w.ID
		}
		if err := db.Where("id IN ?", ids).Delete(&Wall{}).Error; err != nil {
			return nil, err
		}
	}

	return orphaned, nil
}

// --- Wall CRUD ---

func (db *DB) UpsertWall(w *Wall) error {
	// Normalize wall direction so (x1,y1) < (x2,y2)
	if w.X1 > w.X2 || (w.X1 == w.X2 && w.Y1 > w.Y2) {
		w.X1, w.Y1, w.X2, w.Y2 = w.X2, w.Y2, w.X1, w.Y1
	}
	var existing Wall
	err := db.Where("map_id = ? AND x1 = ? AND y1 = ? AND x2 = ? AND y2 = ?",
		w.MapID, w.X1, w.Y1, w.X2, w.Y2).First(&existing).Error
	if err == nil {
		w.ID = existing.ID
		return db.Save(w).Error
	}
	return db.Create(w).Error
}

func (db *DB) DeleteWall(mapID uint, x1, y1, x2, y2 int) error {
	// Normalize
	if x1 > x2 || (x1 == x2 && y1 > y2) {
		x1, y1, x2, y2 = x2, y2, x1, y1
	}
	return db.Where("map_id = ? AND x1 = ? AND y1 = ? AND x2 = ? AND y2 = ?",
		mapID, x1, y1, x2, y2).Delete(&Wall{}).Error
}

func (db *DB) ListWalls(mapID uint) ([]Wall, error) {
	var walls []Wall
	if err := db.Where("map_id = ?", mapID).Find(&walls).Error; err != nil {
		return nil, err
	}
	return walls, nil
}

// --- Marker CRUD ---

func (db *DB) UpsertMarker(m *Marker) error {
	var existing Marker
	err := db.Where("map_id = ? AND x = ? AND y = ?", m.MapID, m.X, m.Y).First(&existing).Error
	if err == nil {
		m.ID = existing.ID
		return db.Save(m).Error
	}
	return db.Create(m).Error
}

func (db *DB) DeleteMarker(mapID uint, x, y int) error {
	return db.Where("map_id = ? AND x = ? AND y = ?", mapID, x, y).Delete(&Marker{}).Error
}

func (db *DB) ListMarkers(mapID uint) ([]Marker, error) {
	var markers []Marker
	if err := db.Where("map_id = ?", mapID).Find(&markers).Error; err != nil {
		return nil, err
	}
	return markers, nil
}

// MapState contains the full state of a map for JSON serialization.
type MapState struct {
	Map     Map      `json:"map"`
	Cells   []Cell   `json:"cells"`
	Walls   []Wall   `json:"walls"`
	Markers []Marker `json:"markers"`
}

func (db *DB) GetMapState(id uint) (*MapState, error) {
	m, err := db.GetMap(id)
	if err != nil {
		return nil, err
	}
	cells, err := db.ListCells(id)
	if err != nil {
		return nil, err
	}
	walls, err := db.ListWalls(id)
	if err != nil {
		return nil, err
	}
	markers, err := db.ListMarkers(id)
	if err != nil {
		return nil, err
	}
	return &MapState{
		Map:     *m,
		Cells:   cells,
		Walls:   walls,
		Markers: markers,
	}, nil
}
