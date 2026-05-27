package service

import (
	"time"

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

// DocumentOverview 单公文纵览:含总数 + 按用户 + 按部门
type DocumentOverview struct {
	Document     model.Document      `json:"document"`
	Summary      DocumentSummary     `json:"summary"`
	ByUser       []UserSubmissionRow `json:"by_user"`
	ByDepartment []DeptSummaryRow    `json:"by_department,omitempty"`
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

func (s *StatsService) DocumentOverview(docID int64) (*DocumentOverview, error) {
	var doc model.Document
	if err := s.db.First(&doc, docID).Error; err != nil {
		return nil, errs.ErrNotFound
	}

	type row struct {
		Sub      model.Submission `gorm:"embedded"`
		Username string
		RealName string
	}
	var subs []model.Submission
	if err := s.db.Where("document_id = ?", docID).Find(&subs).Error; err != nil {
		return nil, err
	}

	// 关联用户
	uids := make([]int64, len(subs))
	for i, s := range subs {
		uids[i] = s.UserID
	}
	var users []model.User
	if len(uids) > 0 {
		_ = s.db.Where("id IN ?", uids).Find(&users).Error
	}
	uMap := make(map[int64]model.User, len(users))
	for _, u := range users {
		uMap[u.ID] = u
	}
	// 关联部门名
	deptIDs := make([]int64, 0, len(subs))
	for _, sub := range subs {
		if sub.DepartmentID != nil {
			deptIDs = append(deptIDs, *sub.DepartmentID)
		}
	}
	var depts []model.Department
	if len(deptIDs) > 0 {
		_ = s.db.Where("id IN ?", deptIDs).Find(&depts).Error
	}
	dMap := make(map[int64]model.Department, len(depts))
	for _, d := range depts {
		dMap[d.ID] = d
	}

	now := time.Now()
	overview := &DocumentOverview{Document: doc}
	deptAgg := make(map[int64]*DeptSummaryRow)

	for _, sub := range subs {
		disp := displayStatus(&sub, &doc)
		_ = now // 保留供后续扩展
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

		// 汇总
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

		// 按部门
		if deptID != nil {
			r, ok := deptAgg[*deptID]
			if !ok {
				r = &DeptSummaryRow{DepartmentID: *deptID, DepartmentName: deptName}
				deptAgg[*deptID] = r
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
	if overview.ByUser == nil {
		overview.ByUser = []UserSubmissionRow{}
	}
	return overview, nil
}
