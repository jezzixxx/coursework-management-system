package config

// === Статусы файлов ===
const (
	FileStatusUploaded = "uploaded" // Загружен студентом
	FileStatusUpdated  = "updated"  // Обновлён студентом
	FileStatusAccepted = "accepted" // Принят админом
	FileStatusRevision = "revision" // Возвращён на доработку
)

// === Статусы проектов ===
const (
	ProjectStatusNew         = "new"          // Только создан
	ProjectStatusNeedsReview = "needs_review" // Есть файлы на проверку
	ProjectStatusRevision    = "revision"     // Есть файлы на доработку
	ProjectStatusApproved    = "approved"     // Частично согласовано
	ProjectStatusCompleted   = "completed"    // Все обязательные типы завершены
)

// === Мапы для фронтенда ===
var FileStatusLabels = map[string]string{
	FileStatusUploaded: "Загружен",
	FileStatusUpdated:  "Обновлён",
	FileStatusAccepted: "Принят",
	FileStatusRevision: "На доработку",
}

var ProjectStatusLabels = map[string]string{
	ProjectStatusNew:         "Новый",
	ProjectStatusNeedsReview: "Нуждается в проверке",
	ProjectStatusRevision:    "На доработку",
	ProjectStatusApproved:    "Согласовано",
	ProjectStatusCompleted:   "Завершён",
}
