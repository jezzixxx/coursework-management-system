package models

import "time"

type Project struct {
	ID            uint   `gorm:"primaryKey"`
	Title         string `gorm:"not null;size:200"`
	Description   string `gorm:"type:text"`
	FolderPath    string `gorm:"size:255"`      // Путь к папке с файлами
	CurrentStatus string `gorm:"default:'new'"` // new, in_progress, review, approved, rejected
	CreatedAt     time.Time
	UpdatedAt     time.Time

	// Связь многие-ко-многим с пользователями
	Members []User `gorm:"many2many:project_members;"`

	// Связь один-ко-многим с файлами
	Files []File `gorm:"foreignKey:ProjectID"`

	// Связь с комментариями
	Comments []Comment `gorm:"foreignKey:ProjectID"`
}
