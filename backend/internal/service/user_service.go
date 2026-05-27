package service

import (
	"errors"
	"net/http"
	"strings"

	"gorm.io/gorm"

	"github.com/ksm/docflow/internal/auth"
	"github.com/ksm/docflow/internal/config"
	errs "github.com/ksm/docflow/internal/errors"
	"github.com/ksm/docflow/internal/model"
)

type UserService struct {
	db  *gorm.DB
	cfg *config.Config
}

func NewUserService(db *gorm.DB, cfg *config.Config) *UserService {
	return &UserService{db: db, cfg: cfg}
}

type UserDTO struct {
	model.User
	DepartmentName string `json:"department_name,omitempty"`
}

type ListUsersFilter struct {
	Role         string
	DepartmentID *int64
	Page         int
	Size         int
}

type ListUsersResult struct {
	Items []UserDTO `json:"items"`
	Total int64     `json:"total"`
	Page  int       `json:"page"`
	Size  int       `json:"size"`
}

func (s *UserService) List(f ListUsersFilter) (*ListUsersResult, error) {
	if f.Page <= 0 {
		f.Page = 1
	}
	if f.Size <= 0 || f.Size > 100 {
		f.Size = 20
	}
	q := s.db.Model(&model.User{})
	if f.Role != "" {
		q = q.Where("role = ?", f.Role)
	}
	if f.DepartmentID != nil {
		q = q.Where("department_id = ?", *f.DepartmentID)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}
	var users []model.User
	if err := q.Order("id ASC").
		Limit(f.Size).Offset((f.Page - 1) * f.Size).
		Find(&users).Error; err != nil {
		return nil, err
	}
	// 关联部门名
	deptIDs := make([]int64, 0, len(users))
	for _, u := range users {
		if u.DepartmentID != nil {
			deptIDs = append(deptIDs, *u.DepartmentID)
		}
	}
	deptNames := make(map[int64]string)
	if len(deptIDs) > 0 {
		var ds []model.Department
		if err := s.db.Where("id IN ?", deptIDs).Find(&ds).Error; err != nil {
			return nil, err
		}
		for _, d := range ds {
			deptNames[d.ID] = d.Name
		}
	}
	out := make([]UserDTO, len(users))
	for i, u := range users {
		dto := UserDTO{User: u}
		if u.DepartmentID != nil {
			dto.DepartmentName = deptNames[*u.DepartmentID]
		}
		out[i] = dto
	}
	return &ListUsersResult{Items: out, Total: total, Page: f.Page, Size: f.Size}, nil
}

type CreateUserInput struct {
	Username     string `json:"username" binding:"required,min=2,max=64"`
	Password     string `json:"password" binding:"required,min=8,max=128"`
	Role         string `json:"role" binding:"required,oneof=super dept user"`
	DepartmentID *int64 `json:"department_id"`
	RealName     string `json:"real_name" binding:"max=64"`
}

func (s *UserService) Create(in CreateUserInput) (*model.User, error) {
	in.Username = strings.TrimSpace(in.Username)
	if in.Role != model.RoleSuper && in.DepartmentID == nil {
		return nil, errs.New(http.StatusBadRequest, "BAD_REQUEST", "非顶级用户必须指定所属部门")
	}
	if in.DepartmentID != nil {
		var d model.Department
		if err := s.db.First(&d, *in.DepartmentID).Error; err != nil {
			return nil, errs.New(http.StatusBadRequest, "BAD_REQUEST", "部门不存在")
		}
	}
	hash, err := auth.HashPassword(in.Password, s.cfg.Auth.BcryptCost)
	if err != nil {
		return nil, err
	}
	u := &model.User{
		Username:     in.Username,
		PasswordHash: hash,
		Role:         in.Role,
		DepartmentID: in.DepartmentID,
		RealName:     in.RealName,
	}
	if err := s.db.Create(u).Error; err != nil {
		if isUniqueViolation(err) {
			return nil, errs.New(http.StatusConflict, "DUPLICATE", "用户名已存在")
		}
		return nil, err
	}
	return u, nil
}

type UpdateUserInput struct {
	RealName     *string `json:"real_name"`
	Disabled     *bool   `json:"disabled"`
	DepartmentID *int64  `json:"department_id"`
}

func (s *UserService) Update(id int64, in UpdateUserInput) (*model.User, error) {
	var u model.User
	if err := s.db.First(&u, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrNotFound
		}
		return nil, err
	}
	patches := map[string]any{}
	if in.RealName != nil {
		patches["real_name"] = *in.RealName
	}
	if in.Disabled != nil {
		patches["disabled"] = *in.Disabled
	}
	if in.DepartmentID != nil {
		var d model.Department
		if err := s.db.First(&d, *in.DepartmentID).Error; err != nil {
			return nil, errs.New(http.StatusBadRequest, "BAD_REQUEST", "部门不存在")
		}
		patches["department_id"] = *in.DepartmentID
	}
	if len(patches) == 0 {
		return &u, nil
	}
	if err := s.db.Model(&u).Updates(patches).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *UserService) ResetPassword(id int64, newPassword string) error {
	if len(newPassword) < 8 {
		return errs.New(http.StatusBadRequest, "BAD_REQUEST", "密码至少 8 位")
	}
	hash, err := auth.HashPassword(newPassword, s.cfg.Auth.BcryptCost)
	if err != nil {
		return err
	}
	res := s.db.Model(&model.User{}).Where("id = ?", id).
		Update("password_hash", hash)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errs.ErrNotFound
	}
	return nil
}
