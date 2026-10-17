package models

type PostUpdateSchedule struct {
	Account string `gorm:"primaryKey;size:64"`
	State   string `gorm:"type:text;not null"`
}
