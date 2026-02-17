package logs

import (
	"encoding/json"
	"fmt"
	"log"
	"slices"
	"strings"
	"time"

	"monks.co/pkg/database"
)

type Model struct {
	*database.DB
}

func Open() (*Model, error) {
	db, err := database.OpenFromDataFolder("logs")
	if err != nil {
		return nil, err
	}
	m := &Model{db}
	if err := m.migrate(); err != nil {
		return nil, err
	}
	return m, nil
}

func OpenPath(path string) (*Model, error) {
	db, err := database.Open(path)
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
		CREATE TABLE IF NOT EXISTS events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp DATETIME NOT NULL,
			data TEXT NOT NULL,

			app           TEXT GENERATED ALWAYS AS (json_extract(data, '$."app.name"')) STORED,
			level         TEXT GENERATED ALWAYS AS (json_extract(data, '$."level"')) STORED,
			msg           TEXT GENERATED ALWAYS AS (json_extract(data, '$."msg"')) STORED,
			request_id    TEXT GENERATED ALWAYS AS (json_extract(data, '$."req.id"')) STORED,
			method        TEXT GENERATED ALWAYS AS (json_extract(data, '$."http.method"')) STORED,
			host          TEXT GENERATED ALWAYS AS (json_extract(data, '$."http.host"')) STORED,
			path          TEXT GENERATED ALWAYS AS (json_extract(data, '$."http.path"')) STORED,
			route         TEXT GENERATED ALWAYS AS (json_extract(data, '$."http.route"')) STORED,
			status        INTEGER GENERATED ALWAYS AS (json_extract(data, '$."http.status"')) STORED,
			duration_ms   REAL GENERATED ALWAYS AS (json_extract(data, '$."http.duration_ms"')) STORED,
			remote_addr   TEXT GENERATED ALWAYS AS (json_extract(data, '$."http.remote_addr"')) STORED,
			proxy_upstream TEXT GENERATED ALWAYS AS (json_extract(data, '$."proxy.upstream"')) STORED,

			duration_bucket INTEGER GENERATED ALWAYS AS (
				CASE
					WHEN json_extract(data, '$."http.duration_ms"') IS NULL THEN NULL
					WHEN json_extract(data, '$."http.duration_ms"') <=    1 THEN 1
					WHEN json_extract(data, '$."http.duration_ms"') <=    2 THEN 2
					WHEN json_extract(data, '$."http.duration_ms"') <=    5 THEN 5
					WHEN json_extract(data, '$."http.duration_ms"') <=   10 THEN 10
					WHEN json_extract(data, '$."http.duration_ms"') <=   25 THEN 25
					WHEN json_extract(data, '$."http.duration_ms"') <=   50 THEN 50
					WHEN json_extract(data, '$."http.duration_ms"') <=  100 THEN 100
					WHEN json_extract(data, '$."http.duration_ms"') <=  250 THEN 250
					WHEN json_extract(data, '$."http.duration_ms"') <=  500 THEN 500
					WHEN json_extract(data, '$."http.duration_ms"') <= 1000 THEN 1000
					WHEN json_extract(data, '$."http.duration_ms"') <= 2500 THEN 2500
					WHEN json_extract(data, '$."http.duration_ms"') <= 5000 THEN 5000
					ELSE 10000
				END
			) STORED
		);

		CREATE INDEX IF NOT EXISTS idx_events_timestamp ON events(timestamp DESC);
		CREATE INDEX IF NOT EXISTS idx_events_request_id ON events(request_id) WHERE request_id IS NOT NULL;
		CREATE INDEX IF NOT EXISTS idx_events_app_timestamp ON events(app, timestamp);
		CREATE INDEX IF NOT EXISTS idx_events_msg ON events(msg);
		CREATE INDEX IF NOT EXISTS idx_events_msg_timestamp ON events(msg, timestamp DESC);
	`).Error; err != nil {
		return err
	}

	return m.Exec(`
		CREATE TABLE IF NOT EXISTS daily_stats (
			day DATETIME NOT NULL,
			app TEXT NOT NULL DEFAULT '',
			host TEXT NOT NULL DEFAULT '',
			method TEXT DEFAULT 'unknown',
			status INTEGER NOT NULL DEFAULT 0,
			duration_bucket INTEGER NOT NULL DEFAULT 0,
			count INTEGER DEFAULT 0,
			PRIMARY KEY (day, app, host, method, status, duration_bucket)
		);

		CREATE TABLE IF NOT EXISTS page_daily (
			day DATETIME NOT NULL,
			host TEXT NOT NULL,
			path TEXT NOT NULL,
			count INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (day, host, path)
		);

		DROP INDEX IF EXISTS idx_page_daily_host_path_day;
		CREATE INDEX IF NOT EXISTS idx_page_daily_day_count ON page_daily(day, host, path, count);
		CREATE INDEX IF NOT EXISTS idx_daily_stats_day_status ON daily_stats(day, status, count);
		CREATE INDEX IF NOT EXISTS idx_daily_stats_day_duration ON daily_stats(day, duration_bucket, count);
	`).Error
}

// JSONText is a json.RawMessage that scans from a SQL string column.
type JSONText json.RawMessage

func (j *JSONText) Scan(value any) error {
	switch v := value.(type) {
	case string:
		*j = JSONText(v)
	case []byte:
		*j = JSONText(v)
	default:
		return fmt.Errorf("JSONText.Scan: unsupported type %T", value)
	}
	return nil
}

func (j JSONText) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return []byte("null"), nil
	}
	return []byte(j), nil
}

// Event is a single log event with its parsed fields.
type Event struct {
	ID        int64     `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Data      JSONText  `json:"data"`

	App        *string  `json:"app"`
	Level      *string  `json:"level"`
	Msg        *string  `json:"msg"`
	RequestID  *string  `json:"request_id"`
	Method     *string  `json:"method"`
	Host       *string  `json:"host"`
	Path       *string  `json:"path"`
	Status     *int     `json:"status"`
	DurationMs *float64 `json:"duration_ms"`
	RemoteAddr *string  `json:"remote_addr"`
}

// ChartPoint is a single data point for the chart.
type ChartPoint struct {
	Host          string `json:"host"`
	WindowStartAt string `json:"window_start_at"`
	Count         int64  `json:"count"`
}

// IngestTx wraps Ingest in a database transaction for bulk performance.
func (m *Model) IngestTx(events []json.RawMessage) error {
	tx := m.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	txModel := &Model{&database.DB{DB: tx}}
	if err := txModel.Ingest(events); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

// Ingest stores a batch of raw JSON events.
func (m *Model) Ingest(events []json.RawMessage) error {
	for _, raw := range events {
		// Parse timestamp from JSON.
		var envelope struct {
			Time time.Time `json:"time"`
		}
		if err := json.Unmarshal(raw, &envelope); err != nil {
			log.Printf("logs: skipping event with unparseable timestamp: %v", err)
			continue
		}
		if envelope.Time.IsZero() {
			envelope.Time = time.Now()
		}

		if err := m.Exec(`INSERT INTO events (timestamp, data) VALUES (?, ?)`,
			envelope.Time, string(raw)).Error; err != nil {
			return err
		}

		// Update aggregation tables using fields from the JSON.
		var fields struct {
			App        string  `json:"app.name"`
			Host       string  `json:"http.host"`
			Path       string  `json:"http.path"`
			Method     string  `json:"http.method"`
			Status     int     `json:"http.status"`
			DurationMs float64 `json:"http.duration_ms"`
			Msg        string  `json:"msg"`
		}
		if err := json.Unmarshal(raw, &fields); err != nil {
			continue
		}

		// Only aggregate HTTP request events.
		if fields.Msg != "request" {
			continue
		}

		day := envelope.Time.Truncate(24 * time.Hour)
		host := strings.ToLower(fields.Host)
		method := fields.Method
		if method == "" {
			method = "unknown"
		}
		bucket := durationBucketMs(fields.DurationMs)

		m.incrementDailyStat(day, fields.App, host, method, fields.Status, bucket)
		if fields.Path != "" {
			m.incrementPageDaily(day, host, fields.Path)
		}
	}
	return nil
}

var durationBucketsMs = []int64{1, 2, 5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000}

func durationBucketMs(ms float64) int64 {
	for _, b := range durationBucketsMs {
		if ms <= float64(b) {
			return b
		}
	}
	return durationBucketsMs[len(durationBucketsMs)-1]
}

func (m *Model) incrementDailyStat(day time.Time, app, host, method string, status int, durationBucket int64) {
	err := m.Exec(`
		INSERT INTO daily_stats (day, app, host, method, status, duration_bucket, count)
		VALUES (?, ?, ?, ?, ?, ?, 1)
		ON CONFLICT (day, app, host, method, status, duration_bucket)
		DO UPDATE SET count = count + 1
	`, day, app, host, method, status, durationBucket).Error
	if err != nil {
		log.Printf("logs: incrementDailyStat error: %v", err)
	}
}

func (m *Model) incrementPageDaily(day time.Time, host, path string) {
	err := m.Exec(`
		INSERT INTO page_daily (day, host, path, count)
		VALUES (?, ?, ?, 1)
		ON CONFLICT (day, host, path)
		DO UPDATE SET count = count + 1
	`, day, host, path).Error
	if err != nil {
		log.Printf("logs: incrementPageDaily error: %v", err)
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
	GroupBy string
	Filters []Filter
}

// validColumns lists whitelisted group-by and filter columns.
var validColumns = []string{"app", "proxy_upstream", "level", "msg", "host", "method", "status", "duration_bucket", "route"}

func isValidColumn(col string) bool {
	return slices.Contains(validColumns, col)
}

func (f Filter) buildSQL(colExpr string) (string, []any) {
	if len(f.Values) == 0 {
		return "", nil
	}
	if len(f.Values) == 1 {
		op := "="
		if f.Negate {
			op = "!="
		}
		return fmt.Sprintf(` AND %s %s ?`, colExpr, op), []any{f.Values[0]}
	}
	placeholders := strings.Repeat("?,", len(f.Values))
	placeholders = placeholders[:len(placeholders)-1]
	op := "IN"
	if f.Negate {
		op = "NOT IN"
	}
	args := make([]any, len(f.Values))
	for i, v := range f.Values {
		args[i] = v
	}
	return fmt.Sprintf(` AND %s %s (%s)`, colExpr, op, placeholders), args
}

// ParseQuery parses a wire-format query string like "group:host,host:monks.co".
func ParseQuery(s string) Query {
	var q Query
	for part := range strings.SplitSeq(s, ",") {
		kv := strings.SplitN(part, ":", 2)
		if len(kv) != 2 {
			continue
		}
		k, v := kv[0], kv[1]
		switch k {
		case "source":
			// Ignored for backwards compatibility.
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

	var sqlStr string
	var args []any

	if days < 7 {
		// Use raw events table for short ranges.
		groupCol := q.GroupBy
		sqlStr = fmt.Sprintf(`SELECT CAST(%s AS TEXT) as host,
			strftime('%%Y-%%m-%%d %%H:00:00+00:00', timestamp) as window_start_at,
			count(*) as count
			FROM events
			WHERE msg = 'request' AND timestamp >= ? AND timestamp <= ?`, groupCol)
		args = append(args, tr.StartTime(), tr.EndTime())
		for _, f := range q.Filters {
			s, a := f.buildSQL(f.Column)
			sqlStr += s
			args = append(args, a...)
		}
		sqlStr += fmt.Sprintf(` GROUP BY CAST(%s AS TEXT), strftime('%%Y-%%m-%%d %%H', timestamp)
			ORDER BY window_start_at ASC`, groupCol)
	} else {
		dayCol := "day"
		groupCol := q.GroupBy

		// Map event columns to daily_stats columns.
		statsCol := groupCol
		switch groupCol {
		case "proxy_upstream", "level", "route":
			// These don't exist in daily_stats; fall back to events table.
			return m.queryChartDataFromEvents(tr, q)
		}

		var timeExpr string
		if days >= 180 {
			timeExpr = fmt.Sprintf(`strftime('%%Y-%%m-%%dT00:00:00Z', %s, '-' || ((cast(strftime('%%w', %s) as integer) + 6) %% 7) || ' days')`, dayCol, dayCol)
		} else {
			timeExpr = fmt.Sprintf(`strftime('%%Y-%%m-%%dT00:00:00Z', %s)`, dayCol)
		}

		sqlStr = fmt.Sprintf(`SELECT CAST(%s AS TEXT) as host, %s as window_start_at, SUM(count) as count
			FROM daily_stats
			WHERE %s >= ? AND %s <= ?`, statsCol, timeExpr, dayCol, dayCol)
		args = append(args, tr.StartTime(), tr.EndTime())
		for _, f := range q.Filters {
			s, a := f.buildSQL(f.Column)
			sqlStr += s
			args = append(args, a...)
		}
		sqlStr += fmt.Sprintf(` GROUP BY CAST(%s AS TEXT), window_start_at
			ORDER BY window_start_at ASC`, statsCol)
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

// queryChartDataFromEvents handles long-range queries for dimensions not in daily_stats.
func (m *Model) queryChartDataFromEvents(tr TimeRange, q Query) (map[string][]ChartPoint, error) {
	days := tr.Days()
	groupCol := q.GroupBy

	var timeExpr string
	if days >= 180 {
		timeExpr = `strftime('%Y-%m-%dT00:00:00Z', timestamp, '-' || ((cast(strftime('%w', timestamp) as integer) + 6) % 7) || ' days')`
	} else {
		timeExpr = `strftime('%Y-%m-%dT00:00:00Z', timestamp)`
	}

	var sqlStr strings.Builder
	sqlStr.WriteString(fmt.Sprintf(`SELECT CAST(%s AS TEXT) as host, %s as window_start_at, count(*) as count
		FROM events
		WHERE msg = 'request' AND timestamp >= ? AND timestamp <= ?`, groupCol, timeExpr))
	args := []any{tr.StartTime(), tr.EndTime()}
	for _, f := range q.Filters {
		s, a := f.buildSQL(f.Column)
		sqlStr.WriteString(s)
		args = append(args, a...)
	}
	sqlStr.WriteString(fmt.Sprintf(` GROUP BY CAST(%s AS TEXT), window_start_at
		ORDER BY window_start_at ASC`, groupCol))

	var points []ChartPoint
	if err := m.Raw(sqlStr.String(), args...).Scan(&points).Error; err != nil {
		return nil, err
	}

	result := make(map[string][]ChartPoint)
	for _, p := range points {
		result[p.Host] = append(result[p.Host], p)
	}
	return result, nil
}

// GetDimensionValues returns distinct values for a dimension within a time range.
func (m *Model) GetDimensionValues(tr TimeRange, dim string) ([]string, error) {
	if !isValidColumn(dim) {
		return nil, fmt.Errorf("invalid dimension %q", dim)
	}

	days := tr.Days()
	var sqlStr string

	// Dimensions only available in events table.
	eventsOnly := dim == "proxy_upstream" || dim == "level" || dim == "route"

	if days < 7 || eventsOnly {
		sqlStr = fmt.Sprintf(`SELECT DISTINCT CAST(%s AS TEXT) as val FROM events WHERE msg = 'request' AND timestamp >= ? AND timestamp <= ? AND %s IS NOT NULL ORDER BY val`, dim, dim)
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

// GetTrace returns all events with a given request ID, ordered by timestamp.
func (m *Model) GetTrace(requestID string) ([]Event, error) {
	var events []Event
	if err := m.Raw(`
		SELECT id, timestamp, data, app, level, msg, request_id, method, host, path, status, duration_ms, remote_addr
		FROM events
		WHERE request_id = ?
		ORDER BY timestamp ASC
	`, requestID).Scan(&events).Error; err != nil {
		return nil, err
	}
	return events, nil
}

// GetRecentEvents returns recent request events within a time range.
func (m *Model) GetRecentEvents(tr TimeRange, limit int) ([]Event, error) {
	var events []Event
	if err := m.Raw(`
		SELECT id, timestamp, data, app, level, msg, request_id, method, host, path, status, duration_ms, remote_addr
		FROM events
		WHERE msg = 'request' AND timestamp >= ? AND timestamp <= ?
		ORDER BY timestamp DESC
		LIMIT ?
	`, tr.StartTime(), tr.EndTime(), limit).Scan(&events).Error; err != nil {
		return nil, err
	}
	return events, nil
}

// GetFilteredEvents returns paginated events matching the given query filters.
func (m *Model) GetFilteredEvents(tr TimeRange, q Query, limit, offset int) ([]Event, int, error) {
	for _, f := range q.Filters {
		if !isValidColumn(f.Column) {
			return nil, 0, fmt.Errorf("invalid filter column %q", f.Column)
		}
	}

	// Default to msg=request unless an explicit msg filter is provided.
	hasMsgFilter := false
	for _, f := range q.Filters {
		if f.Column == "msg" {
			hasMsgFilter = true
			break
		}
	}

	where := `WHERE timestamp >= ? AND timestamp <= ?`
	args := []any{tr.StartTime(), tr.EndTime()}
	if !hasMsgFilter {
		where += ` AND msg = 'request'`
	}
	for _, f := range q.Filters {
		s, a := f.buildSQL(f.Column)
		where += s
		args = append(args, a...)
	}

	// Get total count.
	var total int
	countSQL := `SELECT COUNT(*) FROM events ` + where
	row := m.Raw(countSQL, args...).Row()
	if err := row.Scan(&total); err != nil {
		return nil, 0, err
	}

	// Get page of events.
	dataSQL := `SELECT id, timestamp, data, app, level, msg, request_id, method, host, path, status, duration_ms, remote_addr
		FROM events ` + where + ` ORDER BY timestamp DESC LIMIT ? OFFSET ?`
	dataArgs := append(append([]any{}, args...), limit, offset)

	var events []Event
	if err := m.Raw(dataSQL, dataArgs...).Scan(&events).Error; err != nil {
		return nil, 0, err
	}
	return events, total, nil
}
