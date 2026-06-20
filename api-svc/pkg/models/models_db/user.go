package modelsdb

import "gorm.io/gorm"

type User struct {
	gorm.Model
	Username     string `gorm:"uniqueIndex;not null;size:50"`
	PasswordHash string `gorm:"not null"`
	Role         string `gorm:"not null;default:'user'"` // "admin" or "user"
}
