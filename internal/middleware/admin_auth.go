package middleware

import (
	"coursework/internal/models"
	"net/http"

	"github.com/gin-gonic/gin"
)

// AdminReauthMiddleware требует повторного ввода пароля для критических операций
func AdminReauthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := c.Get("currentUser")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Требуется авторизация"})
			c.Abort()
			return
		}

		currentUser := user.(models.User)
		if currentUser.Role != "admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Требуется права администратора"})
			c.Abort()
			return
		}

		// Проверяем, подтвердил ли админ пароль в этой сессии
		reauthConfirmed, _ := c.Cookie("admin_reauth_confirmed")
		if reauthConfirmed != "true" {
			// Для AJAX-запросов возвращаем JSON
			if c.GetHeader("X-Requested-With") == "XMLHttpRequest" ||
				c.GetHeader("Accept") == "application/json" {
				c.JSON(http.StatusUnauthorized, gin.H{
					"error":           "Требуется подтверждение пароля",
					"reauth_required": true,
				})
				c.Abort()
				return
			}

			// Для обычных запросов — редирект
			c.Redirect(http.StatusTemporaryRedirect, "/admin/confirm-password")
			c.Abort()
			return
		}

		c.Next()
	}
}

// ConfirmAdminPassword подтверждает пароль администратора (HTML форма)
func ConfirmAdminPassword(c *gin.Context) {
	user, _ := c.Get("currentUser")
	currentUser := user.(models.User)

	password := c.PostForm("password")
	nextURL := c.PostForm("next")

	if !currentUser.CheckPassword(password) {
		c.HTML(http.StatusOK, "admin_reauth.html", gin.H{
			"user":  user,
			"error": "Неверный пароль",
			"next":  nextURL,
		})
		return
	}

	// Устанавливаем куку подтверждения (на 5 минут)
	c.SetCookie("admin_reauth_confirmed", "true", 300, "/", "", false, true)

	// Редирект на следующую страницу
	if nextURL != "" {
		c.Redirect(http.StatusTemporaryRedirect, nextURL)
	} else {
		c.Redirect(http.StatusTemporaryRedirect, "/admin")
	}
}

// ConfirmAdminPasswordJSON подтверждает пароль для AJAX-запросов
func ConfirmAdminPasswordJSON(c *gin.Context) {
	user, _ := c.Get("currentUser")
	currentUser := user.(models.User)

	password := c.PostForm("password")

	if !currentUser.CheckPassword(password) {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "Неверный пароль",
		})
		return
	}

	// Устанавливаем куку подтверждения (на 5 минут)
	c.SetCookie("admin_reauth_confirmed", "true", 300, "/", "", false, true)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Пароль подтверждён",
	})
}

// ClearAdminReauth очищает подтверждение после операции
func ClearAdminReauth(c *gin.Context) {
	c.SetCookie("admin_reauth_confirmed", "", -1, "/", "", false, true)
}
