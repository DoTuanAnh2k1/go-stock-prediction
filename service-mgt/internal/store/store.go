package store

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Store is the persistence layer for service registrations. All writes are
// write-through: the registry core calls these before updating its cache.
type Store interface {
	Upsert(inst *Instance) error
	UpdateStatus(instanceID, status string) error
	Delete(instanceID string) error
	FlushLastSeen(seen map[string]time.Time) error
	LoadAll() ([]*Instance, error)
}

type gormStore struct{ db *gorm.DB }

func NewGorm(db *gorm.DB) Store { return &gormStore{db: db} }

func (g *gormStore) Upsert(inst *Instance) error {
	now := time.Now()
	if inst.RegisteredAt.IsZero() {
		inst.RegisteredAt = now
	}
	inst.UpdatedAt = now
	if inst.LastSeen.IsZero() {
		inst.LastSeen = now
	}
	if inst.Status == "" {
		inst.Status = "UP"
	}
	return g.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "instance_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"service_name", "address", "port", "metadata", "status", "ttl_seconds", "last_seen", "updated_at"}),
	}).Create(inst).Error
}

func (g *gormStore) UpdateStatus(instanceID, status string) error {
	return g.db.Model(&Instance{}).Where("instance_id = ?", instanceID).
		Updates(map[string]any{"status": status, "updated_at": time.Now()}).Error
}

func (g *gormStore) Delete(instanceID string) error {
	return g.db.Where("instance_id = ?", instanceID).Delete(&Instance{}).Error
}

func (g *gormStore) FlushLastSeen(seen map[string]time.Time) error {
	if len(seen) == 0 {
		return nil
	}
	return g.db.Transaction(func(tx *gorm.DB) error {
		for id, ts := range seen {
			if err := tx.Model(&Instance{}).Where("instance_id = ?", id).
				Update("last_seen", ts).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (g *gormStore) LoadAll() ([]*Instance, error) {
	var out []*Instance
	err := g.db.Find(&out).Error
	return out, err
}
