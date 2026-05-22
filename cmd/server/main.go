package main

import (
	"coursework/internal/config"
	"coursework/internal/handlers"
	"coursework/internal/middleware"
	"coursework/internal/models"
	"coursework/internal/service"
	"log"
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

func main() {
	// 1. Подключаем базу данных
	config.InitDB()

	// 2. Миграция таблиц
	err := config.DB.AutoMigrate(
		&models.User{},
		&models.Project{},
		&models.File{},
		&models.Comment{},
		&models.ProjectStatusHistory{},
		&models.ProjectDocumentType{},
	)
	if err != nil {
		log.Fatal("Failed to migrate database:", err)
	}
	log.Println("Database tables created successfully!")

	// 3. Создаём админа
	err = service.CreateAdmin()
	if err != nil {
		log.Fatal("Failed to create admin:", err)
	}
	log.Println("Admin user ready (login: admin, password: admin123)")

	// 4. Создаем веб-сервер Gin
	r := gin.Default()

	// Загружаем HTML шаблоны
	root, _ := filepath.Abs(".")
	r.LoadHTMLGlob(filepath.Join(root, "templates/*"))

	r.Static("/static", filepath.Join(root, "static"))

	// === Публичные маршруты (без авторизации) ===
	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusTemporaryRedirect, "/login")
	})
	r.GET("/login", handlers.ShowLoginPage)

	// ← ТОЛЬКО ОДИН РАЗ! С Rate Limiting
	r.POST("/login", middleware.RateLimitMiddleware(), handlers.Login)

	// === Защищённые маршруты ===
	protected := r.Group("/")
	protected.Use(middleware.AuthMiddleware())
	protected.Use(middleware.PasswordChangeMiddleware())
	{
		protected.GET("/dashboard", func(c *gin.Context) {
			user, _ := c.Get("currentUser")
			c.HTML(http.StatusOK, "dashboard.html", gin.H{
				"user": user,
			})
		})
		protected.GET("/logout", handlers.Logout)

		// Мои проекты (для всех)
		protected.GET("/my-projects", handlers.ShowMyProjects)
		protected.GET("/project/:id", handlers.ShowProjectDetails)

		// Смена пароля
		protected.GET("/change-password", handlers.ShowChangePassword)
		protected.POST("/change-password", handlers.ChangePassword)

		// Файлы
		protected.GET("/project/:id/files", handlers.ShowProjectFiles)
		protected.POST("/files/upload", handlers.UploadFile)
		protected.GET("/files/:id/download", handlers.DownloadFile)
		protected.GET("/files/:id/history", handlers.ShowFileHistory)

		// Комментарии
		protected.POST("/comments/add", handlers.AddComment)
		protected.POST("/comments/delete", handlers.DeleteComment)
		protected.GET("/project/:id/checklist", handlers.ShowChecklist)

		// === Маршруты только для админа ===
		admin := protected.Group("/")
		admin.Use(middleware.AdminMiddleware())
		{
			admin.GET("/admin", handlers.ShowAdminDashboard)
			admin.GET("/import", handlers.ShowImportPage)
			admin.POST("/import", handlers.ImportUsers)

			// Проекты (админ)
			admin.GET("/projects", handlers.ShowProjects)
			admin.GET("/projects/create", handlers.ShowCreateProject)
			admin.POST("/projects/create", handlers.CreateProject)

			// Управление пользователями (админ)
			admin.GET("/admin/users", handlers.ShowUsersPage)
			admin.POST("/admin/users/reset-password", handlers.AdminResetPassword)

			// Управление участниками проекта (админ)
			admin.GET("/projects/:id/add-member", handlers.ShowAddMemberPage)
			admin.POST("/admin/projects/add-member", handlers.AddMemberToProject)
			admin.POST("/admin/projects/remove-member", handlers.RemoveMemberFromProject)

			// Статус проекта
			admin.POST("/admin/projects/update-status", handlers.UpdateProjectStatus)
			admin.POST("/admin/files/:id/review", handlers.ReviewFile)
			// Подтверждение пароля админа
			admin.POST("/admin/confirm-password", middleware.ConfirmAdminPassword)

			// Удаление с подтверждением
			admin.POST("/admin/users/delete", middleware.AdminReauthMiddleware(), handlers.AdminDeleteUser)
			admin.POST("/admin/projects/delete", middleware.AdminReauthMiddleware(), handlers.AdminDeleteProject)
			protected.POST("/files/:id/delete", handlers.DeleteFile)
			// В блок admin добавь:
			admin.POST("/admin/confirm-password-json", middleware.ConfirmAdminPasswordJSON)
			// === Редактирование проекта (только админ) ===
			admin.GET("/projects/:id/edit", handlers.ShowEditProject)
			admin.POST("/projects/:id/edit", handlers.UpdateProject)
			admin.GET("/download-passwords", handlers.DownloadPasswords)
			admin.GET("/download-reset-passwords", handlers.DownloadResetPasswords)
			// Управление чек-листом проекта (toggle)
			admin.POST("/admin/projects/:id/documents/:code/complete", handlers.ToggleDocumentTypeComplete)
		}
	}

	// 5. Запускаем сервер
	log.Println("Server starting on http://localhost:8000")
	err = r.Run(":8000")
	if err != nil {
		log.Fatal("Failed to start server:", err)
	}
}
