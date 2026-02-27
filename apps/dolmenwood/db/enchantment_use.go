package db

import "time"

type EnchantmentUse struct {
	ID          uint      `gorm:"primarykey"`
	CharacterID uint      `gorm:"column:character_id;index"`
	Used        bool      `gorm:"column:used"`
	CreatedAt   time.Time `gorm:"column:created_at"`
}
