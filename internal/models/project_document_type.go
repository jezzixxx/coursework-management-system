package models

import "time"

type ProjectDocumentType struct {
	ID          uint   `gorm:"primaryKey"`
	ProjectID   uint   `gorm:"index;not null"`
	TypeCode    string `gorm:"size:50;not null"`
	IsComplete  bool   `gorm:"default:false"`
	CompletedBy uint
	CompletedAt time.Time
}

func (ProjectDocumentType) TableName() string { return "project_document_types" }
