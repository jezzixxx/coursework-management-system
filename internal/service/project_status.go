package service

import (
	"coursework/internal/config"
	"coursework/internal/models"
	"log"

	"gorm.io/gorm"
)

// CalculateProjectStatus вычисляет статус проекта на основе статусов файлов
func CalculateProjectStatus(projectID uint) string {
	var files []models.File
	config.DB.Where("project_id = ? AND is_active = ?", projectID, true).Find(&files)

	if len(files) == 0 {
		return config.ProjectStatusNew
	}

	hasPending := false
	hasRevision := false
	for _, f := range files {
		if f.Status == config.FileStatusUploaded || f.Status == config.FileStatusUpdated {
			hasPending = true
		}
		if f.Status == config.FileStatusRevision {
			hasRevision = true
		}
	}

	if hasRevision {
		return config.ProjectStatusRevision
	}
	if hasPending {
		return config.ProjectStatusNeedsReview
	}
	// Все загруженные файлы приняты админом
	return config.ProjectStatusApproved
}

// SyncProjectStatus пересчитывает статус, сохраняет его и логирует в историю
func SyncProjectStatus(projectID uint) error {
	newStatus := CalculateProjectStatus(projectID)

	var project models.Project
	if err := config.DB.First(&project, projectID).Error; err != nil {
		return err
	}

	if project.CurrentStatus == newStatus {
		return nil // Статус не изменился
	}

	oldStatus := project.CurrentStatus
	project.CurrentStatus = newStatus

	// 1. Убедись, что в imports добавлено: "gorm.io/gorm"

	return config.DB.Transaction(func(tx *gorm.DB) error { // ← было *config.DB, стало *gorm.DB
		if err := tx.Save(&project).Error; err != nil {
			return err
		}

		// 2. ChangedBy в модели имеет тип uint (ID пользователя).
		// Для системных/автоматических действий принято передавать 0.
		// 3. Поля Comment в модели ProjectStatusHistory нет → убираем его.
		tx.Create(&models.ProjectStatusHistory{
			ProjectID: project.ID,
			OldStatus: oldStatus,
			NewStatus: newStatus,
			ChangedBy: 0, // 0 означает "система/автоматически"
		})

		log.Printf("🔄 Проект #%d: статус %s → %s (авто)", project.ID, oldStatus, newStatus)
		return nil
	})
}
