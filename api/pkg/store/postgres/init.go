package postgres

import (
	"fmt"
	"go-stock-prediction/pkg/logger"
	"go-stock-prediction/pkg/models/models_config"
	modelsdb "go-stock-prediction/pkg/models/models_db"

	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Client struct {
	Db  *gorm.DB
	cfg models_config.PostgresConfig
}

var (
	client *Client
)

func GetInstance() *Client {
	if client == nil {
		client = &Client{}
	}
	return client
}

func (c *Client) Init(cfg models_config.DatabaseConfig) error {
	pg := cfg.Pgsql
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Asia/Ho_Chi_Minh",
		pg.Host, pg.User, pg.Password, pg.DbName, pg.Port,
	)
	gormLogger := logger.NewGormLogger(pg.Debug)
	db, err := gorm.Open(gormpostgres.Open(dsn), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		logger.Logger.Debugf("Error connecting to PostgreSQL database: error=%v", err)
		return err
	}
	dsnForLog := fmt.Sprintf("host=%s user=%s dbname=%s port=%s", pg.Host, pg.User, pg.DbName, pg.Port)
	logger.Logger.Infof("Connect to PostgreSQL database: %s", dsnForLog)
	c.Db = db
	c.cfg = pg

	if err := db.AutoMigrate(modelsdb.AllModels...); err != nil {
		logger.Logger.Errorf("AutoMigrate failed: %v", err)
		return err
	}
	logger.Logger.Info("AutoMigrate completed successfully")

	return nil
}

func (c *Client) Ping() error {
	sql, err := c.Db.DB()
	if err != nil {
		return err
	}
	return sql.Ping()
}

func (c *Client) BeginTransaction() *Client {
	tx := c.Db.Begin()
	return &Client{Db: tx, cfg: c.cfg}
}

func (c *Client) Commit() error {
	return c.Db.Commit().Error
}

func (c *Client) Rollback() error {
	return c.Db.Rollback().Error
}
