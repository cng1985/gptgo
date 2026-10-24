package account

import (
	"time"

	"gorm.io/gorm"
)

func Ensure(db *gorm.DB) error {
	var count int64
	if err := db.Model(&Account{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	now := time.Now().UTC()
	return db.Select("Name", "CreatedAt", "UpdatedAt").Create(&Account{
		Name:      "ada",
		CreatedAt: now,
		UpdatedAt: now,
	}).Error
}
