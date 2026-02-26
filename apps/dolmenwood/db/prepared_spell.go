package db

type PreparedSpell struct {
	ID          uint   `gorm:"primarykey"`
	CharacterID uint   `gorm:"column:character_id;index"`
	Name        string `gorm:"column:name"`
	SpellLevel  int    `gorm:"column:spell_level"`
	Used        bool   `gorm:"column:used"`
}
