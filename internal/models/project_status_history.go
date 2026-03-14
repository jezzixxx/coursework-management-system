package models

import "time"

// ProjectStatusHistory хранит историю изменений статусов проекта
type ProjectStatusHistory struct {
	ID        uint   `gorm:"primaryKey"`
	ProjectID uint   `gorm:"not null;index"`
	OldStatus string `gorm:"size:50"`
	NewStatus string `gorm:"size:50"`
	ChangedBy uint   `gorm:"not null"`
	ChangedAt time.Time
}
