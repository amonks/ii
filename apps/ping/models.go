package main

import (
	"errors"
	"time"

	"crawshaw.io/sqlite"
	"crawshaw.io/sqlite/sqlitex"
)

type Person struct {
	Slug              string
	IsLongestUnpinged bool
	IsActive          bool
	LastPingAt        string
	LastPingAtMS      int64
	LastPingNotes     string
}

type Ping struct {
	PersonSlug string
	At         string
	Notes      string
}

func listPeople(conn *sqlite.Conn) ([]Person, error) {
	people := []Person{}
	const query = `
		select
			p.slug,
			p.is_active,
			ping.at_ms as last_pinged_at_ms,
			ping.notes as last_ping_notes
		from people p
		left join pings ping
			on p.slug = ping.person_slug
			and ping.at_ms = (select max(at_ms) from pings where person_slug = p.slug)
		order by last_pinged_at_ms asc, slug asc;`

	oldestActivePing := time.Now().UnixMilli()

	onResult := func(stmt *sqlite.Stmt) error {
		isActive := stmt.ColumnInt(1) == 1
		pingAtMs := stmt.ColumnInt64(2)
		if isActive && pingAtMs <= oldestActivePing {
			oldestActivePing = pingAtMs
		}

		people = append(people, Person{
			Slug:          stmt.ColumnText(0),
			IsActive:      isActive,
			LastPingAt:    formatTimeMs(pingAtMs),
			LastPingAtMS:  pingAtMs,
			LastPingNotes: stmt.ColumnText(3),
		})
		return nil
	}
	if err := sqlitex.Exec(conn, query, onResult); err != nil {
		return nil, err
	}

	for i, person := range people {
		if person.IsActive && person.LastPingAtMS == oldestActivePing {
			people[i].IsLongestUnpinged = true
		}
	}

	return people, nil
}

func addPerson(conn *sqlite.Conn, slug string) error {
	const query = `
		insert into people
			(slug, is_active)
		values
			(?, 1)`

	return sqlitex.Exec(conn, query, nil, slug)
}

func updatePerson(conn *sqlite.Conn, slug string, isActive bool) error {
	const query = `
		update people
		set is_active = ?
		where slug = ?`

	setter := 0
	if isActive {
		setter = 1
	}

	return sqlitex.Exec(conn, query, nil, setter, slug)
}

func addPing(conn *sqlite.Conn, slug string, notes string) error {
	const query = `
		insert into pings
			(person_slug, at_ms, notes)
		values
			(?, ?, ?)`
	return sqlitex.Exec(conn, query, nil, slug, time.Now().UnixMilli(), notes)
}

func showPerson(conn *sqlite.Conn, slug string) (Person, []Ping, error) {
	person, err := getPerson(conn, slug)
	if err != nil {
		return Person{}, nil, err
	}

	pings, err := getPersonPings(conn, slug)
	if err != nil {
		return Person{}, nil, err
	}

	return person, pings, nil
}

func getPerson(conn *sqlite.Conn, slug string) (Person, error) {
	people, err := listPeople(conn)
	if err != nil {
		return Person{}, err
	}

	for _, pr := range people {
		if slug == pr.Slug {
			return pr, nil
		}
	}

	return Person{}, errors.New("no such person")
}

func getPersonPings(conn *sqlite.Conn, slug string) ([]Ping, error) {
	pings := []Ping{}
	const query = `
		select
			at_ms,
			notes
		from pings
		where person_slug = ?
		order by at_ms desc;`
	onResult := func(stmt *sqlite.Stmt) error {
		pings = append(pings, Ping{
			At:    formatTimeMs(stmt.ColumnInt64(0)),
			Notes: stmt.ColumnText(1),
		})
		return nil
	}
	if err := sqlitex.Exec(conn, query, onResult, slug); err != nil {
		return nil, err
	}

	return pings, nil
}

func formatTimeMs(at int64) string {
	formatted := "never"
	if at > 0 {
		formatted = time.UnixMilli(at).Format("2006-01-02")
	}
	return formatted
}
