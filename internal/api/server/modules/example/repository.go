package example

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// Note is the stored row. Keep it separate from the generated contract type so a
// schema change does not silently reshape the API.
// The table is created by scripts/schema.sql, which compose runs on first start.
// Column shapes live in the SQL, not in tags: nothing here generates DDL.
type Note struct {
	ID        int64 `gorm:"primaryKey"`
	Title     string
	Body      string
	CreatedAt time.Time
}

func (Note) TableName() string { return "example_notes" }

// Repository is the only place that talks to the database; the handler never sees *gorm.DB.
type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// Create stores a note and fills in its generated ID and timestamp.
func (r *Repository) Create(ctx context.Context, note *Note) error {
	// WithContext carries the request deadline into the query, so a stalled database
	// cannot outlive the request. Every method below does the same.
	return r.db.WithContext(ctx).Create(note).Error
}

// List returns the newest notes first.
// For a write that spans several statements, wrap it with database.WithTransaction instead.
func (r *Repository) List(ctx context.Context, limit int) ([]Note, error) {
	var notes []Note
	err := r.db.WithContext(ctx).Order("id DESC").Limit(limit).Find(&notes).Error
	return notes, err
}
