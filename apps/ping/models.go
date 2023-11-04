package main

import (
	"errors"
	"fmt"
	"time"

	"monks.co/pkg/database"
)

type Person struct {
	Slug     string `gorm:"primaryKey"`
	IsActive bool
}

type Ping struct {
	PersonSlug string
	At         time.Time
	Notes      string
}

type AnnotatedPerson struct {
	Person
	LastPingAt        time.Time
	LastPingNotes     string
	IsLongestUnpinged bool
}

type model struct {
	*database.DB
}

func NewModel() (*model, error) {
	db, err := database.Open("ping.db")
	if err != nil {
		return nil, err
	}
	return &model{db}, nil
}

func (m *model) listPeople() ([]AnnotatedPerson, error) {
	people := []AnnotatedPerson{}
	if err := m.DB.Table("people").
		Select("people.slug, people.is_active, pings.at as last_ping_at, pings.notes as last_ping_notes, false as is_longest_unpinged").
		Joins("left join pings on people.slug = pings.person_slug and pings.at = (select max(at) from pings where person_slug = people.slug)").
		Scan(&people).Error; err != nil {
		return nil, fmt.Errorf("error finding people: %w", err)
	}

	oldestActivePing := time.Now()
	for _, person := range people {
		if person.IsActive && time.Time(person.LastPingAt).Before(oldestActivePing) {
			oldestActivePing = time.Time(person.LastPingAt)
		}
	}
	for i, person := range people {
		if person.IsActive && time.Time(person.LastPingAt) == oldestActivePing {
			people[i].IsLongestUnpinged = true
		}
	}

	return people, nil
}

func (m *model) addPerson(slug string) error {
	if err := m.DB.Create(Person{
		Slug:     slug,
		IsActive: true,
	}).Error; err != nil {
		return fmt.Errorf("error adding person: %w", err)
	}
	return nil
}

func (m *model) updatePerson(slug string, isActive bool) error {
	if err := m.DB.Save(Person{
		Slug:     slug,
		IsActive: isActive,
	}).Error; err != nil {
		return fmt.Errorf("error updating person: %w", err)
	}
	return nil
}

func (m *model) addPing(slug string, notes string) error {
	if err := m.DB.Create(Ping{
		PersonSlug: slug,
		Notes:      notes,
		At:         time.Now(),
	}).Error; err != nil {
		return fmt.Errorf("error inserting ping: %w", err)
	}
	return nil
}

func (m *model) showPerson(slug string) (AnnotatedPerson, []Ping, error) {
	person, err := m.getPerson(slug)
	if err != nil {
		return AnnotatedPerson{}, nil, err
	}

	pings, err := m.getPersonPings(slug)
	if err != nil {
		return AnnotatedPerson{}, nil, err
	}

	return person, pings, nil
}

func (m *model) getPerson(slug string) (AnnotatedPerson, error) {
	people, err := m.listPeople()
	if err != nil {
		return AnnotatedPerson{}, err
	}

	for _, pr := range people {
		if slug == pr.Slug {
			return pr, nil
		}
	}

	return AnnotatedPerson{}, errors.New("no such person")
}

func (m *model) getPersonPings(slug string) ([]Ping, error) {
	pings := []Ping{}
	if err := m.DB.Table("pings").
		Where("person_slug = ?", slug).
		Find(&pings).
		Error; err != nil {
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
