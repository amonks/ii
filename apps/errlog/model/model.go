package model

import (
	_ "embed"
	"fmt"

	"github.com/google/uuid"
	"monks.co/pkg/database"
	"monks.co/pkg/errlogger"
)

//go:embed schema.sql
var schema string

type DB struct {
	db *database.DB
}

func New() (*DB, error) {
	db, err := database.OpenFromDataFolder("errlog")
	if err != nil {
		return nil, err
	}

	if err := db.Exec(schema).Error; err != nil {
		return nil, fmt.Errorf("migration err: %w", err)
	}

	return &DB{db}, nil
}

type ErrorReport struct {
	UUID   string
	Report errlogger.ErrorReport `gorm:"embedded"`
}

type ErrorReportSearch struct {
	UUID    string
	App     string
	Machine string
	Report  string
}

func (ers *ErrorReportSearch) TableName() string {
	return `error_report_search`
}

func (db *DB) Capture(report *ErrorReport) error {
	if report.UUID == "" {
		report.UUID = uuid.NewString()
	}

	if err := db.db.
		Create(report).
		Error; err != nil {
		return fmt.Errorf("error inserting error report: %w", err)
	}

	if err := db.db.
		Create(&ErrorReportSearch{
			UUID: report.UUID,

			Machine: report.Report.Machine,
			App:     report.Report.App,
			Report:  report.Report.Report,
		}).
		Error; err != nil {
		return err
	}

	return nil
}

func (db *DB) LastN(n int, where ErrorReport) ([]ErrorReport, error) {
	var reports []ErrorReport
	if err := db.db.Table("error_reports").
		Where(where).
		Order("happened_at desc").
		Find(&reports).
		Error; err != nil {
		return nil, err
	}
	return reports, nil
}
