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
		// Админ уже есть
		return nil
	}

	// Создаём админа
	admin = models.User{
		Login:              "admin",
		FullName:           "Administrator",
		Role:               "admin",
		Year:               0,
		Group:              "",
		IsActive:           true,
		MustChangePassword: false, // ← Админу не нужно менять пароль
	}

	// Пароль по умолчанию - admin123 (первый вход)
	err := admin.SetPassword("admin123")
	if err != nil {
		return err
	}

	return config.DB.Create(&admin).Error
}

// ImportUsersFromCSV импортирует студентов из CSV файла
// Ожидаемый формат CSV:
// Фамилия,Имя,Отчество,Год,Группа
// Иванов,Иван,Иванович,22,ИВТ-22-01
func ImportUsersFromCSV(filePath string) ([]models.User, error) {
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
	var passwords []string // Сохраняем пароли для экспорта
	rowNumber := 1

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		// Ожидаем: Фамилия, Имя, Отчество, Год, Группа
		if len(record) < 5 {
			continue // Пропускаем строки с недостаточным количеством полей
		}

		lastname := record[0]
		firstname := record[1]
		patronymic := record[2]
		year := 22         // По умолчанию, можно парсить из record[3]
		group := record[4] // ← НОВОЕ: читаем группу

		// Парсим год, если он есть в CSV
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

		// Сохраняем пароль для последующего экспорта
		passwords = append(passwords, password)

		err = config.DB.Create(&user).Error
		if err != nil {
			return nil, err
		}

		users = append(users, user)
		rowNumber++
	}

	// Сохраняем пароли во временный файл для экспорта
	err = exportPasswords(users, passwords, "exported_passwords.csv")
	if err != nil {
		return nil, err
	}

	return users, nil
}

// exportPasswords создаёт CSV файл с логинами и паролями для раздачи
func exportPasswords(users []models.User, passwords []string, outputPath string) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Заголовок
	writer.Write([]string{"Login", "FullName", "Group", "Password"})

	// Данные
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
