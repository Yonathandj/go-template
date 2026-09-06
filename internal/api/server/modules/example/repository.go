package example

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type Note struct {
	ID        int64 `gorm:"primaryKey"`
	Title     string
	Body      string
	CreatedAt time.Time
}

func (Note) TableName() string { return "example_notes" }

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, note *Note) error {
	return r.db.WithContext(ctx).Create(note).Error
}

func (r *Repository) List(ctx context.Context, limit int) ([]Note, error) {
	var notes []Note
	err := r.db.WithContext(ctx).Order("id DESC").Limit(limit).Find(&notes).Error
	return notes, err
}
