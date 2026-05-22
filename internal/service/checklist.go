package service

import (
	"coursework/internal/config"
	"coursework/internal/models"
)

// CalculateProjectProgress считает прогресс по флагам админа (IsComplete)
func CalculateProjectProgress(projectID uint) (int, int, int, error) {
	required := config.DefaultRequiredDocuments
	total := len(required)

	var count int64
	config.DB.Model(&models.ProjectDocumentType{}).
		Where("project_id = ? AND type_code IN ? AND is_complete = ?", projectID, required, true).
		Count(&count)

	percent := 0
	if total > 0 {
		percent = int(float64(count) / float64(total) * 100)
	}
	return int(count), total, percent, nil
}
