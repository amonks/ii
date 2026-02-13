package traffic

import (
	"fmt"
	"log"
	"net"
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

		DROP TABLE IF EXISTS traffic_meta;
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
		m.incrementAggregate(e.Host, weekStart(t), 7*24*time.Hour)
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

// weekStart returns the Monday 00:00 UTC at the start of t's ISO week.
func weekStart(t time.Time) time.Time {
	t = t.UTC().Truncate(24 * time.Hour)
	offset := (int(t.Weekday()) + 6) % 7 // Monday=0 … Sunday=6
	return t.AddDate(0, 0, -offset)
}

func (m *Model) GetTrafficAggregates(tr TimeRange) (map[string][]TrafficAggregate, error) {
	windowDuration := int64(time.Hour)
	days := tr.Days()
	if days >= 180 {
		windowDuration = int64(7 * 24 * time.Hour)
	} else if days >= 7 {
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
