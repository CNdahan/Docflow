package model

import "time"

const (
	RoleSuper  = "super"
	RoleDept   = "dept"
	RoleNormal = "normal"
)

type Department struct {
	ID        int64     `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:64;uniqueIndex;not null" json:"name"`
	Disabled  bool      `gorm:"not null;default:false" json:"disabled"`
	CreatedAt time.Time `json:"created_at"`
}

func (Department) TableName() string { return "departments" }

type User struct {
	ID           int64     `gorm:"primaryKey" json:"id"`
	Username     string    `gorm:"size:64;uniqueIndex;not null" json:"username"`
	PasswordHash string    `gorm:"size:255;not null" json:"-"`
	Role         string    `gorm:"size:16;not null" json:"role"`
	DepartmentID *int64    `json:"department_id"`
	RealName     string    `gorm:"size:64" json:"real_name"`
	Disabled     bool      `gorm:"not null;default:false" json:"disabled"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (User) TableName() string { return "users" }

type UserDepartment struct {
	UserID       int64 `gorm:"primaryKey" json:"user_id"`
	DepartmentID int64 `gorm:"primaryKey" json:"department_id"`
}

func (UserDepartment) TableName() string { return "user_departments" }
