package handlers

import (
	"coursework/internal/config"
	"coursework/internal/models"
	"coursework/internal/service"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
)

// ShowProjects показывает список всех проектов с фильтрацией (админ)
func ShowProjects(c *gin.Context) {
	user, _ := c.Get("currentUser")

	filterYear := c.Query("year")
	filterStudent := c.Query("student")
	filterTitle := c.Query("title")
	filterStatus := c.Query("status")

	query := config.DB.Model(&models.Project{}).Select("projects.*")

	// Простые фильтры по полям проекта
	if filterTitle != "" {
		query = query.Where("title LIKE ?", "%"+filterTitle+"%")
	}
	if filterStatus != "" {
		query = query.Where("current_status = ?", filterStatus)
	}

	// Фильтры по участникам требуют JOIN
	if filterStudent != "" || filterYear != "" {
		query = query.
			Joins("JOIN project_members pm ON pm.project_id = projects.id").
			Joins("JOIN users u ON u.id = pm.user_id")

		if filterYear != "" {
			query = query.Where("u.year = ?", filterYear)
		}
		if filterStudent != "" {
			s := "%" + filterStudent + "%"
			query = query.Where("u.full_name LIKE ? OR u.login LIKE ?", s, s)
		}
		// Группируем, чтобы JOIN не размножал строки проектов
		query = query.Group("projects.id")
	}

	var projects []models.Project
	query.Preload("Members").Find(&projects)

	// Доступные годы для выпадающего списка
	var years []string
	config.DB.Model(&models.User{}).
		Where("role = ? AND is_active = ?", "student", true).
		Distinct("year").
		Order("year DESC").
		Pluck("year", &years)

		// Вспомогательная структура
	type ProjectWithProgress struct {
		models.Project
		ProgressPercent int
		ProgressFilled  int
		ProgressTotal   int
	}

	// Считаем прогресс для каждого проекта
	projectsWithProgress := make([]ProjectWithProgress, 0, len(projects))
	for _, p := range projects {
		filled, total, percent, _ := service.CalculateProjectProgress(p.ID)
		projectsWithProgress = append(projectsWithProgress, ProjectWithProgress{
			Project:         p,
			ProgressPercent: percent,
			ProgressFilled:  filled,
			ProgressTotal:   total,
		})
	}

	c.HTML(http.StatusOK, "projects.html", gin.H{
		"user":          user,
		"projects":      projectsWithProgress,
		"filterYear":    filterYear,
		"filterStudent": filterStudent,
		"filterTitle":   filterTitle,
		"filterStatus":  filterStatus,
		"years":         years,
	})
}

func ShowCreateProject(c *gin.Context) {
	user, _ := c.Get("currentUser")
	filterYear := c.Query("year")
	filterSearch := c.Query("search")

	// 👇 Одна строка вместо 15 строк запроса
	students := GetFilteredStudents(filterYear, filterSearch)
	years := GetAvailableYears()

	c.HTML(http.StatusOK, "create_project.html", gin.H{
		"user":           user,
		"students":       students,
		"filterYear":     filterYear,
		"filterSearch":   filterSearch,
		"availableYears": years,
	})
}

// GetFilteredStudents возвращает список активных студентов с фильтрацией по году и поиску
// Не исключает участников проектов — это делается на уровне шаблона или при добавлении
func GetFilteredStudents(year, search string) []models.User {
	query := config.DB.Where("role = ? AND is_active = ?", "student", true)

	// Фильтр по году
	if year != "" {
		// Пробуем привести к int, если в БД поле year — integer
		var yearVal int
		if _, err := fmt.Sscanf(year, "%d", &yearVal); err == nil {
			query = query.Where("year = ?", yearVal)
		} else {
			query = query.Where("year = ?", year)
		}
	}

	// Поиск по логину или ФИО
	if search != "" {
		term := "%" + search + "%"
		query = query.Where("login LIKE ? OR full_name LIKE ?", term, term)
	}

	var students []models.User
	query.Order("year ASC, login ASC").Find(&students)
	return students
}

// GetAvailableYears возвращает список годов для выпадающего списка
func GetAvailableYears() []int {
	var years []int
	config.DB.Model(&models.User{}).
		Where("role = ? AND is_active = ?", "student", true).
		Where("year > 0").
		Distinct("year").
		Order("year ASC").
		Pluck("year", &years)
	return years
}

// CreateProject создаёт новый проект
func CreateProject(c *gin.Context) {
	currentUser, _ := c.Get("currentUser")
	year := c.PostForm("filter_year")
	search := c.PostForm("filter_search")
	title := c.PostForm("title")
	description := c.PostForm("description")
	memberIDs := c.PostFormArray("members")

	// Проверка лимита (макс 2 человека)
	if len(memberIDs) > 2 {
		c.HTML(http.StatusOK, "create_project.html", gin.H{
			"user":     currentUser,
			"error":    "Максимальное количество участников: 2 человека",
			"students": getStudents(year, search),
		})
		return
	}

	if len(memberIDs) < 1 {
		c.HTML(http.StatusOK, "create_project.html", gin.H{
			"user":     currentUser,
			"error":    "Проект должен иметь хотя бы одного участника",
			"students": getStudents(year, search),
		})
		return
	}

	project := models.Project{
		Title:         title,
		Description:   description,
		CurrentStatus: "new",
	}

	err := config.DB.Create(&project).Error
	if err != nil {
		c.HTML(http.StatusOK, "create_project.html", gin.H{
			"user":     currentUser,
			"error":    "Ошибка создания проекта: " + err.Error(),
			"students": getStudents(year, search),
		})
		return
	}

	folderPath := filepath.Join("uploads", "projects", fmt.Sprintf("%d", project.ID))
	err = os.MkdirAll(folderPath, 0755)
	if err != nil {
		c.HTML(http.StatusOK, "create_project.html", gin.H{
			"user":         currentUser,
			"error":        "Ошибка создания папки: " + err.Error(),
			"students":     getStudents(year, search),
			"filterYear":   year,
			"filterSearch": search,
		})
		return
	}

	project.FolderPath = folderPath
	config.DB.Save(&project)

	for _, idStr := range memberIDs {
		var member models.User
		config.DB.First(&member, idStr)
		project.Members = append(project.Members, member)
		config.DB.Model(&models.User{}).Where("id = ?", member.ID).Update("project_id", project.ID)
	}
	config.DB.Save(&project)

	c.Redirect(http.StatusSeeOther, "/projects")
}

// ShowMyProjects показывает проекты текущего пользователя
func ShowMyProjects(c *gin.Context) {
	user, _ := c.Get("currentUser")
	currentUser := user.(models.User)

	var projects []models.Project

	if currentUser.Role == "admin" {
		config.DB.Preload("Members").Find(&projects)
	} else {
		config.DB.Table("projects").
			Joins("JOIN project_members ON project_members.project_id = projects.id").
			Where("project_members.user_id = ?", currentUser.ID).
			Preload("Members").
			Find(&projects)
	}

	c.HTML(http.StatusOK, "my_projects.html", gin.H{
		"user":     user,
		"projects": projects,
	})
}

// ShowProjectDetails показывает детали проекта
func ShowProjectDetails(c *gin.Context) {
	user, _ := c.Get("currentUser")
	currentUser := user.(models.User)
	projectID := c.Param("id")

	var project models.Project

	if currentUser.Role == "admin" {
		config.DB.Preload("Members").Preload("Files").Preload("Comments").Preload("Comments.Author").First(&project, projectID)
	} else {
		config.DB.Preload("Members").Preload("Files").Preload("Comments").Preload("Comments.Author").
			Joins("JOIN project_members ON project_members.project_id = projects.id").
			Where("project_members.user_id = ? AND projects.id = ?", currentUser.ID, projectID).
			First(&project)
	}

	if project.ID == 0 {
		c.HTML(http.StatusForbidden, "error.html", gin.H{
			"user":      user,
			"error":     "Доступ к этому проекту запрещён",
			"errorCode": "forbidden",
		})
		return
	}

	c.HTML(http.StatusOK, "project_details.html", gin.H{
		"user":    user,
		"project": project,
	})
}

// AddMemberToProject добавляет студента в проект
func AddMemberToProject(c *gin.Context) {
	user, _ := c.Get("currentUser")
	currentUser := user.(models.User)

	if currentUser.Role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Только администратор может добавлять участников"})
		return
	}

	projectID := c.PostForm("project_id")
	studentID := c.PostForm("student_id")

	var project models.Project
	result := config.DB.Preload("Members").First(&project, projectID)
	if result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Проект не найден"})
		return
	}

	if len(project.Members) >= 2 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Проект уже имеет максимальное количество участников (2 человека)",
		})
		return
	}

	var student models.User
	result = config.DB.First(&student, studentID)
	if result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Студент не найден"})
		return
	}

	var existingMemberCount int64
	config.DB.Table("project_members").
		Where("project_id = ? AND user_id = ?", projectID, studentID).
		Count(&existingMemberCount)

	if existingMemberCount > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Студент уже является участником этого проекта"})
		return
	}

	var otherProjectCount int64
	config.DB.Table("project_members").
		Where("user_id = ? AND project_id != ?", studentID, projectID).
		Count(&otherProjectCount)

	if otherProjectCount > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Студент уже состоит в другом проекте. Сначала удалите его оттуда.",
		})
		return
	}

	config.DB.Model(&project).Association("Members").Append(&student)
	config.DB.Model(&models.User{}).Where("id = ?", studentID).Update("project_id", project.ID)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Студент успешно добавлен в проект",
		"student": student.FullName,
		"project": project.Title,
	})
}

func ShowAddMemberPage(c *gin.Context) {
	user, _ := c.Get("currentUser")
	currentUser := user.(models.User)

	// 🔐 Проверка админа — остаётся здесь, это бизнес-логика хэндлера
	if currentUser.Role != "admin" {
		c.HTML(http.StatusForbidden, "error.html", gin.H{
			"user":      user,
			"error":     "Только администратор может добавлять участников",
			"errorCode": "forbidden",
		})
		return
	}

	projectID := c.Param("id")
	var project models.Project
	if err := config.DB.Preload("Members").First(&project, projectID).Error; err != nil {
		c.HTML(http.StatusNotFound, "error.html", gin.H{"user": user, "error": "Проект не найден", "errorCode": "not_found"})
		return
	}

	filterYear := c.Query("year")
	filterSearch := c.Query("search")

	// 👇 Получаем всех студентов (без исключения участников проекта)
	availableStudents := GetFilteredStudents(filterYear, filterSearch)
	availableYears := GetAvailableYears()

	// 👇 Собираем IDs уже добавленных участников — для фильтрации в шаблоне
	joinedIDs := make(map[uint]bool)
	for _, m := range project.Members {
		joinedIDs[m.ID] = true
	}

	// Преобразуем filterYear в int для корректного сравнения в шаблоне
	var filterYearInt int
	if filterYear != "" {
		fmt.Sscanf(filterYear, "%d", &filterYearInt)
	}

	c.HTML(http.StatusOK, "add_member.html", gin.H{
		"user":              user,
		"project":           project,
		"availableStudents": availableStudents,
		"availableYears":    availableYears,
		"joinedIDs":         joinedIDs,
		"memberCount":       len(project.Members),
		"maxMembers":        2,
		"canAdd":            len(project.Members) < 2,
		"filterYear":        filterYearInt, // 👈 Передаём как int!
		"filterSearch":      filterSearch,
	})
}

// RemoveMemberFromProject удаляет студента из проекта
func RemoveMemberFromProject(c *gin.Context) {
	user, _ := c.Get("currentUser")
	currentUser := user.(models.User)

	if currentUser.Role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Только администратор может удалять участников"})
		return
	}

	projectID := c.PostForm("project_id")
	studentID := c.PostForm("student_id")

	var project models.Project
	config.DB.First(&project, projectID)

	var student models.User
	config.DB.First(&student, studentID)

	config.DB.Model(&project).Association("Members").Delete(&student)
	config.DB.Model(&models.User{}).Where("id = ?", studentID).Update("project_id", 0)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Студент удалён из проекта",
	})
}

// UpdateProjectStatus обновляет статус проекта (только админ)
func UpdateProjectStatus(c *gin.Context) {
	user, _ := c.Get("currentUser")
	currentUser := user.(models.User)

	if currentUser.Role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Только администратор может менять статус"})
		return
	}

	projectID := c.PostForm("project_id")
	newStatus := c.PostForm("status")

	validStatuses := map[string]bool{
		"new": true, "in_progress": true, "review": true,
		"approved": true, "rejected": true,
	}

	if !validStatuses[newStatus] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Недопустимый статус"})
		return
	}

	var project models.Project
	result := config.DB.First(&project, projectID)
	if result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Проект не найден"})
		return
	}

	history := models.ProjectStatusHistory{
		ProjectID: project.ID,
		OldStatus: project.CurrentStatus,
		NewStatus: newStatus,
		ChangedBy: currentUser.ID,
		ChangedAt: time.Now(),
	}
	config.DB.Create(&history)

	project.CurrentStatus = newStatus
	config.DB.Save(&project)

	c.Redirect(http.StatusSeeOther, "/project/"+projectID)
}

// === ВСПОМОГАТЕЛЬНЫЕ ФУНКЦИИ ===

// checkProjectAccess проверяет доступ к проекту
func checkProjectAccess(c *gin.Context, projectID string) bool {
	user, _ := c.Get("currentUser")
	currentUser := user.(models.User)

	if currentUser.Role == "admin" {
		return true
	}

	var count int64
	config.DB.Table("project_members").
		Where("user_id = ? AND project_id = ?", currentUser.ID, projectID).
		Count(&count)

	return count > 0
}

// deleteProject удаляет проект со всеми файлами
func deleteProject(projectID uint) {
	var files []models.File
	config.DB.Where("project_id = ?", projectID).Find(&files)

	for _, file := range files {
		var project models.Project
		config.DB.First(&project, file.ProjectID)
		filePath := filepath.Join(project.FolderPath, file.StorageUUID+filepath.Ext(file.DisplayName))
		os.Remove(filePath)
	}

	config.DB.Where("project_id = ?", projectID).Delete(&models.File{})
	config.DB.Where("project_id = ?", projectID).Delete(&models.Comment{})
	config.DB.Table("project_members").Where("project_id = ?", projectID).Delete(nil)
	config.DB.Delete(&models.Project{}, projectID)
}

// AdminDeleteProject удаляет проект (админ)
func AdminDeleteProject(c *gin.Context) {
	projectID := c.PostForm("project_id")

	var project models.Project
	config.DB.First(&project, projectID)

	if project.ID == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Проект не найден"})
		return
	}

	deleteProject(project.ID)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Проект удалён",
	})
}

// getStudents возвращает список активных студентов (с опциональной фильтрацией)
func getStudents(year, search string) []models.User {
	query := config.DB.Where("role = ? AND is_active = ?", "student", true)

	if year != "" {
		query = query.Where("year = ?", year)
	}
	if search != "" {
		searchTerm := "%" + search + "%"
		query = query.Where("login LIKE ? OR full_name LIKE ?", searchTerm, searchTerm)
	}

	var students []models.User
	query.Order("year ASC, login ASC").Find(&students)
	return students
}

// ShowEditProject показывает страницу редактирования проекта (только админ)
func ShowEditProject(c *gin.Context) {
	user, _ := c.Get("currentUser")
	currentUser := user.(models.User)

	// Проверка: только админ
	if currentUser.Role != "admin" {
		c.HTML(http.StatusForbidden, "error.html", gin.H{
			"user":      user,
			"error":     "Только администратор может редактировать проекты",
			"errorCode": "forbidden",
		})
		return
	}

	projectID := c.Param("id")
	var project models.Project
	result := config.DB.Preload("Members").First(&project, projectID)

	if result.Error != nil {
		c.HTML(http.StatusNotFound, "error.html", gin.H{
			"user":      user,
			"error":     "Проект не найден",
			"errorCode": "not_found",
		})
		return
	}

	c.HTML(http.StatusOK, "edit_project.html", gin.H{
		"user":    user,
		"project": project,
	})
}

// UpdateProject обновляет название и описание проекта (только админ)
func UpdateProject(c *gin.Context) {
	user, _ := c.Get("currentUser")
	currentUser := user.(models.User)

	// Проверка: только админ
	if currentUser.Role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Только администратор может редактировать проекты"})
		return
	}

	projectID := c.Param("id")
	title := c.PostForm("title")
	description := c.PostForm("description")

	// Валидация
	if title == "" {
		c.HTML(http.StatusOK, "edit_project.html", gin.H{
			"user":      user,
			"project":   models.Project{ID: 0}, // заглушка
			"error":     "Название проекта не может быть пустым",
			"errorCode": "validation",
		})
		return
	}

	var project models.Project
	result := config.DB.Preload("Members").First(&project, projectID)
	if result.Error != nil {
		c.HTML(http.StatusNotFound, "error.html", gin.H{
			"user":      user,
			"error":     "Проект не найден",
			"errorCode": "not_found",
		})
		return
	}

	// Обновляем поля
	project.Title = title
	project.Description = description
	config.DB.Save(&project)

	// Редирект обратно на список проектов
	c.Redirect(http.StatusSeeOther, "/projects")
}
func CalculateProjectStatus(projectID uint) string {
	var files []models.File
	config.DB.Where("project_id = ? AND is_active = ?", projectID, true).Find(&files)

	if len(files) == 0 {
		return config.ProjectStatusNew
	}

	// Есть файлы на проверку?
	hasPending := false
	for _, f := range files {
		if f.Status == config.FileStatusUploaded || f.Status == config.FileStatusUpdated {
			hasPending = true
			break
		}
	}
	if hasPending {
		return config.ProjectStatusNeedsReview
	}

	// Считаем завершённые типы
	var completedTypes int64
	config.DB.Model(&models.ProjectDocumentType{}).
		Where("project_id = ? AND type_code IN ? AND is_complete = ?",
			projectID, config.DefaultRequiredDocuments, true).
		Count(&completedTypes)

	totalTypes := len(config.DefaultRequiredDocuments)

	if int(completedTypes) == totalTypes {
		return config.ProjectStatusCompleted
	}
	if completedTypes > 0 {
		// Есть хоть один завершённый тип, но не все
		// Проверяем, есть ли файлы в статусе revision
		var revisionCount int64
		config.DB.Model(&models.File{}).
			Where("project_id = ? AND status = ? AND is_active = ?", projectID, config.FileStatusRevision, true).
			Count(&revisionCount)

		if revisionCount > 0 {
			return config.ProjectStatusRevision
		}
		return config.ProjectStatusApproved // Частично согласовано
	}

	return config.ProjectStatusNeedsReview
}
