package traffic

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"time"

	"monks.co/pkg/color"
	"monks.co/pkg/database"
)

type Request struct {
	ID        uint `gorm:"primarykey"`
	CreatedAt *time.Time
	UpdatedAt *time.Time

	Host  string
	Path  string
	Query string

	RemoteAddr string
	UserAgent  string
	Referer    string

	StatusCode int
	Duration   time.Duration
}

func (r *Request) PrintDate() string {
	return r.CreatedAt.Format("2006-01-02 15:04:05")
}

func (r *Request) PrintDuration() string {
	return fmt.Sprintf("%dµs", r.Duration.Microseconds())
}

func (r *Request) PrintURL() string {
	if r.Query == "" {
		return r.Host + r.Path
	}
	return r.Host + r.Path + "?" + r.Query
}

func (r *Request) PrintRemoteAddr() string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (r *Request) ColorRemoteAddr() string {
	return color.Hash(r.PrintRemoteAddr())
}

func (r *Request) PrintUserAgent() string {
	return r.UserAgent
}

type TrafficAggregate struct {
	Host           string        `gorm:"primaryKey" json:"host"`
	WindowDuration time.Duration `gorm:"primaryKey" json:"-"`
	WindowStartAt  time.Time     `gorm:"primaryKey" json:"window_start_at"`
	Count          int64         `json:"count"`
}

func (TrafficAggregate) TableName() string {
	return "traffic_aggregates"
}

type Model struct {
	*database.DB
}

func Open() (*Model, error) {
	db, err := database.OpenFromDataFolder("traffic")
	if err != nil {
		return nil, err
	}
	m := &Model{db}
	if err := m.migrate(); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *Model) migrate() error {
	sql := `
		CREATE INDEX IF NOT EXISTS idx_requests_created_at ON requests(created_at DESC);
		CREATE INDEX IF NOT EXISTS idx_requests_created_at_remote_addr ON requests(created_at, remote_addr);
		CREATE INDEX IF NOT EXISTS idx_requests_created_at_host_path ON requests(created_at, host, path);
		DROP INDEX IF EXISTS idx_requests_deleted_at;

		CREATE TABLE IF NOT EXISTS traffic_aggregates (
			host TEXT NOT NULL,
			window_duration INTEGER NOT NULL,
			window_start_at DATETIME NOT NULL,
			count INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (host, window_duration, window_start_at)
		);

		CREATE TABLE IF NOT EXISTS traffic_meta (
			key TEXT PRIMARY KEY,
			value TEXT
		);
	`
	return m.Exec(sql).Error
}

// LogEntry is the wire format for traffic log entries sent from the proxy.
type LogEntry struct {
	Timestamp  time.Time     `json:"timestamp"`
	Host       string        `json:"host"`
	Path       string        `json:"path"`
	Query      string        `json:"query"`
	RemoteAddr string        `json:"remote_addr"`
	UserAgent  string        `json:"user_agent"`
	Referer    string        `json:"referer"`
	StatusCode int           `json:"status_code"`
	Duration   time.Duration `json:"duration"`
}

func (m *Model) LogEntries(entries []LogEntry) error {
	for _, e := range entries {
		t := e.Timestamp
		r := &Request{
			CreatedAt:  &t,
			Host:       e.Host,
			Path:       e.Path,
			Query:      e.Query,
			RemoteAddr: e.RemoteAddr,
			UserAgent:  e.UserAgent,
			Referer:    e.Referer,
			StatusCode: e.StatusCode,
			Duration:   e.Duration,
		}
		if tx := m.Create(r); tx.Error != nil {
			return tx.Error
		}
		m.incrementAggregate(e.Host, t.Truncate(time.Hour), time.Hour)
		m.incrementAggregate(e.Host, t.Truncate(24*time.Hour), 24*time.Hour)
	}
	return nil
}

func (m *Model) incrementAggregate(host string, windowStart time.Time, windowDuration time.Duration) {
	err := m.Exec(`
		INSERT INTO traffic_aggregates (host, window_duration, window_start_at, count)
		VALUES (?, ?, ?, 1)
		ON CONFLICT (host, window_duration, window_start_at)
		DO UPDATE SET count = count + 1
	`, host, int64(windowDuration), windowStart).Error
	if err != nil {
		log.Printf("traffic: incrementAggregate error: %v", err)
	}
}

func (m *Model) BackfillAggregates() {
	// Check high-water mark
	var hwm string
	err := m.Raw("SELECT value FROM traffic_meta WHERE key = 'backfill_hwm'").Scan(&hwm).Error
	if err == nil && hwm == "done" {
		log.Println("traffic: backfill already complete")
		return
	}

	var lastID uint
	if hwm != "" {
		if v, err := strconv.ParseUint(hwm, 10, 64); err == nil {
			lastID = uint(v)
		}
	}

	log.Printf("traffic: starting backfill from id %d", lastID)

	for {
		var batch []struct {
			ID        uint
			Host      string
			CreatedAt time.Time
		}
		if err := m.Raw(
			"SELECT id, host, created_at FROM requests WHERE id > ? ORDER BY id ASC LIMIT 10000",
			lastID,
		).Scan(&batch).Error; err != nil {
			log.Printf("traffic: backfill query error: %v", err)
			return
		}
		if len(batch) == 0 {
			break
		}

		for _, r := range batch {
			m.incrementAggregate(r.Host, r.CreatedAt.Truncate(time.Hour), time.Hour)
			m.incrementAggregate(r.Host, r.CreatedAt.Truncate(24*time.Hour), 24*time.Hour)
		}

		lastID = batch[len(batch)-1].ID
		m.Exec("INSERT INTO traffic_meta (key, value) VALUES ('backfill_hwm', ?) ON CONFLICT (key) DO UPDATE SET value = ?",
			fmt.Sprintf("%d", lastID), fmt.Sprintf("%d", lastID))
		log.Printf("traffic: backfilled through id %d", lastID)
	}

	m.Exec("INSERT INTO traffic_meta (key, value) VALUES ('backfill_hwm', 'done') ON CONFLICT (key) DO UPDATE SET value = 'done'")
	log.Println("traffic: backfill complete")
}

func (m *Model) GetTrafficAggregates(tr TimeRange) (map[string][]TrafficAggregate, error) {
	windowDuration := int64(time.Hour)
	if tr.Days() > 30 {
		windowDuration = int64(24 * time.Hour)
	}

	var points []TrafficAggregate
	if err := m.Raw(
		"SELECT host, window_duration, window_start_at, count FROM traffic_aggregates WHERE window_start_at >= ? AND window_start_at <= ? AND window_duration = ? ORDER BY window_start_at ASC",
		tr.StartTime(), tr.EndTime(), windowDuration,
	).Scan(&points).Error; err != nil {
		return nil, err
	}

	result := make(map[string][]TrafficAggregate)
	for _, p := range points {
		result[p.Host] = append(result[p.Host], p)
	}
	return result, nil
}
