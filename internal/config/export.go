package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// GetExportDir возвращает директорию для экспорта файлов
func GetExportDir() string {
	dir := os.Getenv("EXPORT_DIR")
	if dir == "" {
		dir = "exports"
	}
	// Создаём папку, если нет
	os.MkdirAll(dir, 0755)
	return dir
}

// GenerateExportFilename создаёт понятное имя файла
func GenerateExportFilename(group, login, prefix string) string {
	// Очищаем от спецсимволов
	safeGroup := strings.ReplaceAll(group, "/", "-")
	safeLogin := strings.ReplaceAll(login, "_", "")
	timestamp := time.Now().Format("20060102")

	return fmt.Sprintf("%s_%s_%s_%s.csv", safeGroup, safeLogin, prefix, timestamp)
}
