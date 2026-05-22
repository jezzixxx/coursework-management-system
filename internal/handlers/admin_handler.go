package handlers

import (
	"coursework/internal/config"
	"coursework/internal/models"
	"coursework/internal/service"
	"coursework/internal/utils"
	"crypto/rand"
	"encoding/csv"
	"encoding/hex"
	"fmt"

	//"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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

// generateSecureFilename создаёт имя вида prefix_randomstring.csv
func generateSecureFilename(prefix string) string {
	b := make([]byte, 16)
	_, _ = rand.Read(b) // игнорируем ошибку для учебной задачи, в проде стоит обработать
	return fmt.Sprintf("%s_%s.csv", prefix, hex.EncodeToString(b))
}

func ImportUsers(c *gin.Context) {
	user, _ := c.Get("currentUser")

	file, header, err := c.Request.FormFile("csvfile")
	if err != nil {
		c.HTML(http.StatusOK, "import.html", gin.H{
			"success": "", "error": "Ошибка загрузки файла: " + err.Error(), "user": user,
		})
		return
	}
	defer file.Close()

	// Сохраняем временный файл
	os.MkdirAll("uploads", 0755)
	tempPath := filepath.Join("uploads", "temp_"+header.Filename)
	if err := c.SaveUploadedFile(header, tempPath); err != nil {
		c.HTML(http.StatusOK, "import.html", gin.H{
			"success": "", "error": "Ошибка сохранения: " + err.Error(), "user": user,
		})
		return
	}
	defer os.Remove(tempPath)

	// 🔒 Создаём безопасный файл для паролей ПРЯМО в secure_uploads
	secureDir := "secure_uploads"
	os.MkdirAll(secureDir, 0750)
	secureName := generateSecureFilename("import_passwords_") // ← теперь с подчёркиванием!
	securePath := filepath.Join(secureDir, secureName)

	secureFile, err := os.Create(securePath)
	if err != nil {
		c.HTML(http.StatusOK, "import.html", gin.H{
			"success": "", "error": "Ошибка создания файла паролей: " + err.Error(), "user": user,
		})
		return
	}
	// Закрываем файл ПОСЛЕ того, как сервис в него запишет
	defer secureFile.Close()

	// 🔄 Вызываем сервис, передавая файл как io.Writer
	users, err := service.ImportUsersFromCSV(tempPath, secureFile)
	if err != nil {
		os.Remove(securePath) // откат: удаляем неполный файл
		c.HTML(http.StatusOK, "import.html", gin.H{
			"success": "", "error": "Ошибка импорта: " + err.Error(), "user": user,
		})
		return
	}

	// Всё успешно — отдаём страницу со ссылкой
	// Файл уже лежит в secure_uploads, serveSecureCSV его найдёт и удалит после скачивания
	c.HTML(http.StatusOK, "import.html", gin.H{
		"success":      fmt.Sprintf("Успешно импортировано: %d", len(users)),
		"error":        "",
		"user":         user,
		"count":        len(users),
		"downloadLink": fmt.Sprintf("/download-passwords?file=%s", secureName),
		"downloadName": secureName,
	})
}

func DownloadPasswords(c *gin.Context) {
	serveSecureCSV(c, "import_passwords_")
}

func DownloadResetPasswords(c *gin.Context) {
	serveSecureCSV(c, "reset_")
}

// serveSecureCSV отдаёт файл с паролями, ставит безопасные заголовки и удаляет файл после отдачи
func serveSecureCSV(c *gin.Context, allowedPrefix string) {
	// 1. Проверка авторизации
	if _, exists := c.Get("currentUser"); !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Требуется авторизация"})
		return
	}

	// 2. Получаем имя файла
	fileName := c.Query("file")
	if fileName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Не указан файл"})
		return
	}

	// 3. Защита от Path Traversal
	baseName := filepath.Base(fileName)
	if !strings.HasPrefix(baseName, allowedPrefix) || !strings.HasSuffix(baseName, ".csv") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Недопустимое имя файла"})
		return
	}

	// 4. 🔧 Приводим путь к абсолютному — ключевое исправление для Docker!
	relPath := filepath.Join("secure_uploads", baseName)
	absPath, err := filepath.Abs(relPath)
	if err != nil {
		log.Printf("❌ Ошибка получения абсолютного пути: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Серверная ошибка"})
		return
	}

	// 🔍 Отладка: логируем, что ищем
	cwd, _ := os.Getwd()
	log.Printf("🔎 CWD: %s | Относительный: %s | Абсолютный: %s", cwd, relPath, absPath)

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		log.Printf("❌ Файл не найден: %s", absPath)
		// Бонус: покажем, что вообще есть в папке
		entries, _ := os.ReadDir("secure_uploads")
		for _, e := range entries {
			log.Printf("📁 В secure_uploads: %s (dir=%v)", e.Name(), e.IsDir())
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "Файл не найден или срок его действия истёк"})
		return
	}

	// 5. Заголовки безопасности
	c.Header("Cache-Control", "no-store, no-cache, must-revalidate, private")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
	c.Header("X-Content-Type-Options", "nosniff")

	// 6. Отдаём файл и удаляем после
	c.FileAttachment(absPath, baseName)
	defer os.Remove(absPath)
}

// ShowUsersPage показывает страницу управления пользователями с серверной фильтрацией
func ShowUsersPage(c *gin.Context) {
	user, _ := c.Get("currentUser")

	// Получаем параметры из query string: /admin/users?year=22&search=иван
	filterYear := c.Query("year")
	filterSearch := c.Query("search")

	// Базовый запрос: только активные пользователи
	query := config.DB.Where("is_active = ?", true)

	// Фильтр по году (если передан)
	if filterYear != "" {
		query = query.Where("year = ?", filterYear)
	}

	// Поиск по подстроке в логине или ФИО
	if filterSearch != "" {
		searchTerm := "%" + filterSearch + "%"
		query = query.Where("login LIKE ? OR full_name LIKE ?", searchTerm, searchTerm)
	}

	// Выполняем запрос пользователей
	var users []models.User
	query.Order("year ASC, login ASC").Find(&users)

	// Получаем уникальные года для фильтра (из всех активных)
	var years []int
	dbRes := config.DB.Model(&models.User{}).
		Where("is_active = ?", true).
		Where("year > 0").
		Distinct("year").
		Order("year ASC").
		Pluck("year", &years)

	if dbRes.Error != nil {
		log.Printf("⚠️ Не удалось получить список годов: %v", dbRes.Error)
	}

	c.HTML(http.StatusOK, "admin_users.html", gin.H{
		"user":         user,
		"users":        users,
		"filterYear":   filterYear,
		"filterSearch": filterSearch,
		"years":        years,
	})
}

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

	secureDir := "secure_uploads"
	os.MkdirAll(secureDir, 0750)
	secureName := generateSecureFilename(fmt.Sprintf("reset_%s_", targetUser.Login))
	securePath := filepath.Join(secureDir, secureName)

	secureFile, err := os.Create(securePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка создания файла"})
		return
	}
	defer secureFile.Close()

	// Пишем прямо в файл через csv.Writer
	writer := csv.NewWriter(secureFile)
	defer writer.Flush()
	writer.Write([]string{"Timestamp", "Login", "FullName", "Group", "Password"})
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	writer.Write([]string{timestamp, targetUser.Login, targetUser.FullName, targetUser.Group, newPassword})

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"login":        targetUser.Login,
		"message":      "Пароль сброшен",
		"autoDownload": fmt.Sprintf("/download-reset-passwords?file=%s", secureName),
		"downloadName": secureName,
	})
}

// Вспомогательная функция: добавляет запись в CSV со сброшенными паролями
func appendResetPasswordToCSV(login, fullName, group, password string) error {
	filePath := "reset_passwords.csv"
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

	if !fileExists {
		writer.Write([]string{"Timestamp", "Login", "FullName", "Group", "Password"})
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	writer.Write([]string{timestamp, login, fullName, group, password})

	return nil
}

// AdminDeleteUser - админ удаляет пользователя
func AdminDeleteUser(c *gin.Context) {
	userID := c.PostForm("user_id")
	currentUser, _ := c.Get("currentUser")
	currentUserID := currentUser.(models.User).ID

	var targetUser models.User
	result := config.DB.First(&targetUser, userID)

	if result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Пользователь не найден"})
		return
	}

	if currentUserID == targetUser.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Нельзя удалить самого себя"})
		return
	}

	var projects []models.Project
	config.DB.Table("projects").
		Joins("JOIN project_members ON project_members.project_id = projects.id").
		Where("project_members.user_id = ?", targetUser.ID).
		Find(&projects)

	for _, project := range projects {
		var memberCount int64
		config.DB.Table("project_members").
			Where("project_id = ?", project.ID).
			Count(&memberCount)

		if memberCount <= 1 {
			deleteProject(project.ID)
		} else {
			config.DB.Table("project_members").
				Where("project_id = ? AND user_id = ?", project.ID, targetUser.ID).
				Delete(nil)
		}
	}

	targetUser.IsActive = false
	targetUser.Login = fmt.Sprintf("deleted_%d_%s", targetUser.ID, targetUser.Login)
	targetUser.FullName = "Удалённый пользователь"

	result = config.DB.Save(&targetUser)
	if result.Error != nil {
		log.Printf("❌ Ошибка при деактивации пользователя %d: %v", targetUser.ID, result.Error)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при деактивации пользователя"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Пользователь деактивирован",
	})
}
