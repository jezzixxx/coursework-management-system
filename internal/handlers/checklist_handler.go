package handlers

import (
	"coursework/internal/config"
	"coursework/internal/models"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ShowChecklist показывает прогресс заполнения документов
func ShowChecklist(c *gin.Context) {
	user, _ := c.Get("currentUser")
	projectID := c.Param("id")

	// Проверяем доступ
	if !checkProjectAccess(c, projectID) {
		c.HTML(http.StatusForbidden, "error.html", gin.H{
			"user":  user,
			"error": "Доступ запрещён",
		})
		return
	}

	// Получаем загруженные файлы проекта
	var files []models.File
	config.DB.Where("project_id = ?", projectID).Find(&files)

	// Считаем прогресс по категориям
	uploadedTypes := make(map[string]bool)
	for _, file := range files {
		if file.IsActive {
			uploadedTypes[file.FileType] = true
		}
	}

	// Считаем проценты
	totalRequired := 0
	filledRequired := 0
	categoryProgress := make(map[string]map[string]int) // category -> {total, filled}

	for _, doc := range models.RequiredDocuments {
		if doc.IsRequired {
			totalRequired++
			if uploadedTypes[doc.Code] {
				filledRequired++
			}
		}

		if _, ok := categoryProgress[doc.Category]; !ok {
			categoryProgress[doc.Category] = map[string]int{"total": 0, "filled": 0}
		}
		if doc.IsRequired {
			categoryProgress[doc.Category]["total"]++
			if uploadedTypes[doc.Code] {
				categoryProgress[doc.Category]["filled"]++
			}
		}
	}

	overallProgress := 0
	if totalRequired > 0 {
		overallProgress = (filledRequired * 100) / totalRequired
	}

	c.HTML(http.StatusOK, "checklist.html", gin.H{
		"user":             user,
		"documents":        models.RequiredDocuments,
		"uploadedTypes":    uploadedTypes,
		"overallProgress":  overallProgress,
		"categoryProgress": categoryProgress,
		"totalRequired":    totalRequired,
		"filledRequired":   filledRequired,
	})
}

func CalculateProjectProgress(projectID uint) (int, int, int, error) {
	required := config.DefaultRequiredDocuments
	total := len(required)

	var completedCount int64
	err := config.DB.Model(&models.ProjectDocumentType{}).
		Where("project_id = ? AND type_code IN ? AND is_complete = ?", projectID, required, true).
		Count(&completedCount).Error

	if err != nil {
		return 0, total, 0, err
	}

	percent := 0
	if total > 0 {
		percent = int(float64(completedCount) / float64(total) * 100)
	}
	return int(completedCount), total, percent, nil
}
