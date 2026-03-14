package models

import "time"

type File struct {
	ID          uint   `gorm:"primaryKey"`
	ProjectID   uint   `gorm:"not null;index"`
	StorageUUID string `gorm:"uniqueIndex;not null;size:36"` // Имя на диске (UUID)
	DisplayName string `gorm:"not null;size:255"`            // Имя для показа
	FileType    string `gorm:"not null;size:50"`             // report, source, docs
	MimeType    string `gorm:"size:100"`                     // application/pdf
	Size        int64
	Version     int  `gorm:"not null"`
	IsActive    bool `gorm:"default:true"` // Актуальная версия
	ValidFrom   time.Time
	ValidTo     time.Time // 2999-12-31 для актуальной версии
	UploadedBy  uint      `gorm:"not null"`
	UploadedAt  time.Time
}
