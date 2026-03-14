package models

import (
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID                 uint   `gorm:"primaryKey"`
	Login              string `gorm:"uniqueIndex;not null;size:50"`
	PasswordHash       string `gorm:"not null;size:255"`
	Role               string `gorm:"not null;size:20"`
	FullName           string `gorm:"size:100"`
	Year               int
	Group              string `gorm:"size:20"`
	IsActive           bool   `gorm:"default:true"`
	MustChangePassword bool   `gorm:"default:true"`
}

// SetPassword хэширует пароль перед сохранением (Безопасность!)
func (u *User) SetPassword(password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.PasswordHash = string(hash)
	return nil
}

// CheckPassword сравнивает введённый пароль с хэшем в базе
func (u *User) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password))
	return err == nil
}
