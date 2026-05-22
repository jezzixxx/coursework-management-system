package service

import (
	"coursework/internal/config"
	"coursework/internal/models"
	"coursework/internal/utils"
	"encoding/csv"
	"fmt"
	"io"
	"os"
)

// CreateAdmin создаёт первого администратора, если его нет
func CreateAdmin() error {
	var admin models.User
	result := config.DB.Where("role = ?", "admin").First(&admin)

	if result.Error == nil {
		return nil
	}

	admin = models.User{
		Login:              "admin",
		FullName:           "Administrator",
		Role:               "admin",
		Year:               0,
		Group:              "",
		IsActive:           true,
		MustChangePassword: false,
	}

	err := admin.SetPassword("admin123")
	if err != nil {
		return err
	}

	return config.DB.Create(&admin).Error
}

// ImportUsersFromCSV импортирует студентов и записывает пароли в предоставленный writer
// Возвращает список созданных пользователей и ошибку
func ImportUsersFromCSV(filePath string, passwordsWriter io.Writer) ([]models.User, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	// Пропускаем заголовок
	_, err = reader.Read()
	if err != nil {
		return nil, err
	}

	var users []models.User
	var passwords []string
	rowNumber := 1

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if len(record) < 5 {
			continue
		}

		// ... (парсинг, генерация логина/пароля, создание пользователя) ...
		lastname := record[0]
		firstname := record[1]
		patronymic := record[2]
		year := 22
		group := record[4]

		if len(record[3]) > 0 {
			fmt.Sscanf(record[3], "%d", &year)
		}

		login := utils.GenerateLogin(lastname, firstname, patronymic, year, rowNumber)
		password, err := utils.GenerateSecurePassword()
		if err != nil {
			return nil, err
		}

		user := models.User{
			Login:              login,
			FullName:           fmt.Sprintf("%s %s %s", lastname, firstname, patronymic),
			Role:               "student",
			Year:               year,
			Group:              group,
			IsActive:           true,
			MustChangePassword: true,
		}

		err = user.SetPassword(password)
		if err != nil {
			return nil, err
		}

		// После успешного создания пользователя:
		passwords = append(passwords, password)
		err = config.DB.Create(&user).Error
		if err != nil {
			return nil, err
		}
		users = append(users, user)
		rowNumber++
	}

	// 🔥 Ключевое изменение: пишем пароли в переданный writer, а не в файл!
	if passwordsWriter != nil {
		if err := exportPasswordsToWriter(users, passwords, passwordsWriter); err != nil {
			return nil, fmt.Errorf("ошибка записи паролей: %w", err)
		}
	}

	return users, nil
}

// exportPasswordsToWriter — приватная функция, пишет CSV в любой io.Writer
func exportPasswordsToWriter(users []models.User, passwords []string, w io.Writer) error {
	writer := csv.NewWriter(w)
	defer writer.Flush()

	writer.Write([]string{"Login", "FullName", "Group", "Password"})
	for i, user := range users {
		writer.Write([]string{
			user.Login,
			user.FullName,
			user.Group,
			passwords[i],
		})
	}
	return nil
}

// exportPasswords создаёт CSV файл с логинами и паролями (приватная функция)
func exportPasswords(users []models.User, passwords []string, outputPath string) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	writer.Write([]string{"Login", "FullName", "Group", "Password"})

	for i, user := range users {
		writer.Write([]string{
			user.Login,
			user.FullName,
			user.Group,
			passwords[i], // ← Берём из слайса, НЕ из модели!
		})
	}

	return nil
}
