package utils

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"unicode"
)

// GenerateSecurePassword генерирует случайный пароль длиной 16 символов
func GenerateSecurePassword() (string, error) {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
	result := make([]byte, 16)
	for i := 0; i < 16; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			return "", err
		}
		result[i] = chars[num.Int64()]
	}
	return string(result), nil
}

// GenerateLogin создаёт логин по формату: фамилия_и_о_год_номерстроки
func GenerateLogin(lastname, firstname, patronymic string, year int, rowNumber int) string {
	fInitial := getFirstLetter(firstname)
	pInitial := getFirstLetter(patronymic)
	cleanLastname := sanitize(lastname)

	return strings.ToLower(fmt.Sprintf("%s_%s%s_%d_%02d", cleanLastname, fInitial, pInitial, year, rowNumber))
}

func getFirstLetter(s string) string {
	for _, r := range s {
		if unicode.IsLetter(r) {
			return strings.ToLower(string(r))
		}
	}
	return "x"
}

func sanitize(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
