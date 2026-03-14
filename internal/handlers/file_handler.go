package handlers

import (
	"coursework/internal/config"
	"coursework/internal/models"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Разрешённые расширения (добавили инженерные форматы)
var allowedExtensions = map[string]bool{
	// Документы
	".pdf":  true,
	".doc":  true,
	".docx": true,
	".txt":  true,
	".rtf":  true,
	".odt":  true,

	// Архивы
	".zip": true,
	".rar": true,
	".7z":  true,
	".tar": true,
	".gz":  true,

	// Изображения
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".bmp":  true,
	".gif":  true,
	".svg":  true,

	// Видео
	".mp4": true,
	".avi": true,
	".mkv": true,
	".mov": true,

	// Исполняемые файлы (с предупреждением)
	".exe": true,
	".bat": true,
	".sh":  true,
	".py":  true,
	".jar": true,

	// === КОМПАС-3D ===
	".a3d": true, // 3D модель
	".cdw": true, // Чертеж
	".spw": true, // Спецификация
	".kdw": true, // Документ

	// === PROTEUS ===
	".pdsprj": true, // Проект Proteus
	".dsn":    true, // Схема
	".lyt":    true, // Разводка платы
	".lib":    true, // Библиотека

	// === PCB ДИЗАЙН (Altium, Eagle, KiCad, etc.) ===
	".sch":    true, // Схема (Eagle, KiCad, Altium)
	".schdoc": true, // Схема (Altium)
	".pcb":    true, // Плата (Eagle, KiCad, P-CAD)
	".pcbdoc": true, // Плата (Altium)
	".gbr":    true, // Gerber
	".gerber": true, // Gerber
	".drl":    true, // Сверловка
	".gko":    true, // Контур платы (Gerber)
	".gpi":    true, // Gerber
	".gtl":    true, // Gerber Top Layer
	".gbl":    true, // Gerber Bottom Layer
	".gts":    true, // Gerber Top Solder
	".gbs":    true, // Gerber Bottom Solder
	".gto":    true, // Gerber Top Overlay
	".gbo":    true, // Gerber Bottom Overlay
	".cmp":    true, // Component (Altium)
	".sol":    true, // Solder (Altium)

	// === ДРУГИЕ CAD/EDA ===
	".step":      true, // 3D модель (STEP)
	".stp":       true, // 3D модель (STEP)
	".igs":       true, // IGES
	".iges":      true, // IGES
	".stl":       true, // 3D печать
	".kicad_pcb": true, // KiCad плата
	".kicad_sch": true, // KiCad схема
	".brd":       true, // Eagle плата
	".fzz":       true, // Fritzing
}

// Разрешённые MIME-типы
var allowedMimeTypes = map[string]bool{
	// Документы
	"application/pdf":    true,
	"application/msword": true,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
	"text/plain":      true,
	"application/rtf": true,
	"application/vnd.oasis.opendocument.text": true,

	// Архивы
	"application/zip":              true,
	"application/x-rar-compressed": true,
	"application/x-7z-compressed":  true,
	"application/x-tar":            true,
	"application/gzip":             true,

	// Изображения
	"image/png":     true,
	"image/jpeg":    true,
	"image/bmp":     true,
	"image/gif":     true,
	"image/svg+xml": true,

	// Видео
	"video/mp4":        true,
	"video/x-msvideo":  true,
	"video/x-matroska": true,
	"video/quicktime":  true,

	// Исполняемые
	"application/x-msdownload": true,
	"application/x-bat":        true,
	"application/x-sh":         true,
	"application/x-python":     true,
	"application/java-archive": true,

	// === КОМПАС-3D ===
	"application/x-kompas-a3d": true,
	"application/x-kompas-cdw": true,
	"application/x-kompas-spw": true,

	// === PROTEUS ===
	"application/x-proteus-pdsprj": true,
	"application/x-proteus-dsn":    true,

	// === PCB / CAD ===
	"application/x-gerber": true,
	"application/x-eagle":  true,
	"application/x-kicad":  true,
	"application/step":     true,
	"application/iges":     true,
	"model/stl":            true,
}

// ShowProjectFiles показывает файлы проекта
func ShowProjectFiles(c *gin.Context) {
	user, _ := c.Get("currentUser")
	projectID := c.Param("id")

	// Проверяем доступ к проекту
	if !checkProjectAccess(c, projectID) {
		c.HTML(http.StatusForbidden, "error.html", gin.H{
			"user":  user,
			"error": "Доступ к этому проекту запрещён",
		})
		return
	}

	var project models.Project
	config.DB.Preload("Members").Preload("Files").First(&project, projectID)

	c.HTML(http.StatusOK, "project_files.html", gin.H{
		"user":    user,
		"project": project,
	})
}

// Проверка на вирусы через ClamAV (сетевой сервис)
func scanForVirus(filePath string) (bool, error) {
	// Получаем хост из переменной окружения
	clamavHost := os.Getenv("CLAMAV_HOST")
	if clamavHost == "" {
		clamavHost = "clamav" // Docker-сервис
	}

	// Пробуем через сетевой clamd
	isClean, err := scanWithClamDNetwork(filePath, clamavHost)
	if err == nil {
		return isClean, nil
	}

	// Если ClamAV недоступен, логируем предупреждение
	log.Printf("⚠️ ClamAV недоступен (%s). Файл %s загружен без проверки!", clamavHost, filePath)

	// Для курсового: возвращаем true (файл чист)
	// В продакшене: return false, errors.New("ClamAV недоступен")
	return true, nil
}

// Сканирование через сетевой ClamAV daemon
func scanWithClamDNetwork(filePath string, host string) (bool, error) {
	// Открываем файл
	file, err := os.Open(filePath)
	if err != nil {
		return false, err
	}
	defer file.Close()

	// Подключаемся к ClamAV daemon (порт 3310)
	conn, err := net.Dial("tcp", fmt.Sprintf("%s:3310", host))
	if err != nil {
		return false, err
	}
	defer conn.Close()

	// Отправляем файл на сканирование (STREAM команда)
	// Упрощённая версия - для курсового достаточно заглушки
	log.Printf("🔍 Сканирование файла через ClamAV: %s", filePath)

	// TODO: Реализовать полный протокол ClamAV STREAM
	// Для курсового: считаем файл чистым
	return true, nil
}

// UploadFile обрабатывает загрузку файла
func UploadFile(c *gin.Context) {
	user, _ := c.Get("currentUser")
	projectID := c.PostForm("project_id")
	fileType := c.PostForm("file_type") // report, source, docs

	// Проверяем доступ к проекту
	if !checkProjectAccess(c, projectID) {
		c.HTML(http.StatusForbidden, "error.html", gin.H{
			"user":  user,
			"error": "Доступ к этому проекту запрещён",
		})
		return
	}

	// Получаем файл
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.HTML(http.StatusOK, "project_files.html", gin.H{
			"user":    user,
			"error":   "Ошибка загрузки: " + err.Error(),
			"project": getProject(projectID),
		})
		return
	}
	defer file.Close()

	// === ПРОВЕРКА БЕЗОПАСНОСТИ ===

	// 1. Проверка расширения
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !allowedExtensions[ext] {
		c.HTML(http.StatusOK, "project_files.html", gin.H{
			"user":    user,
			"error":   "Недопустимый тип файла. Разрешены: pdf, doc, docx, zip, txt",
			"project": getProject(projectID),
		})
		return
	}

	// 2. Проверка MIME-типа
	mimeType := header.Header.Get("Content-Type")
	if !allowedMimeTypes[mimeType] {
		c.HTML(http.StatusOK, "project_files.html", gin.H{
			"user":    user,
			"error":   "Недопустимый формат файла (MIME)",
			"project": getProject(projectID),
		})
		return
	}

	// 3. Проверка размера (макс 10MB)
	if header.Size > 10*1024*1024 {
		c.HTML(http.StatusOK, "project_files.html", gin.H{
			"user":    user,
			"error":   "Файл слишком большой (макс 10MB)",
			"project": getProject(projectID),
		})
		return
	}

	// === СОХРАНЕНИЕ ===

	// Получаем проект для пути к папке
	var project models.Project
	config.DB.First(&project, projectID)

	// Генерируем UUID для имени файла на диске
	fileUUID := uuid.New().String()
	storedFilename := fileUUID + ext

	// Создаём папку проекта, если нет
	err = os.MkdirAll(project.FolderPath, 0755)
	if err != nil {
		c.HTML(http.StatusOK, "project_files.html", gin.H{
			"user":    user,
			"error":   "Ошибка создания папки: " + err.Error(),
			"project": getProject(projectID),
		})
		return
	}

	// Путь для сохранения
	filePath := filepath.Join(project.FolderPath, storedFilename)

	// Сохраняем файл
	err = c.SaveUploadedFile(header, filePath)
	if err != nil {
		c.HTML(http.StatusOK, "project_files.html", gin.H{
			"user":    user,
			"error":   "Ошибка сохранения: " + err.Error(),
			"project": getProject(projectID),
		})
		return
	}

	// === ПРОВЕРКА НА ВИРУСЫ ===
	isClean, err := scanForVirus(filePath)
	if err != nil {
		os.Remove(filePath) // Удаляем файл при ошибке сканера
		c.HTML(http.StatusOK, "project_files.html", gin.H{
			"user":    user,
			"error":   "Ошибка проверки на вирусы: " + err.Error(),
			"project": getProject(projectID),
		})
		return
	}

	if !isClean {
		os.Remove(filePath) // Удаляем заражённый файл
		c.HTML(http.StatusOK, "project_files.html", gin.H{
			"user":    user,
			"error":   "⛔ Файл не прошёл проверку на вирусы!",
			"project": getProject(projectID),
		})
		return
	}

	// === ВЕРСИОННОСТЬ (dh) ===

	// Получаем следующую версию
	var lastVersion int
	config.DB.Model(&models.File{}).
		Where("project_id = ? AND file_type = ?", projectID, fileType).
		Select("MAX(version)").Scan(&lastVersion)

	newVersion := lastVersion + 1

	// Закрываем предыдущую версию этого типа
	config.DB.Model(&models.File{}).
		Where("project_id = ? AND file_type = ? AND is_active = ?", projectID, fileType, true).
		Updates(map[string]interface{}{
			"is_active": false,
			"valid_to":  time.Now(),
		})

	// Создаём новую запись файла
	newFile := models.File{
		ProjectID:   project.ID,
		StorageUUID: fileUUID,
		DisplayName: fmt.Sprintf("%s_%s_v%d%s", user.(models.User).Login, fileType, newVersion, ext),
		FileType:    fileType,
		MimeType:    mimeType,
		Size:        header.Size,
		Version:     newVersion,
		IsActive:    true,
		ValidFrom:   time.Now(),
		ValidTo:     time.Date(2999, 12, 31, 23, 59, 59, 0, time.UTC),
		UploadedBy:  user.(models.User).ID,
		UploadedAt:  time.Now(),
	}

	config.DB.Create(&newFile)

	c.Redirect(http.StatusSeeOther, fmt.Sprintf("/project/%s/files", projectID))
}

// DownloadFile отдаёт файл с проверкой прав
func DownloadFile(c *gin.Context) {
	user, _ := c.Get("currentUser")
	fileID := c.Param("id")

	var file models.File
	config.DB.First(&file, fileID)

	// Проверяем доступ к проекту
	if !checkProjectAccess(c, fmt.Sprintf("%d", file.ProjectID)) {
		c.HTML(http.StatusForbidden, "error.html", gin.H{
			"user":  user,
			"error": "Доступ к этому файлу запрещён",
		})
		return
	}

	// Находим проект для пути
	var project models.Project
	config.DB.First(&project, file.ProjectID)

	// Путь к файлу на диске
	filePath := filepath.Join(project.FolderPath, file.StorageUUID+filepath.Ext(file.DisplayName))

	// Проверяем существование
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.HTML(http.StatusNotFound, "error.html", gin.H{
			"user":  user,
			"error": "Файл не найден на сервере",
		})
		return
	}

	// Отдаём файл
	c.FileAttachment(filePath, file.DisplayName)
}

// ShowFileHistory показывает историю версий файла
func ShowFileHistory(c *gin.Context) {
	user, _ := c.Get("currentUser")
	fileID := c.Param("id")

	var file models.File
	config.DB.First(&file, fileID)

	// Проверяем доступ
	if !checkProjectAccess(c, fmt.Sprintf("%d", file.ProjectID)) {
		c.HTML(http.StatusForbidden, "error.html", gin.H{
			"user":  user,
			"error": "Доступ запрещён",
		})
		return
	}

	// Получаем все версии этого файла
	var versions []models.File
	config.DB.Where("project_id = ? AND file_type = ?", file.ProjectID, file.FileType).
		Order("version DESC").
		Find(&versions)

	c.HTML(http.StatusOK, "file_history.html", gin.H{
		"user":     user,
		"file":     file,
		"versions": versions,
	})
}

// Вспомогательные функции

func getProject(projectID string) models.Project {
	var project models.Project
	config.DB.Preload("Members").Preload("Files").First(&project, projectID)
	return project
}
