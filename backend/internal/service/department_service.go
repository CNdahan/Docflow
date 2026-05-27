package service

import (
	"errors"
	"net/http"
	"strings"

	"gorm.io/gorm"

	errs "github.com/ksm/docflow/internal/errors"
	"github.com/ksm/docflow/internal/model"
)

type DepartmentService struct {
	db *gorm.DB
}

func NewDepartmentService(db *gorm.DB) *DepartmentService {
	return &DepartmentService{db: db}
}

type DepartmentDTO struct {
	model.Department
	UserCount int64 `json:"user_count"`
}

func (s *DepartmentService) List() ([]DepartmentDTO, error) {
	var depts []model.Department
	if err := s.db.Order("id ASC").Find(&depts).Error; err != nil {
		return nil, err
	}
	if len(depts) == 0 {
		return []DepartmentDTO{}, nil
	}
	ids := make([]int64, len(depts))
	for i, d := range depts {
		ids[i] = d.ID
	}
	type row struct {
		DepartmentID int64
		Cnt          int64
	}
	var rows []row
	if err := s.db.Model(&model.User{}).
		Select("department_id, COUNT(*) AS cnt").
		Where("department_id IN ? AND disabled = false", ids).
		Group("department_id").Scan(&rows).Error; err != nil {
		return nil, err
	}
	cntMap := make(map[int64]int64, len(rows))
	for _, r := range rows {
		cntMap[r.DepartmentID] = r.Cnt
	}
	out := make([]DepartmentDTO, len(depts))
	for i, d := range depts {
		out[i] = DepartmentDTO{Department: d, UserCount: cntMap[d.ID]}
	}
	return out, nil
}

func (s *DepartmentService) Create(name string) (*model.Department, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errs.New(http.StatusBadRequest, "BAD_REQUEST", "部门名称不能为空")
	}
	d := &model.Department{Name: name}
	if err := s.db.Create(d).Error; err != nil {
		if isUniqueViolation(err) {
			return nil, errs.New(http.StatusConflict, "DUPLICATE", "部门名称已存在")
		}
		return nil, err
	}
	return d, nil
}

type DepartmentUpdate struct {
	Name     *string `json:"name"`
	Disabled *bool   `json:"disabled"`
}

func (s *DepartmentService) Update(id int64, in DepartmentUpdate) (*model.Department, error) {
	var d model.Department
	if err := s.db.First(&d, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrNotFound
		}
		return nil, err
	}
	patches := map[string]any{}
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" {
			return nil, errs.New(http.StatusBadRequest, "BAD_REQUEST", "部门名称不能为空")
		}
		patches["name"] = name
	}
	if in.Disabled != nil {
		patches["disabled"] = *in.Disabled
	}
	if len(patches) == 0 {
		return &d, nil
	}
	if err := s.db.Model(&d).Updates(patches).Error; err != nil {
		if isUniqueViolation(err) {
			return nil, errs.New(http.StatusConflict, "DUPLICATE", "部门名称已存在")
		}
		return nil, err
	}
	return &d, nil
}

// isUniqueViolation 判断是否唯一键冲突 (PostgreSQL 23505)
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "23505") ||
		strings.Contains(err.Error(), "duplicate key")
}
