package mysql

import (
	modelsdb "go-stock-prediction/pkg/models/models_db"
)

func (c *Client) CreateUser(user *modelsdb.User) error {
	return c.Db.Create(user).Error
}

func (c *Client) GetUserByUsername(username string) (*modelsdb.User, error) {
	var user modelsdb.User
	err := c.Db.Where("username = ?", username).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (c *Client) GetUserByID(id uint) (*modelsdb.User, error) {
	var user modelsdb.User
	err := c.Db.First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (c *Client) GetAllUsers() ([]modelsdb.User, error) {
	var users []modelsdb.User
	err := c.Db.Order("created_at ASC").Find(&users).Error
	return users, err
}

func (c *Client) DeleteUser(id uint) error {
	return c.Db.Delete(&modelsdb.User{}, id).Error
}

func (c *Client) AdminExists() (bool, error) {
	var count int64
	err := c.Db.Model(&modelsdb.User{}).Where("role = ?", "admin").Count(&count).Error
	return count > 0, err
}

func (c *Client) UpdateUserPassword(id uint, passwordHash string) error {
	return c.Db.Model(&modelsdb.User{}).Where("id = ?", id).Update("password_hash", passwordHash).Error
}
