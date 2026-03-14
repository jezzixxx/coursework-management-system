package handlers

import (
	"coursework/internal/config"
	"coursework/internal/models"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// AddComment добавляет комментарий к проекту
func AddComment(c *gin.Context) {
	user, _ := c.Get("currentUser")
	currentUser := user.(models.User)

	projectID := c.PostForm("project_id")
	text := c.PostForm("text")

	if text == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Комментарий не может быть пустым"})
		return
	}

	// Проверяем доступ к проекту
	if !checkProjectAccess(c, projectID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Доступ запрещён"})
		return
	}

	// Получаем проект для проверки
	var project models.Project
	config.DB.First(&project, projectID)

	// Создаём комментарий
	comment := models.Comment{
		ProjectID: project.ID, // ← ИСПРАВЛЕНО: используем project.ID
		AuthorID:  currentUser.ID,
		Text:      text,
		CreatedAt: time.Now(),
	}

	config.DB.Create(&comment)

	c.Redirect(http.StatusSeeOther, "/project/"+projectID)
}

// DeleteComment удаляет комментарий (только админ или автор)
func DeleteComment(c *gin.Context) {
	user, _ := c.Get("currentUser")
	currentUser := user.(models.User)

	commentID := c.PostForm("comment_id")
	projectID := c.PostForm("project_id")

	var comment models.Comment
	config.DB.First(&comment, commentID)

	// Проверка прав (админ или автор)
	if currentUser.Role != "admin" && comment.AuthorID != currentUser.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Нет прав на удаление"})
		return
	}

	config.DB.Delete(&comment)

	c.Redirect(http.StatusSeeOther, "/project/"+projectID)
}
