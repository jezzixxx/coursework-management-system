package handlers

import (
	"coursework/internal/config"
	"coursework/internal/models"
	"coursework/internal/scanner"
	"coursework/internal/service"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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

// FormatFileSize преобразует байты в человекочитаемый формат
func FormatFileSize(bytes int64) string {
	if bytes <= 0 {
		return "0 B"
	}

	units := []string{"B", "KB", "MB", "GB", "TB"}
	var unitIdx int
	size := float64(bytes)

	for size >= 1024 && unitIdx < len(units)-1 {
		size /= 1024
		unitIdx++
	}

	if unitIdx == 0 {
		return fmt.Sprintf("%d B", int64(size))
	}
	return fmt.Sprintf("%.1f %s", size, units[unitIdx])
}

func ShowProjectFiles(c *gin.Context) {
	user, _ := c.Get("currentUser")
	idStr := c.Param("id")

	pid, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{"user": user, "error": "Неверный ID проекта", "errorCode": "validation"})
		return
	}
	if !checkProjectAccess(c, idStr) {
		c.HTML(http.StatusForbidden, "error.html", gin.H{"user": user, "error": "Доступ запрещён", "errorCode": "forbidden"})
		return
	}

	var project models.Project
	config.DB.Preload("Members").Preload("Files").First(&project, pid)

	// 1. Группируем АКТИВНЫЕ файлы по FileType
	grouped := make(map[string][]models.File)
	for _, f := range project.Files {
		if f.IsActive {
			grouped[f.FileType] = append(grouped[f.FileType], f)
		}
	}

	// 2. Загружаем флаги завершения типов
	var docTypes []models.ProjectDocumentType
	config.DB.Where("project_id = ?", pid).Find(&docTypes)
	completeMap := make(map[string]bool)
	for _, dt := range docTypes {
		completeMap[dt.TypeCode] = dt.IsComplete
	}

	filled, total, percent, _ := service.CalculateProjectProgress(uint(pid))

	c.HTML(http.StatusOK, "project_files.html", gin.H{
		"user":        user,
		"project":     project,
		"grouped":     grouped, // ← map[FileType][]File
		"completeMap": completeMap,
		"docLabels":   config.DocumentLabels,
		"required":    config.DefaultRequiredDocuments,
		"progress":    gin.H{"filled": filled, "total": total, "percent": percent},
		"filterType":  c.Query("type"),
	})
}

// UploadFile обрабатывает загрузку файла
func UploadFile(c *gin.Context) {
	user, _ := c.Get("currentUser")
	projectID := c.PostForm("project_id")
	fileType := c.PostForm("file_type")
	logicalNameInput := c.PostForm("logical_name") // ← Подхватываем из формы

	// 1. Проверка доступа к проекту
	if !checkProjectAccess(c, projectID) {
		c.HTML(http.StatusForbidden, "error.html", gin.H{
			"user":      user,
			"error":     "Доступ к этому проекту запрещён",
			"errorCode": "forbidden",
		})
		return
	}

	// 2. Получаем файл
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

	// 3. Базовые проверки
	ext := strings.ToLower(filepath.Ext(header.Filename))
	mimeType := header.Header.Get("Content-Type")

	if header.Size > 100*1024*1024 {
		c.HTML(http.StatusOK, "project_files.html", gin.H{
			"user":      user,
			"error":     "Файл слишком большой (макс 100MB)",
			"project":   getProject(projectID),
			"errorCode": "validation",
		})
		return
	}

	// 🛡️ ВАЖНО: Проверка расширения (у тебя мап был объявлен, но не использовался)
	if _, ok := allowedExtensions[ext]; !ok {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"user":      user,
			"error":     "Формат файла не поддерживается",
			"errorCode": "validation",
		})
		return
	}

	// 4. ОПРЕДЕЛЯЕМ LOGICAL_NAME
	var logicalName string
	if logicalNameInput != "" {
		// Валидация: только безопасные символы (защита от инъекций и обхода путей)
		matched, _ := regexp.MatchString(`^[a-zA-Z0-9_\-]+$`, logicalNameInput)
		if !matched {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{"user": user, "error": "Недопустимый формат имени документа", "errorCode": "validation"})
			return
		}

		// Проверяем, что такой логический документ уже существует в этом проекте
		var exists int64
		config.DB.Model(&models.File{}).
			Where("project_id = ? AND file_type = ? AND logical_name = ?", projectID, fileType, logicalNameInput).
			Count(&exists)
		if exists == 0 {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{"user": user, "error": "Указанный документ не найден в проекте"})
			return
		}
		logicalName = logicalNameInput
	} else {
		// Фоллбэк: автогенерация для обычной загрузки
		var docCount int64
		config.DB.Model(&models.File{}).
			Select("COUNT(DISTINCT logical_name)").
			Where("project_id = ? AND file_type = ?", projectID, fileType).
			Count(&docCount)
		logicalName = fmt.Sprintf("%s%d", fileType, docCount+1)
	}

	// 5. ВЕРСИОНИРОВАНИЕ
	var lastVersion int
	config.DB.Model(&models.File{}).
		Where("project_id = ? AND logical_name = ?", projectID, logicalName).
		Select("COALESCE(MAX(version), 0)").
		Scan(&lastVersion)
	newVersion := lastVersion + 1

	// Архивируем предыдущую активную версию
	config.DB.Model(&models.File{}).
		Where("project_id = ? AND logical_name = ? AND is_active = ?", projectID, logicalName, true).
		Updates(map[string]interface{}{"is_active": false, "valid_to": time.Now()})

	// 6. СОХРАНЕНИЕ НА ДИСК
	var project models.Project
	config.DB.First(&project, projectID)

	err = os.MkdirAll(project.FolderPath, 0755)
	if err != nil {
		c.HTML(http.StatusOK, "project_files.html", gin.H{
			"user":    user,
			"error":   "Ошибка создания папки: " + err.Error(),
			"project": getProject(projectID),
		})
		return
	}

	fileUUID := uuid.New().String()
	storedFilename := fileUUID + ext
	filePath := filepath.Join(project.FolderPath, storedFilename)

	err = c.SaveUploadedFile(header, filePath)
	if err != nil {
		c.HTML(http.StatusOK, "project_files.html", gin.H{
			"user":    user,
			"error":   "Ошибка сохранения: " + err.Error(),
			"project": getProject(projectID),
		})
		return
	}

	// 7. ПРОВЕРКА НА ВИРУСЫ
	isClean, err := scanner.ScanForVirus(filePath) // ← с большой буквы
	if err != nil {
		os.Remove(filePath)
		log.Printf("Virus scan failed for file %s: %v", filePath, err) // ← в логи с деталями

		c.HTML(http.StatusOK, "project_files.html", gin.H{
			"user":      user,
			"error":     "Не удалось проверить файл на вирусы. Попробуйте позже или обратитесь к администратору.", // ← пользователю безопасно
			"project":   getProject(projectID),
			"errorCode": "system",
		})
		return
	}

	if !isClean {
		os.Remove(filePath)
		log.Printf("Threat detected in file %s: %v", filePath, err) // ← в логи название угрозы

		c.HTML(http.StatusOK, "project_files.html", gin.H{
			"user":      user,
			"error":     "Файл не прошёл проверку безопасности и был удалён.", // ← без деталей
			"project":   getProject(projectID),
			"errorCode": "system",
		})
		return
	}
	// 8. ЗАПИСЬ В БД
	newFile := models.File{
		ProjectID:   project.ID,
		StorageUUID: fileUUID,
		DisplayName: fmt.Sprintf("%s_%s_v%d%s", logicalName, user.(models.User).Login, newVersion, ext),
		LogicalName: logicalName,
		FileType:    fileType,
		MimeType:    mimeType,
		Size:        header.Size,
		Version:     newVersion,
		IsActive:    true,
		ValidFrom:   time.Now(),
		ValidTo:     time.Date(2999, 12, 31, 23, 59, 59, 0, time.UTC),
		UploadedBy:  user.(models.User).ID,
		UploadedAt:  time.Now(),
		Status:      config.FileStatusUploaded,
	}
	config.DB.Create(&newFile)
	_ = service.SyncProjectStatus(newFile.ProjectID)

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
			"user":      user,
			"error":     "Доступ к этому файлу запрещён",
			"errorCode": "forbidden",
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
			"user":      user,
			"error":     "Файл не найден на сервере",
			"errorCode": "not_found",
		})
		return
	}

	// Отдаём файл
	c.FileAttachment(filePath, file.DisplayName)
}

// ShowFileHistory показывает историю версий конкретного логического документа
func ShowFileHistory(c *gin.Context) {
	user, _ := c.Get("currentUser")
	fileID := c.Param("id")

	var file models.File
	if err := config.DB.First(&file, fileID).Error; err != nil {
		c.HTML(http.StatusNotFound, "error.html", gin.H{"user": user, "error": "Файл не найден", "errorCode": "not_found"})
		return
	}

	if !checkProjectAccess(c, fmt.Sprintf("%d", file.ProjectID)) {
		c.HTML(http.StatusForbidden, "error.html", gin.H{"user": user, "error": "Доступ запрещён", "errorCode": "forbidden"})
		return
	}

	// 🔹 Ищем ВСЕ версии этого логического документа (активные + архивные)
	var versions []models.File
	config.DB.Where("project_id = ? AND file_type = ? AND logical_name = ?",
		file.ProjectID, file.FileType, file.LogicalName).
		Order("version DESC").Find(&versions)
	var project models.Project
	config.DB.First(&project, file.ProjectID)

	c.HTML(http.StatusOK, "file_history.html", gin.H{
		"user":      user,
		"file":      file,
		"project":   project, // ← добавь
		"versions":  versions,
		"docLabels": config.DocumentLabels, // ← добавь
	})
}

// Вспомогательные функции

func getProject(projectID string) models.Project {
	var project models.Project
	config.DB.Preload("Members").Preload("Files").First(&project, projectID)
	return project
}

// ReviewFile меняет статус файла по решению админа
func ReviewFile(c *gin.Context) {
	fileID := c.Param("id")
	newStatus := c.PostForm("status")

	if newStatus != config.FileStatusAccepted && newStatus != config.FileStatusRevision {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Недопустимый статус"})
		return
	}

	var file models.File
	if err := config.DB.First(&file, fileID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Файл не найден"})
		return
	}

	file.Status = newStatus
	if err := config.DB.Save(&file).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка сохранения статуса"})
		return
	}

	// 🔄 Автоматически пересчитываем статус проекта
	if err := service.SyncProjectStatus(file.ProjectID); err != nil {
		log.Printf("⚠️ Ошибка авто-статуса проекта #%d: %v", file.ProjectID, err)
	}

	c.Redirect(http.StatusSeeOther, fmt.Sprintf("/project/%d/files", file.ProjectID))
}

func DeleteFile(c *gin.Context) {
	user, _ := c.Get("currentUser")
	fileID := c.Param("id")

	var file models.File
	if err := config.DB.First(&file, fileID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Файл не найден"})
		return
	}

	// Проверка прав: админ или тот, кто загрузил
	currentUser := user.(models.User)
	if currentUser.Role != "admin" && file.UploadedBy != currentUser.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Нет прав на удаление"})
		return
	}

	// 🔹 SOFT DELETE: не стираем физически, а архивируем
	now := time.Now()
	config.DB.Model(&file).Updates(map[string]interface{}{
		"is_active": false,
		"valid_to":  now,
	})

	// Пересчитываем статус проекта
	_ = service.SyncProjectStatus(file.ProjectID)

	c.Redirect(http.StatusSeeOther, fmt.Sprintf("/project/%d/files", file.ProjectID))
}

// ToggleDocumentTypeComplete переключает флаг "достаточности" типа документа
func ToggleDocumentTypeComplete(c *gin.Context) {
	user, _ := c.Get("currentUser")

	// 1. Парсим ID проекта и код типа из URL
	projectIDStr := c.Param("id")
	typeCode := c.Param("code")

	projectID, err := strconv.ParseUint(projectIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный ID проекта"})
		return
	}
	pid := uint(projectID)

	// 2. Ищем или создаём запись в БД
	var docType models.ProjectDocumentType
	config.DB.Where("project_id = ? AND type_code = ?", pid, typeCode).
		FirstOrCreate(&docType, models.ProjectDocumentType{
			ProjectID:  pid,
			TypeCode:   typeCode,
			IsComplete: false,
		})

	// 3. Переключаем флаг
	admin := user.(models.User)
	docType.IsComplete = !docType.IsComplete
	docType.CompletedBy = admin.ID
	docType.CompletedAt = time.Now()
	config.DB.Save(&docType)

	// 4. Пересчитываем прогресс
	filled, total, percent, _ := service.CalculateProjectProgress(pid)

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"isComplete": docType.IsComplete,
		"progress":   gin.H{"filled": filled, "total": total, "percent": percent},
	})
}
