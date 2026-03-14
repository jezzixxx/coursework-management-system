package models

import "time"

type Comment struct {
	ID        uint   `gorm:"primaryKey"`
	ProjectID uint   `gorm:"not null;index"`
	AuthorID  uint   `gorm:"not null"`
	Text      string `gorm:"type:text;not null"`
	CreatedAt time.Time

	// Связь с автором
	Author User `gorm:"foreignKey:AuthorID"`
}
