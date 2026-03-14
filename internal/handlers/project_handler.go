package handlers

import (
	"coursework/internal/config"
	"coursework/internal/models"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
)

// ShowProjects показывает список всех проектов (админ)
func ShowProjects(c *gin.Context) {
	user, _ := c.Get("currentUser")

	var projects []models.Project
	config.DB.Preload("Members").Find(&projects)

	c.HTML(http.StatusOK, "projects.html", gin.H{
		"user":     user,
		"projects": projects,
	})
}

// ShowCreateProject показывает страницу создания проекта
func ShowCreateProject(c *gin.Context) {
	user, _ := c.Get("currentUser")

	var students []models.User
	config.DB.Where("role = ? AND is_active = ?", "student", true).Find(&students)

	c.HTML(http.StatusOK, "create_project.html", gin.H{
		"user":     user,
		"students": students,
	})
}

// CreateProject создаёт новый проект
func CreateProject(c *gin.Context) {
	currentUser, _ := c.Get("currentUser")

	title := c.PostForm("title")
	description := c.PostForm("description")
	memberIDs := c.PostFormArray("members")

	// Проверка лимита (макс 2 человека)
	if len(memberIDs) > 2 {
		c.HTML(http.StatusOK, "create_project.html", gin.H{
			"user":     currentUser,
			"error":    "Максимальное количество участников: 2 человека",
			"students": getStudents(),
		})
		return
	}

	if len(memberIDs) < 1 {
		c.HTML(http.StatusOK, "create_project.html", gin.H{
			"user":     currentUser,
			"error":    "Проект должен иметь хотя бы одного участника",
			"students": getStudents(),
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
			"students": getStudents(),
		})
		return
	}

	folderPath := filepath.Join("uploads", "projects", fmt.Sprintf("%d", project.ID))
	err = os.MkdirAll(folderPath, 0755)
	if err != nil {
		c.HTML(http.StatusOK, "create_project.html", gin.H{
			"user":     currentUser,
			"error":    "Ошибка создания папки: " + err.Error(),
			"students": getStudents(),
		})
		return
	}

	project.FolderPath = folderPath
	config.DB.Save(&project)

	for _, idStr := range memberIDs {
		var member models.User
		config.DB.First(&member, idStr)
		project.Members = append(project.Members, member)
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
			"user":  user,
			"error": "Доступ к этому проекту запрещён",
		})
		return
	}

	c.HTML(http.StatusOK, "project_details.html", gin.H{
		"user":    user,
		"project": project,
	})
}

// ShowAddMemberPage показывает страницу добавления участника
func ShowAddMemberPage(c *gin.Context) {
	user, _ := c.Get("currentUser")
	projectID := c.Param("id")

	currentUser := user.(models.User)
	if currentUser.Role != "admin" {
		c.HTML(http.StatusForbidden, "error.html", gin.H{
			"user":  user,
			"error": "Только администратор может добавлять участников",
		})
		return
	}

	var project models.Project
	config.DB.Preload("Members").First(&project, projectID)

	memberCount := len(project.Members)

	var availableStudents []models.User
	config.DB.Where("role = ? AND is_active = ?", "student", true).
		Not("id IN (?)", config.DB.Table("project_members").
			Select("user_id").
			Where("project_id = ?", projectID)).
		Find(&availableStudents)

	c.HTML(http.StatusOK, "add_member.html", gin.H{
		"user":              user,
		"project":           project,
		"availableStudents": availableStudents,
		"memberCount":       memberCount,
		"maxMembers":        2,
		"canAdd":            memberCount < 2,
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

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Студент успешно добавлен в проект",
		"student": student.FullName,
		"project": project.Title,
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

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Статус обновлён",
	})
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

// getStudents возвращает список всех активных студентов
func getStudents() []models.User {
	var students []models.User
	config.DB.Where("role = ? AND is_active = ?", "student", true).Find(&students)
	return students
}
