package traffic

import (
	"fmt"
	"log"
	"net"
	"strings"
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

// knownApps is the set of recognized app names (first path component).
// Requests whose first path segment isn't in this set get app="other".
var knownApps = map[string]bool{
	"air": true, "aranet": true, "calendar": true, "directory": true,
	"dogs": true, "errlog": true, "golink": true, "homepage": true,
	"mailer": true, "mailrules": true, "map": true, "monitor": true,
	"movies": true, "ping": true, "proxy": true, "reddit": true,
	"scrobbles": true, "sms": true, "template": true, "traffic": true,
	"writing": true, "youtube": true,
}

// appFromPath extracts the first path component and returns it if it's
// a known app, otherwise "other".
func appFromPath(path string) string {
	p := strings.TrimPrefix(path, "/")
	if idx := strings.IndexByte(p, '/'); idx >= 0 {
		p = p[:idx]
	}
	if knownApps[p] {
		return p
	}
	return "other"
}

// appExpr returns a SQL expression that extracts the app name from a path column,
// returning the first path component if it's a known app, otherwise 'other'.
func appExpr(pathCol string) string {
	seg := fmt.Sprintf(`SUBSTR(%s, 2, INSTR(SUBSTR(%s, 2) || '/', '/') - 1)`, pathCol, pathCol)
	var apps []string
	for app := range knownApps {
		apps = append(apps, "'"+app+"'")
	}
	return fmt.Sprintf(`CASE WHEN %s IN (%s) THEN %s ELSE 'other' END`,
		seg, strings.Join(apps, ","), seg)
}

type Request struct {
	ID        uint `gorm:"primarykey"`
	CreatedAt *time.Time
	UpdatedAt *time.Time

	Host   string
	Path   string
	Query  string
	Method string

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
	Host          string `json:"host"`
	WindowStartAt string `json:"window_start_at"`
	Count         int64  `json:"count"`
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
	if err := m.Exec(`
		CREATE INDEX IF NOT EXISTS idx_requests_created_at ON requests(created_at DESC);
		CREATE INDEX IF NOT EXISTS idx_requests_created_at_remote_addr ON requests(created_at, remote_addr);
		CREATE INDEX IF NOT EXISTS idx_requests_created_at_host_path ON requests(created_at, host, path);
		DROP INDEX IF EXISTS idx_requests_deleted_at;
	`).Error; err != nil {
		return err
	}

	return m.Exec(`
		CREATE TABLE IF NOT EXISTS daily_stats (
			day DATETIME NOT NULL,
			host TEXT NOT NULL,
			app TEXT NOT NULL DEFAULT '',
			method TEXT NOT NULL DEFAULT 'unknown',
			status_code INTEGER NOT NULL,
			duration_bucket INTEGER NOT NULL,
			count INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (day, host, app, method, status_code, duration_bucket)
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
	`).Error
}

// LogEntry is the wire format for traffic log entries sent from the proxy.
type LogEntry struct {
	Timestamp  time.Time     `json:"timestamp"`
	Host       string        `json:"host"`
	Path       string        `json:"path"`
	Query      string        `json:"query"`
	Method     string        `json:"method"`
	RemoteAddr string        `json:"remote_addr"`
	UserAgent  string        `json:"user_agent"`
	Referer    string        `json:"referer"`
	StatusCode int           `json:"status_code"`
	Duration   time.Duration `json:"duration"`
	App        string        `json:"app"`
}

func (m *Model) LogEntries(entries []LogEntry) error {
	for _, e := range entries {
		e.Host = strings.ToLower(e.Host)
		t := e.Timestamp
		method := e.Method
		if method == "" {
			method = "unknown"
		}
		r := &Request{
			CreatedAt:  &t,
			Host:       e.Host,
			Path:       e.Path,
			Query:      e.Query,
			Method:     method,
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
		app := e.App
		if app == "" {
			app = appFromPath(e.Path)
		}
		m.incrementDailyStat(e.Host, app, method, day, e.StatusCode, durationBucketMs(e.Duration))
		m.incrementPageDaily(e.Host, e.Path, day)
	}
	return nil
}

func (m *Model) incrementDailyStat(host, app, method string, day time.Time, statusCode int, durationBucket int64) {
	err := m.Exec(`
		INSERT INTO daily_stats (day, host, app, method, status_code, duration_bucket, count)
		VALUES (?, ?, ?, ?, ?, ?, 1)
		ON CONFLICT (day, host, app, method, status_code, duration_bucket)
		DO UPDATE SET count = count + 1
	`, day, host, app, method, statusCode, durationBucket).Error
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

// Filter describes a single filter condition.
type Filter struct {
	Column string   `json:"Column"`
	Negate bool     `json:"Negate"`
	Values []string `json:"Values"`
}

// Query describes a chart query: how to group, and optional filters.
type Query struct {
	GroupBy string   // column to group by
	Filters []Filter // column filters
}

// buildSQL returns a SQL clause fragment and args for this filter.
// colExpr is the SQL expression to filter on (may differ from f.Column, e.g. for duration_bucket).
func (f Filter) buildSQL(colExpr string) (string, []interface{}) {
	if len(f.Values) == 0 {
		return "", nil
	}
	if len(f.Values) == 1 {
		op := "="
		if f.Negate {
			op = "!="
		}
		return fmt.Sprintf(` AND %s %s ?`, colExpr, op), []interface{}{f.Values[0]}
	}
	placeholders := strings.Repeat("?,", len(f.Values))
	placeholders = placeholders[:len(placeholders)-1]
	op := "IN"
	if f.Negate {
		op = "NOT IN"
	}
	args := make([]interface{}, len(f.Values))
	for i, v := range f.Values {
		args[i] = v
	}
	return fmt.Sprintf(` AND %s %s (%s)`, colExpr, op, placeholders), args
}

// validColumns lists whitelisted group-by and filter columns.
var validColumns = []string{"host", "status_code", "duration_bucket", "app", "method"}

func isValidColumn(col string) bool {
	for _, c := range validColumns {
		if c == col {
			return true
		}
	}
	return false
}

// ParseQuery parses a wire-format query string like "source:stats,group:host,host:monks.co".
// Filters support negation with a "!" prefix on the key and multi-value with "|" in the value:
//
//	host:monks.co          → host = monks.co
//	!host:monks.co         → host != monks.co
//	host:monks.co|foo.com  → host IN (monks.co, foo.com)
//	!host:monks.co|foo.com → host NOT IN (monks.co, foo.com)
func ParseQuery(s string) Query {
	var q Query
	for _, part := range strings.Split(s, ",") {
		kv := strings.SplitN(part, ":", 2)
		if len(kv) != 2 {
			continue
		}
		k, v := kv[0], kv[1]
		switch k {
		case "source":
			// Ignored for backwards compatibility with old URLs.
		case "group":
			q.GroupBy = v
		default:
			negate := false
			col := k
			if strings.HasPrefix(k, "!") {
				negate = true
				col = k[1:]
			}
			values := strings.Split(v, "|")
			q.Filters = append(q.Filters, Filter{Column: col, Negate: negate, Values: values})
		}
	}
	if q.GroupBy == "" {
		q.GroupBy = "host"
	}
	return q
}

// FormatQuery serializes a Query back to the wire format.
func (q Query) FormatQuery() string {
	parts := []string{"group:" + q.GroupBy}
	for _, f := range q.Filters {
		key := f.Column
		if f.Negate {
			key = "!" + key
		}
		parts = append(parts, key+":"+strings.Join(f.Values, "|"))
	}
	return strings.Join(parts, ",")
}

// QueryChartData runs a query and returns chart points grouped by the group-by column.
func (m *Model) QueryChartData(tr TimeRange, q Query) (map[string][]ChartPoint, error) {
	if !isValidColumn(q.GroupBy) {
		return nil, fmt.Errorf("invalid group_by %q", q.GroupBy)
	}
	for _, f := range q.Filters {
		if !isValidColumn(f.Column) {
			return nil, fmt.Errorf("invalid filter column %q", f.Column)
		}
	}

	days := tr.Days()

	// Build the query depending on time range.
	var sqlStr string
	var args []interface{}

	if days < 7 {
		// Use raw requests table for short ranges.
		groupCol := q.GroupBy
		if q.GroupBy == "duration_bucket" {
			groupCol = durationBucketExpr()
		}
		if q.GroupBy == "app" {
			groupCol = appExpr("path")
		}
		sqlStr = fmt.Sprintf(`SELECT %s as host,
			strftime('%%Y-%%m-%%d %%H:00:00+00:00', created_at) as window_start_at,
			count(*) as count
			FROM requests
			WHERE created_at >= ? AND created_at <= ?`, groupCol)
		args = append(args, tr.StartTime(), tr.EndTime())
		for _, f := range q.Filters {
			filterCol := f.Column
			if f.Column == "duration_bucket" {
				filterCol = durationBucketExpr()
			}
			if f.Column == "app" {
				filterCol = appExpr("path")
			}
			s, a := f.buildSQL(filterCol)
			sqlStr += s
			args = append(args, a...)
		}
		sqlStr += fmt.Sprintf(` GROUP BY %s, strftime('%%Y-%%m-%%d %%H', created_at)
			ORDER BY window_start_at ASC`, groupCol)
	} else {
		dayCol := "day"
		groupCol := q.GroupBy

		var timeExpr string
		if days >= 180 {
			timeExpr = fmt.Sprintf(`strftime('%%Y-%%m-%%dT00:00:00Z', %s, '-' || ((cast(strftime('%%w', %s) as integer) + 6) %% 7) || ' days')`, dayCol, dayCol)
		} else {
			timeExpr = dayCol
		}

		sqlStr = fmt.Sprintf(`SELECT %s as host, %s as window_start_at, SUM(count) as count
			FROM daily_stats
			WHERE %s >= ? AND %s <= ?`, groupCol, timeExpr, dayCol, dayCol)
		args = append(args, tr.StartTime(), tr.EndTime())
		for _, f := range q.Filters {
			s, a := f.buildSQL(f.Column)
			sqlStr += s
			args = append(args, a...)
		}
		sqlStr += fmt.Sprintf(` GROUP BY %s, window_start_at
			ORDER BY window_start_at ASC`, groupCol)
	}

	var points []ChartPoint
	if err := m.Raw(sqlStr, args...).Scan(&points).Error; err != nil {
		return nil, err
	}

	result := make(map[string][]ChartPoint)
	for _, p := range points {
		result[p.Host] = append(result[p.Host], p)
	}
	return result, nil
}

// durationBucketExpr returns a SQL CASE expression that maps raw duration (nanoseconds) to bucket labels.
func durationBucketExpr() string {
	// duration in requests table is stored as nanoseconds
	return `CASE
		WHEN duration <= 1000000 THEN '1'
		WHEN duration <= 2000000 THEN '2'
		WHEN duration <= 5000000 THEN '5'
		WHEN duration <= 10000000 THEN '10'
		WHEN duration <= 25000000 THEN '25'
		WHEN duration <= 50000000 THEN '50'
		WHEN duration <= 100000000 THEN '100'
		WHEN duration <= 250000000 THEN '250'
		WHEN duration <= 500000000 THEN '500'
		WHEN duration <= 1000000000 THEN '1000'
		WHEN duration <= 2500000000 THEN '2500'
		WHEN duration <= 5000000000 THEN '5000'
		ELSE '10000'
	END`
}

// GetDimensionValues returns distinct values for a dimension within a time range.
func (m *Model) GetDimensionValues(tr TimeRange, dim string) ([]string, error) {
	if !isValidColumn(dim) {
		return nil, fmt.Errorf("invalid dimension %q", dim)
	}

	days := tr.Days()
	var sqlStr string
	if days < 7 {
		col := dim
		if dim == "duration_bucket" {
			col = durationBucketExpr()
		}
		if dim == "app" {
			col = appExpr("path")
		}
		sqlStr = fmt.Sprintf(`SELECT DISTINCT CAST(%s AS TEXT) as val FROM requests WHERE created_at >= ? AND created_at <= ? ORDER BY val`, col)
	} else {
		sqlStr = fmt.Sprintf(`SELECT DISTINCT CAST(%s AS TEXT) as val FROM daily_stats WHERE day >= ? AND day <= ? ORDER BY val`, dim)
	}

	var vals []string
	rows, err := m.Raw(sqlStr, tr.StartTime(), tr.EndTime()).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		vals = append(vals, v)
	}
	return vals, nil
}
