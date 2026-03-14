package handlers

import (
	"coursework/internal/config"
	"coursework/internal/models"
	"coursework/internal/service"
	"coursework/internal/utils"
	"encoding/csv"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
)

// ShowAdminDashboard показывает панель админа
func ShowAdminDashboard(c *gin.Context) {
	user, _ := c.Get("currentUser")
	c.HTML(http.StatusOK, "admin_dashboard.html", gin.H{
		"user": user,
	})
}

// ShowImportPage показывает страницу импорта
func ShowImportPage(c *gin.Context) {
	user, _ := c.Get("currentUser")
	c.HTML(http.StatusOK, "import.html", gin.H{
		"success": "",
		"error":   "",
		"user":    user,
	})
}

// ImportUsers обрабатывает загрузку CSV
func ImportUsers(c *gin.Context) {
	user, _ := c.Get("currentUser")

	file, header, err := c.Request.FormFile("csvfile")
	if err != nil {
		c.HTML(http.StatusOK, "import.html", gin.H{
			"success": "",
			"error":   "Ошибка загрузки файла: " + err.Error(),
			"user":    user,
		})
		return
	}
	defer file.Close()

	tempPath := filepath.Join("uploads", "temp_"+header.Filename)
	err = os.MkdirAll("uploads", 0755)
	if err != nil {
		c.HTML(http.StatusOK, "import.html", gin.H{
			"success": "",
			"error":   "Ошибка создания папки: " + err.Error(),
			"user":    user,
		})
		return
	}

	err = c.SaveUploadedFile(header, tempPath)
	if err != nil {
		c.HTML(http.StatusOK, "import.html", gin.H{
			"success": "",
			"error":   "Ошибка сохранения файла: " + err.Error(),
			"user":    user,
		})
		return
	}
	defer os.Remove(tempPath)

	users, err := service.ImportUsersFromCSV(tempPath)
	if err != nil {
		c.HTML(http.StatusOK, "import.html", gin.H{
			"success": "",
			"error":   "Ошибка импорта: " + err.Error(),
			"user":    user,
		})
		return
	}

	c.HTML(http.StatusOK, "import.html", gin.H{
		"success": "Успешно импортировано пользователей",
		"error":   "",
		"user":    user,
		"count":   len(users),
	})
}

// DownloadPasswords отдаёт файл с паролями
func DownloadPasswords(c *gin.Context) {
	// user не нужен - авторизация уже проверена в middleware
	filePath := "exported_passwords.csv"

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Файл с паролями не найден"})
		return
	}

	c.FileAttachment(filePath, "passwords.csv")
}

// ShowUsersPage показывает страницу управления пользователями
func ShowUsersPage(c *gin.Context) {
	user, _ := c.Get("currentUser")

	var users []models.User
	result := config.DB.Where("is_active = ?", true).Find(&users)
	log.Printf("📊 Найдено пользователей: %d", result.RowsAffected)
	for _, u := range users {
		log.Printf("  - ID: %d, Login: %s, IsActive: %v", u.ID, u.Login, u.IsActive)
	}
	c.HTML(http.StatusOK, "admin_users.html", gin.H{
		"user":  user,
		"users": users,
	})
}

// AdminResetPassword - админ сбрасывает пароль пользователю
func AdminResetPassword(c *gin.Context) {
	userID := c.PostForm("user_id")

	var targetUser models.User
	config.DB.First(&targetUser, userID)

	if targetUser.ID == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Пользователь не найден"})
		return
	}

	newPassword, err := utils.GenerateSecurePassword()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка генерации пароля"})
		return
	}

	err = targetUser.SetPassword(newPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка сохранения пароля"})
		return
	}

	targetUser.MustChangePassword = true
	config.DB.Save(&targetUser)

	// Сохраняем в файл для экспорта
	err = appendResetPasswordToCSV(targetUser.Login, targetUser.FullName, targetUser.Group, newPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка сохранения пароля в файл"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"login":    targetUser.Login,
		"password": newPassword,
		"message":  "Пароль сброшен. Файл с паролями обновлён.",
		"file":     "/admin/download-reset-passwords",
	})
}

// Вспомогательная функция
func appendResetPasswordToCSV(login, fullName, group, password string) error {
	filePath := "reset_passwords.csv"

	// Проверяем, существует ли файл
	fileExists := true
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		fileExists = false
	}

	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Заголовок (если файл новый)
	if !fileExists {
		writer.Write([]string{"Timestamp", "Login", "FullName", "Group", "Password"})
	}

	// Данные
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	writer.Write([]string{timestamp, login, fullName, group, password})

	return nil
}

// DownloadResetPasswords отдаёт файл со сброшенными паролями
func DownloadResetPasswords(c *gin.Context) {
	filePath := "reset_passwords.csv"

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Файл не найден"})
		return
	}

	c.FileAttachment(filePath, "reset_passwords.csv")
}

// AdminDeleteUser - админ удаляет пользователя
func AdminDeleteUser(c *gin.Context) {
	userID := c.PostForm("user_id")

	// Проверяем, что это не сам админ
	currentUser, _ := c.Get("currentUser")
	currentUserID := currentUser.(models.User).ID

	var targetUser models.User
	result := config.DB.First(&targetUser, userID)

	if result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Пользователь не найден"})
		return
	}

	// Нельзя удалить самого себя
	if currentUserID == targetUser.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Нельзя удалить самого себя"})
		return
	}

	// Находим все проекты пользователя
	var projects []models.Project
	config.DB.Table("projects").
		Joins("JOIN project_members ON project_members.project_id = projects.id").
		Where("project_members.user_id = ?", targetUser.ID).
		Find(&projects)

	// Для каждого проекта проверяем, сколько там участников
	for _, project := range projects {
		var memberCount int64
		config.DB.Table("project_members").
			Where("project_id = ?", project.ID).
			Count(&memberCount)

		if memberCount <= 1 {
			// Если участник один — удаляем проект (включая файлы)
			deleteProject(project.ID)
		} else {
			// Если участников больше — просто удаляем из проекта
			config.DB.Table("project_members").
				Where("project_id = ? AND user_id = ?", project.ID, targetUser.ID).
				Delete(nil)
		}
	}

	targetUser.IsActive = false
	targetUser.Login = "deleted_" + targetUser.Login // Освобождаем логин
	targetUser.FullName = "Удалённый пользователь"   // Скрываем ФИО
	config.DB.Save(&targetUser)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Пользователь деактивирован",
	})
}
