package model

import "time"

type Notification struct {
	ID        int64      `gorm:"primaryKey" json:"id"`
	UserID    int64      `gorm:"not null" json:"user_id"`
	Kind      string     `gorm:"size:32;not null" json:"kind"`
	Payload   string     `gorm:"type:jsonb;default:'{}'::jsonb" json:"payload"`
	ReadAt    *time.Time `json:"read_at"`
	CreatedAt time.Time  `json:"created_at"`
}

func (Notification) TableName() string { return "notifications" }
