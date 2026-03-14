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
			"error": "Неверный логин или пароль",
		})
		return
	}

	// Проверяем пароль (сравнение хэша)
	if !user.CheckPassword(password) {
		c.HTML(http.StatusOK, "login.html", gin.H{
			"error": "Неверный логин или пароль",
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
	c.HTML(http.StatusOK, "change_password.html", gin.H{
		"user":  user,
		"error": "",
	})
}

// ChangePassword обрабатывает смену пароля
func ChangePassword(c *gin.Context) {
	user, _ := c.Get("currentUser")
	currentUser := user.(models.User)

	oldPassword := c.PostForm("old_password")
	newPassword := c.PostForm("new_password")
	confirmPassword := c.PostForm("confirm_password")

	// Проверяем старый пароль
	if !currentUser.CheckPassword(oldPassword) {
		c.HTML(http.StatusOK, "change_password.html", gin.H{
			"user":  user,
			"error": "Неверный текущий пароль",
		})
		return
	}

	// Проверяем совпадение новых паролей
	if newPassword != confirmPassword {
		c.HTML(http.StatusOK, "change_password.html", gin.H{
			"user":  user,
			"error": "Новые пароли не совпадают",
		})
		return
	}

	// Проверяем сложность пароля (минимум 8 символов)
	if len(newPassword) < 8 {
		c.HTML(http.StatusOK, "change_password.html", gin.H{
			"user":  user,
			"error": "Пароль должен быть не менее 8 символов",
		})
		return
	}

	// === ИСПРАВЛЕНИЕ: Обновляем запись в БД напрямую ===
	var userToUpdate models.User
	config.DB.First(&userToUpdate, currentUser.ID)

	err := userToUpdate.SetPassword(newPassword)
	if err != nil {
		c.HTML(http.StatusOK, "change_password.html", gin.H{
			"user":  user,
			"error": "Ошибка сохранения пароля: " + err.Error(),
		})
		return
	}

	// Снимаем флаг обязательной смены
	userToUpdate.MustChangePassword = false

	// Сохраняем в базу
	config.DB.Save(&userToUpdate)

	// Редирект на главную
	c.Redirect(http.StatusSeeOther, "/my-projects")
}
