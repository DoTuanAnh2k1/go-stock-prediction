package repository

import modelsdb "go-stock-prediction/pkg/models/models_db"

// UserStore - interface cho user operations
type UserStore interface {
	CreateUser(user *modelsdb.User) error
	GetUserByUsername(username string) (*modelsdb.User, error)
	GetUserByID(id uint) (*modelsdb.User, error)
	GetAllUsers() ([]modelsdb.User, error)
	DeleteUser(id uint) error
	AdminExists() (bool, error)
	UpdateUserPassword(id uint, passwordHash string) error
}
