package service

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/xuri/excelize/v2"
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
	DepartmentName  string  `json:"department_name,omitempty"`
	DepartmentIDs   []int64 `json:"department_ids,omitempty"`
	DepartmentNames []string `json:"department_names,omitempty"`
}

type OperatorCtx struct {
	Role   string
	DeptID *int64
}

type ListUsersFilter struct {
	Role         string
	DepartmentID *int64
	Operator     OperatorCtx
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
	if f.Operator.Role == model.RoleDept {
		q = q.Where("role = ? AND id IN (SELECT user_id FROM user_departments WHERE department_id = ?)",
			model.RoleNormal, *f.Operator.DeptID)
	} else {
		if f.Role != "" {
			q = q.Where("role = ?", f.Role)
		}
		if f.DepartmentID != nil {
			q = q.Where("id IN (SELECT user_id FROM user_departments WHERE department_id = ?) OR department_id = ?",
				*f.DepartmentID, *f.DepartmentID)
		}
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

	// 批量加载所有部门名
	var allDepts []model.Department
	s.db.Find(&allDepts)
	deptNames := make(map[int64]string, len(allDepts))
	for _, d := range allDepts {
		deptNames[d.ID] = d.Name
	}

	// 批量加载 normal 用户的多部门关联
	userIDs := make([]int64, 0, len(users))
	for _, u := range users {
		if u.Role == model.RoleNormal {
			userIDs = append(userIDs, u.ID)
		}
	}
	userDeptMap := make(map[int64][]int64)
	if len(userIDs) > 0 {
		var uds []model.UserDepartment
		s.db.Where("user_id IN ?", userIDs).Find(&uds)
		for _, ud := range uds {
			userDeptMap[ud.UserID] = append(userDeptMap[ud.UserID], ud.DepartmentID)
		}
	}

	out := make([]UserDTO, len(users))
	for i, u := range users {
		dto := UserDTO{User: u}
		if u.Role == model.RoleNormal {
			dto.DepartmentIDs = userDeptMap[u.ID]
			if dto.DepartmentIDs == nil {
				dto.DepartmentIDs = []int64{}
			}
			names := make([]string, 0, len(dto.DepartmentIDs))
			for _, did := range dto.DepartmentIDs {
				names = append(names, deptNames[did])
			}
			dto.DepartmentNames = names
		} else if u.DepartmentID != nil {
			dto.DepartmentName = deptNames[*u.DepartmentID]
		}
		out[i] = dto
	}
	return &ListUsersResult{Items: out, Total: total, Page: f.Page, Size: f.Size}, nil
}

type CreateUserInput struct {
	Username      string  `json:"username" binding:"required,min=2,max=64"`
	Password      string  `json:"password" binding:"required,min=8,max=128"`
	Role          string  `json:"role" binding:"required,oneof=super dept normal"`
	DepartmentID  *int64  `json:"department_id"`
	DepartmentIDs []int64 `json:"department_ids"`
	RealName      string  `json:"real_name" binding:"max=64"`
}

func (s *UserService) Create(in CreateUserInput, op OperatorCtx) (*model.User, error) {
	in.Username = strings.TrimSpace(in.Username)
	if op.Role == model.RoleDept {
		if in.Role != model.RoleNormal {
			return nil, errs.New(http.StatusForbidden, "FORBIDDEN", "部门用户只能创建普通用户")
		}
		if op.DeptID == nil {
			return nil, errs.New(http.StatusForbidden, "FORBIDDEN", "部门用户缺少部门信息")
		}
		in.DepartmentIDs = []int64{*op.DeptID}
	}
	if in.Role == model.RoleDept && in.DepartmentID == nil {
		return nil, errs.New(http.StatusBadRequest, "BAD_REQUEST", "部门用户必须指定所属部门")
	}
	if in.DepartmentID != nil {
		var d model.Department
		if err := s.db.First(&d, *in.DepartmentID).Error; err != nil {
			return nil, errs.New(http.StatusBadRequest, "BAD_REQUEST", "部门不存在")
		}
	}

	var u *model.User
	err := s.db.Transaction(func(tx *gorm.DB) error {
		hash, err := auth.HashPassword(in.Password, s.cfg.Auth.BcryptCost)
		if err != nil {
			return err
		}
		user := &model.User{
			Username:     in.Username,
			PasswordHash: hash,
			Role:         in.Role,
			RealName:     in.RealName,
		}
		if in.Role == model.RoleDept {
			user.DepartmentID = in.DepartmentID
		}
		if err := tx.Create(user).Error; err != nil {
			if isUniqueViolation(err) {
				return errs.New(http.StatusConflict, "DUPLICATE", "用户名已存在")
			}
			return err
		}
		if in.Role == model.RoleNormal && len(in.DepartmentIDs) > 0 {
			uds := make([]model.UserDepartment, len(in.DepartmentIDs))
			for i, did := range in.DepartmentIDs {
				uds[i] = model.UserDepartment{UserID: user.ID, DepartmentID: did}
			}
			if err := tx.Create(&uds).Error; err != nil {
				return err
			}
		}
		u = user
		return nil
	})
	if err != nil {
		return nil, err
	}
	return u, nil
}

type UpdateUserInput struct {
	RealName      *string `json:"real_name"`
	Disabled      *bool   `json:"disabled"`
	DepartmentID  *int64  `json:"department_id"`
	DepartmentIDs *[]int64 `json:"department_ids"`
}

func (s *UserService) Update(id int64, in UpdateUserInput, op OperatorCtx) (*model.User, error) {
	var u model.User
	if err := s.db.First(&u, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrNotFound
		}
		return nil, err
	}
	if op.Role == model.RoleDept {
		if u.Role != model.RoleNormal {
			return nil, errs.ErrForbidden
		}
		var count int64
		s.db.Model(&model.UserDepartment{}).
			Where("user_id = ? AND department_id = ?", u.ID, *op.DeptID).
			Count(&count)
		if count == 0 {
			return nil, errs.ErrForbidden
		}
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		patches := map[string]any{}
		if in.RealName != nil {
			patches["real_name"] = *in.RealName
		}
		if in.Disabled != nil {
			patches["disabled"] = *in.Disabled
		}
		if u.Role == model.RoleDept && in.DepartmentID != nil {
			var d model.Department
			if err := tx.First(&d, *in.DepartmentID).Error; err != nil {
				return errs.New(http.StatusBadRequest, "BAD_REQUEST", "部门不存在")
			}
			patches["department_id"] = *in.DepartmentID
		}
		if len(patches) > 0 {
			if err := tx.Model(&u).Updates(patches).Error; err != nil {
				return err
			}
		}
		if u.Role == model.RoleNormal && in.DepartmentIDs != nil {
			tx.Where("user_id = ?", u.ID).Delete(&model.UserDepartment{})
			if len(*in.DepartmentIDs) > 0 {
				uds := make([]model.UserDepartment, len(*in.DepartmentIDs))
				for i, did := range *in.DepartmentIDs {
					uds[i] = model.UserDepartment{UserID: u.ID, DepartmentID: did}
				}
				if err := tx.Create(&uds).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.db.First(&u, id)
	return &u, nil
}

func (s *UserService) ResetPassword(id int64, newPassword string, op OperatorCtx) error {
	if len(newPassword) < 8 {
		return errs.New(http.StatusBadRequest, "BAD_REQUEST", "密码至少 8 位")
	}
	if op.Role == model.RoleDept {
		var u model.User
		if err := s.db.First(&u, id).Error; err != nil {
			return errs.ErrNotFound
		}
		if u.Role != model.RoleNormal {
			return errs.ErrForbidden
		}
		var count int64
		s.db.Model(&model.UserDepartment{}).
			Where("user_id = ? AND department_id = ?", u.ID, *op.DeptID).
			Count(&count)
		if count == 0 {
			return errs.ErrForbidden
		}
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

// ExportUsers 导出用户列表为 Excel
func (s *UserService) ExportUsers(op OperatorCtx) (*excelize.File, error) {
	q := s.db.Model(&model.User{}).Where("disabled = false")
	if op.Role == model.RoleDept && op.DeptID != nil {
		q = q.Where("role = ? AND id IN (SELECT user_id FROM user_departments WHERE department_id = ?)",
			model.RoleNormal, *op.DeptID)
	}
	var users []model.User
	if err := q.Order("id ASC").Find(&users).Error; err != nil {
		return nil, err
	}

	var allDepts []model.Department
	s.db.Find(&allDepts)
	deptNames := make(map[int64]string, len(allDepts))
	for _, d := range allDepts {
		deptNames[d.ID] = d.Name
	}

	userIDs := make([]int64, 0, len(users))
	for _, u := range users {
		userIDs = append(userIDs, u.ID)
	}
	userDeptMap := make(map[int64][]int64)
	if len(userIDs) > 0 {
		var uds []model.UserDepartment
		s.db.Where("user_id IN ?", userIDs).Find(&uds)
		for _, ud := range uds {
			userDeptMap[ud.UserID] = append(userDeptMap[ud.UserID], ud.DepartmentID)
		}
	}

	f := excelize.NewFile()
	sheet := "用户列表"
	f.SetSheetName("Sheet1", sheet)
	headers := []string{"用户名", "姓名", "所属部门", "角色"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}

	roleLabel := map[string]string{model.RoleSuper: "顶级用户", model.RoleDept: "部门用户", model.RoleNormal: "普通用户"}
	for row, u := range users {
		r := row + 2
		f.SetCellValue(sheet, cellName(1, r), u.Username)
		f.SetCellValue(sheet, cellName(2, r), u.RealName)
		var deptStr string
		if u.Role == model.RoleNormal {
			names := make([]string, 0)
			for _, did := range userDeptMap[u.ID] {
				names = append(names, deptNames[did])
			}
			deptStr = strings.Join(names, ",")
		} else if u.DepartmentID != nil {
			deptStr = deptNames[*u.DepartmentID]
		}
		f.SetCellValue(sheet, cellName(3, r), deptStr)
		f.SetCellValue(sheet, cellName(4, r), roleLabel[u.Role])
	}

	f.SetColWidth(sheet, "A", "A", 20)
	f.SetColWidth(sheet, "B", "B", 15)
	f.SetColWidth(sheet, "C", "C", 30)
	f.SetColWidth(sheet, "D", "D", 12)
	return f, nil
}

// ExportTemplate 导出空白导入模板
func (s *UserService) ExportTemplate() (*excelize.File, error) {
	f := excelize.NewFile()
	sheet := "导入模板"
	f.SetSheetName("Sheet1", sheet)
	headers := []string{"用户名", "姓名", "所属部门", "角色"}
	for i, h := range headers {
		f.SetCellValue(sheet, cellName(i+1, 1), h)
	}
	f.SetCellValue(sheet, cellName(1, 2), "zhangsan")
	f.SetCellValue(sheet, cellName(2, 2), "张三")
	f.SetCellValue(sheet, cellName(3, 2), "技术部,财务部")
	f.SetCellValue(sheet, cellName(4, 2), "普通用户")

	f.SetColWidth(sheet, "A", "A", 20)
	f.SetColWidth(sheet, "B", "B", 15)
	f.SetColWidth(sheet, "C", "C", 30)
	f.SetColWidth(sheet, "D", "D", 12)
	return f, nil
}

type ImportResult struct {
	Total   int      `json:"total"`
	Success int      `json:"success"`
	Errors  []string `json:"errors"`
}

// ImportUsers 从 Excel 导入用户
func (s *UserService) ImportUsers(reader io.Reader, op OperatorCtx, defaultPassword string) (*ImportResult, error) {
	f, err := excelize.OpenReader(reader)
	if err != nil {
		return nil, errs.New(http.StatusBadRequest, "BAD_FILE", "无法解析 Excel 文件")
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, errs.New(http.StatusBadRequest, "BAD_FILE", "Excel 文件无工作表")
	}
	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return nil, errs.New(http.StatusBadRequest, "BAD_FILE", "读取工作表失败")
	}
	if len(rows) < 2 {
		return nil, errs.New(http.StatusBadRequest, "BAD_FILE", "Excel 无数据行")
	}

	var allDepts []model.Department
	s.db.Where("disabled = false").Find(&allDepts)
	deptByName := make(map[string]int64, len(allDepts))
	for _, d := range allDepts {
		deptByName[d.Name] = d.ID
	}

	roleLabelToCode := map[string]string{"普通用户": model.RoleNormal, "部门用户": model.RoleDept, "顶级用户": model.RoleSuper}

	if defaultPassword == "" {
		defaultPassword = "init1234"
	}

	result := &ImportResult{}
	for i, row := range rows[1:] {
		lineNum := i + 2
		result.Total++
		if len(row) < 2 {
			result.Errors = append(result.Errors, fmt.Sprintf("第%d行: 列数不足", lineNum))
			continue
		}
		username := strings.TrimSpace(row[0])
		realName := strings.TrimSpace(row[1])
		if username == "" {
			result.Errors = append(result.Errors, fmt.Sprintf("第%d行: 用户名为空", lineNum))
			continue
		}

		var deptStr string
		if len(row) >= 3 {
			deptStr = strings.TrimSpace(row[2])
		}
		role := model.RoleNormal
		if len(row) >= 4 {
			r := strings.TrimSpace(row[3])
			if code, ok := roleLabelToCode[r]; ok {
				role = code
			}
		}

		if op.Role == model.RoleDept && role != model.RoleNormal {
			result.Errors = append(result.Errors, fmt.Sprintf("第%d行: 部门用户只能导入普通用户", lineNum))
			continue
		}

		var deptIDs []int64
		if deptStr != "" {
			parts := strings.Split(deptStr, ",")
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p == "" {
					continue
				}
				if did, ok := deptByName[p]; ok {
					deptIDs = append(deptIDs, did)
				} else {
					result.Errors = append(result.Errors, fmt.Sprintf("第%d行: 部门\"%s\"不存在", lineNum, p))
				}
			}
		}

		if op.Role == model.RoleDept && op.DeptID != nil {
			found := false
			for _, did := range deptIDs {
				if did == *op.DeptID {
					found = true
					break
				}
			}
			if !found {
				deptIDs = append(deptIDs, *op.DeptID)
			}
		}

		hash, err := auth.HashPassword(defaultPassword, s.cfg.Auth.BcryptCost)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("第%d行: 密码加密失败", lineNum))
			continue
		}

		txErr := s.db.Transaction(func(tx *gorm.DB) error {
			u := &model.User{
				Username:     username,
				PasswordHash: hash,
				Role:         role,
				RealName:     realName,
			}
			if role == model.RoleDept && len(deptIDs) > 0 {
				u.DepartmentID = &deptIDs[0]
			}
			if err := tx.Create(u).Error; err != nil {
				if isUniqueViolation(err) {
					return fmt.Errorf("用户名\"%s\"已存在", username)
				}
				return err
			}
			if role == model.RoleNormal && len(deptIDs) > 0 {
				uds := make([]model.UserDepartment, len(deptIDs))
				for j, did := range deptIDs {
					uds[j] = model.UserDepartment{UserID: u.ID, DepartmentID: did}
				}
				if err := tx.Create(&uds).Error; err != nil {
					return err
				}
			}
			return nil
		})
		if txErr != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("第%d行: %s", lineNum, txErr.Error()))
			continue
		}
		result.Success++
	}
	return result, nil
}

func cellName(col, row int) string {
	name, _ := excelize.CoordinatesToCellName(col, row)
	return name
}
