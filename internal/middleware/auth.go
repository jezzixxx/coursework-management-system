package middleware

import (
	"coursework/internal/config"
	"coursework/internal/models"
	"net/http"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware проверяет авторизацию пользователя
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		login, err := c.Cookie("user_login")
		if err != nil {
			c.Redirect(http.StatusTemporaryRedirect, "/login")
			c.Abort()
			return
		}

		var user models.User
		result := config.DB.Where("login = ? AND is_active = ?", login, true).First(&user)
		if result.Error != nil {
			c.Redirect(http.StatusTemporaryRedirect, "/login")
			c.Abort()
			return
		}

		// ← ВАЖНО: Правильно сохраняем пользователя в контекст
		c.Set("currentUser", user)
		c.Set("userName", user.FullName) // ← Дополнительно для шаблонов
		c.Next()
	}
}

// PasswordChangeMiddleware проверяет смену пароля для студентов
func PasswordChangeMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := c.Get("currentUser")
		if !exists {
			c.Redirect(http.StatusTemporaryRedirect, "/login")
			c.Abort()
			return
		}

		currentUser := user.(models.User)

		// Только для студентов!
		if currentUser.Role == "student" && currentUser.MustChangePassword {
			if c.Request.URL.Path != "/change-password" {
				c.Redirect(http.StatusTemporaryRedirect, "/change-password")
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

// AdminMiddleware проверяет права администратора
func AdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := c.Get("currentUser")
		if !exists {
			c.Redirect(http.StatusTemporaryRedirect, "/login")
			c.Abort()
			return
		}

		currentUser := user.(models.User)
		if currentUser.Role != "admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Требуется права администратора"})
			c.Abort()
			return
		}

		c.Next()
	}
}
