package traffic

import (
	"fmt"
	"log"
	"net"
	"time"

	"monks.co/pkg/color"
	"monks.co/pkg/database"
)

// Duration buckets in milliseconds (upper bounds, non-cumulative).
var durationBucketsMs = []int64{1, 2, 5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000}

func durationBucketMs(d time.Duration) int64 {
	ms := d.Milliseconds()
	for _, b := range durationBucketsMs {
		if ms <= b {
			return b
		}
	}
	return durationBucketsMs[len(durationBucketsMs)-1]
}

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

// ChartPoint is a single data point for the traffic chart.
type ChartPoint struct {
	Host          string    `json:"host"`
	WindowStartAt time.Time `json:"window_start_at"`
	Count         int64     `json:"count"`
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

		CREATE TABLE IF NOT EXISTS daily_stats (
			day DATETIME NOT NULL,
			host TEXT NOT NULL,
			status_code INTEGER NOT NULL,
			duration_bucket INTEGER NOT NULL,
			count INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (day, host, status_code, duration_bucket)
		);

		CREATE TABLE IF NOT EXISTS page_daily (
			day DATETIME NOT NULL,
			host TEXT NOT NULL,
			path TEXT NOT NULL,
			count INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (day, host, path)
		);

		DROP TABLE IF EXISTS traffic_aggregates;
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
		day := t.Truncate(24 * time.Hour)
		m.incrementDailyStat(e.Host, day, e.StatusCode, durationBucketMs(e.Duration))
		m.incrementPageDaily(e.Host, e.Path, day)
	}
	return nil
}

func (m *Model) incrementDailyStat(host string, day time.Time, statusCode int, durationBucket int64) {
	err := m.Exec(`
		INSERT INTO daily_stats (day, host, status_code, duration_bucket, count)
		VALUES (?, ?, ?, ?, 1)
		ON CONFLICT (day, host, status_code, duration_bucket)
		DO UPDATE SET count = count + 1
	`, day, host, statusCode, durationBucket).Error
	if err != nil {
		log.Printf("traffic: incrementDailyStat error: %v", err)
	}
}

func (m *Model) incrementPageDaily(host string, path string, day time.Time) {
	err := m.Exec(`
		INSERT INTO page_daily (day, host, path, count)
		VALUES (?, ?, ?, 1)
		ON CONFLICT (day, host, path)
		DO UPDATE SET count = count + 1
	`, day, host, path).Error
	if err != nil {
		log.Printf("traffic: incrementPageDaily error: %v", err)
	}
}

func (m *Model) GetChartData(tr TimeRange) (map[string][]ChartPoint, error) {
	days := tr.Days()

	var query string
	if days < 7 {
		// Hourly from raw requests (fast for small date ranges)
		query = `SELECT host,
			strftime('%Y-%m-%d %H:00:00+00:00', created_at) as window_start_at,
			count(*) as count
			FROM requests
			WHERE created_at >= ? AND created_at <= ?
			GROUP BY host, strftime('%Y-%m-%d %H', created_at)
			ORDER BY window_start_at ASC`
	} else if days >= 180 {
		// Weekly from daily_stats
		query = `SELECT host,
			strftime('%Y-%m-%d 00:00:00+00:00', day, '-' || ((cast(strftime('%w', day) as integer) + 6) % 7) || ' days') as window_start_at,
			SUM(count) as count
			FROM daily_stats
			WHERE day >= ? AND day <= ?
			GROUP BY host, window_start_at
			ORDER BY window_start_at ASC`
	} else {
		// Daily from daily_stats
		query = `SELECT host, day as window_start_at, SUM(count) as count
			FROM daily_stats
			WHERE day >= ? AND day <= ?
			GROUP BY host, day
			ORDER BY day ASC`
	}

	var points []ChartPoint
	if err := m.Raw(query, tr.StartTime(), tr.EndTime()).Scan(&points).Error; err != nil {
		return nil, err
	}

	result := make(map[string][]ChartPoint)
	for _, p := range points {
		result[p.Host] = append(result[p.Host], p)
	}
	return result, nil
}
