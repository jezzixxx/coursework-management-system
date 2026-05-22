package models

import "time"

type File struct {
	ID          uint   `gorm:"primaryKey"`
	ProjectID   uint   `gorm:"not null;index"`
	StorageUUID string `gorm:"uniqueIndex;not null;size:36"`
	DisplayName string `gorm:"not null;size:255"`
	LogicalName string `gorm:"size:100;index"` // 🔹 НОВОЕ: группирует версии одного документа
	FileType    string `gorm:"not null;size:50"`
	MimeType    string `gorm:"size:100"`
	Size        int64
	Version     int  `gorm:"not null;default:1"`
	IsActive    bool `gorm:"default:true"`
	ValidFrom   time.Time
	ValidTo     time.Time
	UploadedBy  uint `gorm:"not null"`
	UploadedAt  time.Time
	Status      string `gorm:"size:20;default:'uploaded'"`
}
