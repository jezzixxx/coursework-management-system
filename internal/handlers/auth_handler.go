package handlers

import (
	"coursework/internal/config"
	"coursework/internal/models"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ShowLoginPage показывает страницу входа
func ShowLoginPage(c *gin.Context) {
	c.HTML(http.StatusOK, "login.html", gin.H{
		"error": "",
	})
}

// Login обрабатывает вход
func Login(c *gin.Context) {
	login := c.PostForm("login")
	password := c.PostForm("password")

	// Ищем пользователя
	var user models.User
	result := config.DB.Where("login = ? AND is_active = ?", login, true).First(&user)
	if result.Error != nil {
		c.HTML(http.StatusOK, "login.html", gin.H{
			"error":     "Неверный логин или пароль",
			"errorCode": "auth",
		})
		return
	}

	// Проверяем пароль (сравнение хэша)
	if !user.CheckPassword(password) {
		c.HTML(http.StatusOK, "login.html", gin.H{
			"error":     "Неверный логин или пароль",
			"errorCode": "auth",
		})
		return
	}

	// Устанавливаем куку с флагами безопасности!
	c.SetCookie("user_login", user.Login, 3600*24, "/", "", false, true)
	// Параметры: имя, значение, время жизни (сек), путь, домен, secure, httpOnly

	c.Redirect(http.StatusSeeOther, "/dashboard")
}

// Logout обрабатывает выход
func Logout(c *gin.Context) {
	c.SetCookie("user_login", "", -1, "/", "", false, true)
	c.Redirect(http.StatusTemporaryRedirect, "/login")
}

// ShowChangePassword показывает страницу смены пароля
func ShowChangePassword(c *gin.Context) {
	user, _ := c.Get("currentUser")
	u := user.(models.User)

	c.HTML(http.StatusOK, "change_password.html", gin.H{
		"user":               user,
		"mustChangePassword": u.MustChangePassword, // ← передаём флаг в шаблон
		"error":              "",
	})
}

// ChangePassword обрабатывает смену пароля
func ChangePassword(c *gin.Context) {
	user, _ := c.Get("currentUser")
	currentUser := user.(models.User)

	newPassword := c.PostForm("new_password")
	confirmPassword := c.PostForm("confirm_password")

	// Базовые валидации
	if newPassword != confirmPassword {
		c.HTML(http.StatusOK, "change_password.html", gin.H{
			"user": user, "mustChangePassword": currentUser.MustChangePassword,
			"error":     "Новые пароли не совпадают",
			"errorCode": "validation",
		})
		return
	}
	if len(newPassword) < 8 {
		c.HTML(http.StatusOK, "change_password.html", gin.H{
			"user": user, "mustChangePassword": currentUser.MustChangePassword,
			"error":     "Пароль должен быть не менее 8 символов",
			"errorCode": "validation",
		})
		return
	}

	// === ГЛАВНОЕ ИСПРАВЛЕНИЕ ===
	// Если это принудительная смена (первый вход/сброс админом), старый пароль НЕ проверяем.
	// Если обычная смена пользователем — проверяем обязательно.
	if !currentUser.MustChangePassword {
		oldPassword := c.PostForm("old_password")
		if !currentUser.CheckPassword(oldPassword) {
			c.HTML(http.StatusOK, "change_password.html", gin.H{
				"user": user, "mustChangePassword": currentUser.MustChangePassword,
				"error":     "Неверный текущий пароль",
				"errorCode": "auth",
			})
			return
		}
	}

	// Обновляем запись в БД напрямую
	var userToUpdate models.User
	config.DB.First(&userToUpdate, currentUser.ID)

	err := userToUpdate.SetPassword(newPassword)
	if err != nil {
		c.HTML(http.StatusOK, "change_password.html", gin.H{
			"user": user, "mustChangePassword": currentUser.MustChangePassword,
			"error":     "Ошибка сохранения пароля: " + err.Error(),
			"errorCode": "system",
		})
		return
	}

	// Снимаем флаг обязательной смены
	userToUpdate.MustChangePassword = false
	config.DB.Save(&userToUpdate)

	c.Redirect(http.StatusSeeOther, "/dashboard")
}
