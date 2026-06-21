package store

import "time"

// Instance is the persisted registration row. Source of truth in Postgres;
// service-mgt mirrors it into an in-memory cache (write-through).
type Instance struct {
	ID           int64             `gorm:"primaryKey;column:id"`
	ServiceName  string            `gorm:"column:service_name"`
	InstanceID   string            `gorm:"column:instance_id;uniqueIndex"`
	Address      string            `gorm:"column:address"`
	Port         int32             `gorm:"column:port"`
	Metadata     map[string]string `gorm:"column:metadata;serializer:json"`
	Status       string            `gorm:"column:status"`
	TTLSeconds   int32             `gorm:"column:ttl_seconds"`
	LastSeen     time.Time         `gorm:"column:last_seen"`
	RegisteredAt time.Time         `gorm:"column:registered_at"`
	UpdatedAt    time.Time         `gorm:"column:updated_at"`
}

func (Instance) TableName() string { return "service_instances" }
