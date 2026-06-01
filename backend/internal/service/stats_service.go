package service

import (
	"strings"
	"time"

	"github.com/xuri/excelize/v2"

	"gorm.io/gorm"

	errs "github.com/ksm/docflow/internal/errors"
	"github.com/ksm/docflow/internal/model"
)

type StatsService struct {
	db *gorm.DB
}

func NewStatsService(db *gorm.DB) *StatsService {
	return &StatsService{db: db}
}

type UserSubmissionRow struct {
	UserID         int64      `json:"user_id"`
	Username       string     `json:"username"`
	RealName       string     `json:"real_name"`
	DepartmentID   *int64     `json:"department_id"`
	DepartmentName string     `json:"department_name"`
	SubmissionID   int64      `json:"submission_id"`
	CurrentStatus  string     `json:"current_status"`
	DisplayStatus  string     `json:"display_status"`
	SubmittedAt    *time.Time `json:"submitted_at"`
	ReturnCount    int        `json:"return_count"`
	ReturnReason   string     `json:"return_reason"`
}

type DeptSummaryRow struct {
	DepartmentID   int64  `json:"department_id"`
	DepartmentName string `json:"department_name"`
	Total          int64  `json:"total"`
	Submitted      int64  `json:"submitted"`
	Late           int64  `json:"late"`
	Pending        int64  `json:"pending"`
	Overdue        int64  `json:"overdue"`
	Returned       int64  `json:"returned"`
}

// DocumentOverview 单公文纵览:汇总 + 分页用户明细
type DocumentOverview struct {
	Document     model.Document      `json:"document"`
	Summary      DocumentSummary     `json:"summary"`
	ByUser       []UserSubmissionRow `json:"by_user"`
	ByDepartment []DeptSummaryRow    `json:"by_department,omitempty"`
	Total        int64               `json:"total"`
	Page         int                 `json:"page"`
	Size         int                 `json:"size"`
}

type DocOverviewFilter struct {
	DocID  int64
	Status string
	Page   int
	Size   int
}

func (s *StatsService) DocumentOverview(f DocOverviewFilter) (*DocumentOverview, error) {
	if f.Page <= 0 {
		f.Page = 1
	}
	if f.Size <= 0 || f.Size > 200 {
		f.Size = 20
	}

	var doc model.Document
	if err := s.db.First(&doc, f.DocID).Error; err != nil {
		return nil, errs.ErrNotFound
	}

	// 汇总（全量，不受分页/筛选影响）
	var allSubs []model.Submission
	s.db.Where("document_id = ?", f.DocID).Find(&allSubs)

	now := time.Now()
	overview := &DocumentOverview{Document: doc, Page: f.Page, Size: f.Size}
	deptAgg := make(map[int64]*DeptSummaryRow)

	// 加载部门名
	var depts []model.Department
	s.db.Find(&depts)
	dMap := make(map[int64]model.Department, len(depts))
	for _, d := range depts {
		dMap[d.ID] = d
	}

	for _, sub := range allSubs {
		disp := displayStatus(&sub, &doc)
		_ = now
		overview.Summary.Total++
		switch disp {
		case model.SubStatusSubmitted:
			overview.Summary.Submitted++
		case model.SubStatusSubmittedLate:
			overview.Summary.Late++
		case model.SubStatusReturned:
			overview.Summary.Returned++
		case model.SubStatusOverdue:
			overview.Summary.Overdue++
		default:
			overview.Summary.Pending++
		}
		if sub.DepartmentID != nil {
			r, ok := deptAgg[*sub.DepartmentID]
			if !ok {
				r = &DeptSummaryRow{DepartmentID: *sub.DepartmentID, DepartmentName: dMap[*sub.DepartmentID].Name}
				deptAgg[*sub.DepartmentID] = r
			}
			r.Total++
			switch disp {
			case model.SubStatusSubmitted:
				r.Submitted++
			case model.SubStatusSubmittedLate:
				r.Late++
			case model.SubStatusReturned:
				r.Returned++
			case model.SubStatusOverdue:
				r.Overdue++
			default:
				r.Pending++
			}
		}
	}
	if doc.TargetScope == model.ScopeDepartment {
		for _, r := range deptAgg {
			overview.ByDepartment = append(overview.ByDepartment, *r)
		}
	}

	// 分页查询用户明细
	q := s.db.Model(&model.Submission{}).Where("document_id = ?", f.DocID)
	if f.Status != "" {
		if f.Status == model.SubStatusOverdue {
			q = q.Where("current_status = ? AND (SELECT deadline FROM documents WHERE id = ?) IS NOT NULL AND (SELECT deadline FROM documents WHERE id = ?) < NOW()",
				model.SubStatusPending, f.DocID, f.DocID)
		} else {
			q = q.Where("current_status = ?", f.Status)
		}
	}
	var totalFiltered int64
	q.Count(&totalFiltered)
	overview.Total = totalFiltered

	var subs []model.Submission
	q.Order("id ASC").Limit(f.Size).Offset((f.Page - 1) * f.Size).Find(&subs)

	// 关联用户
	uids := make([]int64, len(subs))
	for i, sub := range subs {
		uids[i] = sub.UserID
	}
	var users []model.User
	if len(uids) > 0 {
		s.db.Where("id IN ?", uids).Find(&users)
	}
	uMap := make(map[int64]model.User, len(users))
	for _, u := range users {
		uMap[u.ID] = u
	}

	for _, sub := range subs {
		disp := displayStatus(&sub, &doc)
		u := uMap[sub.UserID]
		var deptName string
		var deptID *int64
		if sub.DepartmentID != nil {
			deptID = sub.DepartmentID
			deptName = dMap[*sub.DepartmentID].Name
		}
		overview.ByUser = append(overview.ByUser, UserSubmissionRow{
			UserID:         u.ID,
			Username:       u.Username,
			RealName:       u.RealName,
			DepartmentID:   deptID,
			DepartmentName: deptName,
			SubmissionID:   sub.ID,
			CurrentStatus:  sub.CurrentStatus,
			DisplayStatus:  disp,
			SubmittedAt:    sub.SubmittedAt,
			ReturnCount:    sub.ReturnCount,
			ReturnReason:   sub.ReturnReason,
		})
	}
	if overview.ByUser == nil {
		overview.ByUser = []UserSubmissionRow{}
	}
	return overview, nil
}

// ExportDocumentOverview 导出单公文纵览为 Excel
func (s *StatsService) ExportDocumentOverview(docID int64) (*excelize.File, error) {
	var doc model.Document
	if err := s.db.First(&doc, docID).Error; err != nil {
		return nil, errs.ErrNotFound
	}
	var subs []model.Submission
	s.db.Where("document_id = ?", docID).Order("id ASC").Find(&subs)

	uids := make([]int64, len(subs))
	for i, sub := range subs {
		uids[i] = sub.UserID
	}
	var users []model.User
	if len(uids) > 0 {
		s.db.Where("id IN ?", uids).Find(&users)
	}
	uMap := make(map[int64]model.User, len(users))
	for _, u := range users {
		uMap[u.ID] = u
	}
	var depts []model.Department
	s.db.Find(&depts)
	dMap := make(map[int64]string, len(depts))
	for _, d := range depts {
		dMap[d.ID] = d.Name
	}

	statusLabel := map[string]string{
		model.SubStatusPending:       "待上报",
		model.SubStatusSubmitted:     "准时已交",
		model.SubStatusSubmittedLate: "逾期已交",
		model.SubStatusReturned:      "已退回",
		model.SubStatusOverdue:       "逾期未交",
	}

	f := excelize.NewFile()
	sheet := doc.Title
	if len([]rune(sheet)) > 31 {
		sheet = string([]rune(sheet)[:28]) + "..."
	}
	sheet = strings.ReplaceAll(sheet, "/", "-")
	f.SetSheetName("Sheet1", sheet)

	headers := []string{"姓名", "用户名", "部门", "状态", "提交时间", "退回次数", "退回原因"}
	for i, h := range headers {
		f.SetCellValue(sheet, cellName(i+1, 1), h)
	}

	for row, sub := range subs {
		r := row + 2
		u := uMap[sub.UserID]
		disp := displayStatus(&sub, &doc)
		var deptName string
		if sub.DepartmentID != nil {
			deptName = dMap[*sub.DepartmentID]
		}
		var submitTime string
		if sub.SubmittedAt != nil {
			submitTime = sub.SubmittedAt.Format("2006-01-02 15:04")
		}
		f.SetCellValue(sheet, cellName(1, r), u.RealName)
		f.SetCellValue(sheet, cellName(2, r), u.Username)
		f.SetCellValue(sheet, cellName(3, r), deptName)
		f.SetCellValue(sheet, cellName(4, r), statusLabel[disp])
		f.SetCellValue(sheet, cellName(5, r), submitTime)
		f.SetCellValue(sheet, cellName(6, r), sub.ReturnCount)
		f.SetCellValue(sheet, cellName(7, r), sub.ReturnReason)
	}

	f.SetColWidth(sheet, "A", "A", 12)
	f.SetColWidth(sheet, "B", "B", 18)
	f.SetColWidth(sheet, "C", "C", 15)
	f.SetColWidth(sheet, "D", "D", 12)
	f.SetColWidth(sheet, "E", "E", 18)
	f.SetColWidth(sheet, "F", "F", 10)
	f.SetColWidth(sheet, "G", "G", 30)
	return f, nil
}

// GlobalOverview super 全局纵览：按部门聚合所有活跃公文的上报统计
type GlobalOverviewResult struct {
	TotalDocuments  int64            `json:"total_documents"`
	ActiveDocuments int64            `json:"active_documents"`
	ByDepartment    []DeptSummaryRow `json:"by_department"`
}

func (s *StatsService) GlobalOverview() (*GlobalOverviewResult, error) {
	result := &GlobalOverviewResult{}
	s.db.Model(&model.Document{}).Count(&result.TotalDocuments)
	s.db.Model(&model.Document{}).Where("status = ?", model.DocumentStatusActive).Count(&result.ActiveDocuments)

	type row struct {
		DepartmentID *int64
		Status       string
		Cnt          int64
	}
	var rows []row
	if err := s.db.Model(&model.Submission{}).
		Select("department_id, current_status AS status, COUNT(*) AS cnt").
		Group("department_id, current_status").Scan(&rows).Error; err != nil {
		return nil, err
	}

	var depts []model.Department
	s.db.Find(&depts)
	dMap := make(map[int64]string, len(depts))
	for _, d := range depts {
		dMap[d.ID] = d.Name
	}

	// 取所有活跃公文 deadline
	var docs []model.Document
	s.db.Select("id, deadline").Where("status = ?", model.DocumentStatusActive).Find(&docs)
	// 为简化，全局纵览不区分逾期（按 submission 表状态聚合即可）
	// 但 PENDING 且过期的需要算 OVERDUE，这里无法精确做到(需要 join)，使用近似统计
	// 更好的方案：直接按部门聚合 submissions JOIN documents
	type subRow struct {
		DepartmentID *int64
		SubStatus    string
		Overdue      bool
		Cnt          int64
	}
	var subRows []subRow
	s.db.Raw(`
		SELECT s.department_id, s.current_status AS sub_status,
			CASE WHEN s.current_status = 'PENDING' AND d.deadline IS NOT NULL AND d.deadline < NOW() THEN true ELSE false END AS overdue,
			COUNT(*) AS cnt
		FROM submissions s JOIN documents d ON s.document_id = d.id
		WHERE d.status = 'ACTIVE'
		GROUP BY s.department_id, s.current_status, overdue
	`).Scan(&subRows)

	agg := make(map[int64]*DeptSummaryRow)
	for _, r := range subRows {
		var deptID int64
		if r.DepartmentID != nil {
			deptID = *r.DepartmentID
		}
		dr, ok := agg[deptID]
		if !ok {
			dr = &DeptSummaryRow{DepartmentID: deptID, DepartmentName: dMap[deptID]}
			if deptID == 0 {
				dr.DepartmentName = "未分配部门"
			}
			agg[deptID] = dr
		}
		dr.Total += r.Cnt
		switch r.SubStatus {
		case model.SubStatusSubmitted:
			dr.Submitted += r.Cnt
		case model.SubStatusSubmittedLate:
			dr.Late += r.Cnt
		case model.SubStatusReturned:
			dr.Returned += r.Cnt
		case model.SubStatusPending:
			if r.Overdue {
				dr.Overdue += r.Cnt
			} else {
				dr.Pending += r.Cnt
			}
		}
	}
	for _, v := range agg {
		result.ByDepartment = append(result.ByDepartment, *v)
	}
	if result.ByDepartment == nil {
		result.ByDepartment = []DeptSummaryRow{}
	}
	return result, nil
}

// DepartmentDocOverview 部门纵览：某部门相关的所有公文统计
type DeptDocRow struct {
	DocumentID int64  `json:"document_id"`
	Title      string `json:"title"`
	Deadline   *time.Time `json:"deadline"`
	Total      int64  `json:"total"`
	Submitted  int64  `json:"submitted"`
	Late       int64  `json:"late"`
	Pending    int64  `json:"pending"`
	Overdue    int64  `json:"overdue"`
	Returned   int64  `json:"returned"`
}

type DepartmentOverviewResult struct {
	Department model.Department `json:"department"`
	Documents  []DeptDocRow     `json:"documents"`
}

func (s *StatsService) DepartmentOverview(deptID int64) (*DepartmentOverviewResult, error) {
	var dept model.Department
	if err := s.db.First(&dept, deptID).Error; err != nil {
		return nil, errs.ErrNotFound
	}

	type row struct {
		DocumentID int64
		Title      string
		Deadline   *time.Time
		SubStatus  string
		IsOverdue  bool
		Cnt        int64
	}
	var rows []row
	s.db.Raw(`
		SELECT s.document_id, d.title, d.deadline,
			s.current_status AS sub_status,
			CASE WHEN s.current_status = 'PENDING' AND d.deadline IS NOT NULL AND d.deadline < NOW() THEN true ELSE false END AS is_overdue,
			COUNT(*) AS cnt
		FROM submissions s JOIN documents d ON s.document_id = d.id
		WHERE s.department_id = ? AND d.status = 'ACTIVE'
		GROUP BY s.document_id, d.title, d.deadline, s.current_status, is_overdue
		ORDER BY s.document_id DESC
	`, deptID).Scan(&rows)

	docMap := make(map[int64]*DeptDocRow)
	var order []int64
	for _, r := range rows {
		dr, ok := docMap[r.DocumentID]
		if !ok {
			dr = &DeptDocRow{DocumentID: r.DocumentID, Title: r.Title, Deadline: r.Deadline}
			docMap[r.DocumentID] = dr
			order = append(order, r.DocumentID)
		}
		dr.Total += r.Cnt
		switch r.SubStatus {
		case model.SubStatusSubmitted:
			dr.Submitted += r.Cnt
		case model.SubStatusSubmittedLate:
			dr.Late += r.Cnt
		case model.SubStatusReturned:
			dr.Returned += r.Cnt
		case model.SubStatusPending:
			if r.IsOverdue {
				dr.Overdue += r.Cnt
			} else {
				dr.Pending += r.Cnt
			}
		}
	}
	docs := make([]DeptDocRow, 0, len(order))
	for _, id := range order {
		docs = append(docs, *docMap[id])
	}
	return &DepartmentOverviewResult{Department: dept, Documents: docs}, nil
}
