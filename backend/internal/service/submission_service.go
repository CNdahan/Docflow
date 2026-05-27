package service

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/ksm/docflow/internal/config"
	errs "github.com/ksm/docflow/internal/errors"
	"github.com/ksm/docflow/internal/model"
	"github.com/ksm/docflow/internal/storage"
)

type SubmissionService struct {
	db      *gorm.DB
	cfg     *config.Config
	storage storage.Storage
}

func NewSubmissionService(db *gorm.DB, cfg *config.Config, s storage.Storage) *SubmissionService {
	return &SubmissionService{db: db, cfg: cfg, storage: s}
}

type SubmissionDTO struct {
	model.Submission
	DisplayStatus string             `json:"display_status"` // 含虚拟 OVERDUE
	Attachments   []model.Attachment `json:"attachments"`
}

func buildSubmissionDTO(db *gorm.DB, doc *model.Document, sub *model.Submission) SubmissionDTO {
	dto := SubmissionDTO{Submission: *sub}
	dto.DisplayStatus = displayStatus(sub, doc)
	var atts []model.Attachment
	_ = db.Where("owner_type = ? AND owner_id = ?", model.OwnerSubmission, sub.ID).
		Order("id ASC").Find(&atts).Error
	if atts == nil {
		atts = []model.Attachment{}
	}
	dto.Attachments = atts
	return dto
}

func displayStatus(sub *model.Submission, doc *model.Document) string {
	if sub.CurrentStatus == model.SubStatusPending && doc.Deadline != nil && doc.Deadline.Before(time.Now()) {
		return model.SubStatusOverdue
	}
	return sub.CurrentStatus
}

// Submit 提交或重提:由当前状态决定
type SubmitInput struct {
	DocumentID    int64
	UserID        int64
	Note          string
	AttachmentIDs []int64 // 必须是当前用户已上传到 SUBMISSION/uploader 的 attachment
}

func (s *SubmissionService) Submit(in SubmitInput) (*model.Submission, error) {
	if len(in.AttachmentIDs) == 0 {
		return nil, errs.New(http.StatusBadRequest, "BAD_REQUEST", "至少需要一个上报附件")
	}
	if len(in.AttachmentIDs) > s.cfg.Storage.MaxAttachmentsPerSubmission {
		return nil, errs.New(http.StatusBadRequest, "TOO_MANY_FILES", "上报附件数量超出限制")
	}

	var subResult *model.Submission
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// 锁住对应 submission 行 (PostgreSQL FOR UPDATE)
		var sub model.Submission
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("document_id = ? AND user_id = ?", in.DocumentID, in.UserID).
			First(&sub).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errs.New(http.StatusForbidden, "NOT_RECIPIENT", "您不是该公文的接收人")
			}
			return err
		}
		var doc model.Document
		if err := tx.First(&doc, in.DocumentID).Error; err != nil {
			return err
		}
		if doc.Status != model.DocumentStatusActive {
			return errs.New(http.StatusBadRequest, "DOC_INACTIVE", "公文已撤回")
		}

		// 状态合法性: PENDING/RETURNED 才能提交;SUBMITTED/SUBMITTED_LATE 必须先退回
		switch sub.CurrentStatus {
		case model.SubStatusPending, model.SubStatusReturned:
			// ok
		case model.SubStatusSubmitted, model.SubStatusSubmittedLate:
			return errs.New(http.StatusBadRequest, "ALREADY_SUBMITTED", "已上报,如需修改请联系上级退回")
		default:
			return errs.New(http.StatusBadRequest, "BAD_STATE", "当前状态不允许提交")
		}

		// 校验附件归属
		var atts []model.Attachment
		if err := tx.Where("id IN ?", in.AttachmentIDs).Find(&atts).Error; err != nil {
			return err
		}
		if len(atts) != len(in.AttachmentIDs) {
			return errs.New(http.StatusBadRequest, "ATT_NOT_FOUND", "存在不存在的附件 ID")
		}
		for _, a := range atts {
			if a.UploaderID != in.UserID {
				return errs.New(http.StatusForbidden, "BAD_OWNER", "附件不属于当前用户")
			}
		}

		isResubmit := sub.CurrentStatus == model.SubStatusReturned

		// 重提: 删除旧 submission 附件 (DB + 磁盘)
		if isResubmit {
			var old []model.Attachment
			if err := tx.Where("owner_type = ? AND owner_id = ?", model.OwnerSubmission, sub.ID).
				Find(&old).Error; err != nil {
				return err
			}
			for _, a := range old {
				_ = s.storage.Remove(a.StoredPath)
			}
			if err := tx.Where("owner_type = ? AND owner_id = ?", model.OwnerSubmission, sub.ID).
				Delete(&model.Attachment{}).Error; err != nil {
				return err
			}
		}

		// 把新附件重新挂到 SUBMISSION/sub.ID,并搬到正式目录
		for i := range atts {
			a := &atts[i]
			newPath, err := s.storage.MoveDraft(a.StoredPath, storage.OwnerSubmission, "", sub.ID)
			if err != nil {
				return err
			}
			updates := map[string]any{
				"owner_type":  model.OwnerSubmission,
				"owner_id":    sub.ID,
				"purpose":     "",
				"stored_path": newPath,
			}
			if err := tx.Model(a).Updates(updates).Error; err != nil {
				return err
			}
		}

		// 状态迁移: 准时 / 逾期
		now := time.Now()
		newStatus := model.SubStatusSubmitted
		if doc.Deadline != nil && now.After(*doc.Deadline) {
			newStatus = model.SubStatusSubmittedLate
		}
		sub.CurrentStatus = newStatus
		sub.SubmittedAt = &now
		sub.LastActionAt = now
		sub.Note = strings.TrimSpace(in.Note)
		if err := tx.Save(&sub).Error; err != nil {
			return err
		}

		actionType := model.ActionSubmit
		if isResubmit {
			actionType = model.ActionResubmit
		}
		act := &model.SubmissionAction{
			SubmissionID: sub.ID,
			ActionType:   actionType,
			OperatorID:   in.UserID,
		}
		if err := tx.Create(act).Error; err != nil {
			return err
		}
		subResult = &sub
		return nil
	})
	if err != nil {
		return nil, err
	}
	return subResult, nil
}

// GetMySubmissions 普通用户查看自己的上报历史
type ListMyFilter struct {
	UserID int64
	Status string // PENDING/SUBMITTED/RETURNED/...
	Page   int
	Size   int
}

type MySubmissionItem struct {
	Submission    model.Submission `json:"submission"`
	DisplayStatus string           `json:"display_status"`
	Document      model.Document   `json:"document"`
}

func (s *SubmissionService) ListMine(f ListMyFilter) ([]MySubmissionItem, int64, error) {
	if f.Page <= 0 {
		f.Page = 1
	}
	if f.Size <= 0 || f.Size > 100 {
		f.Size = 20
	}
	q := s.db.Model(&model.Submission{}).Where("user_id = ?", f.UserID)
	if f.Status != "" {
		q = q.Where("current_status = ?", f.Status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var subs []model.Submission
	if err := q.Order("last_action_at DESC").
		Limit(f.Size).Offset((f.Page - 1) * f.Size).
		Find(&subs).Error; err != nil {
		return nil, 0, err
	}
	if len(subs) == 0 {
		return []MySubmissionItem{}, total, nil
	}
	docIDs := make([]int64, len(subs))
	for i, s := range subs {
		docIDs[i] = s.DocumentID
	}
	var docs []model.Document
	if err := s.db.Where("id IN ?", docIDs).Find(&docs).Error; err != nil {
		return nil, 0, err
	}
	docMap := make(map[int64]model.Document, len(docs))
	for _, d := range docs {
		docMap[d.ID] = d
	}
	out := make([]MySubmissionItem, len(subs))
	for i, sub := range subs {
		d := docMap[sub.DocumentID]
		out[i] = MySubmissionItem{
			Submission:    sub,
			Document:      d,
			DisplayStatus: displayStatus(&sub, &d),
		}
	}
	return out, total, nil
}

// GetDetail 查看一条上报的完整详情 (含附件 + 动作流水)。
// 权限:super 任意;dept 限本部门;user 仅本人。
type SubmissionDetailDTO struct {
	SubmissionDTO
	Document model.Document           `json:"document"`
	Actions  []model.SubmissionAction `json:"actions"`
}

func (s *SubmissionService) GetDetail(id int64, viewerID int64, viewerRole string, viewerDept *int64) (*SubmissionDetailDTO, error) {
	var sub model.Submission
	if err := s.db.First(&sub, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrNotFound
		}
		return nil, err
	}
	if err := assertCanViewSubmission(&sub, viewerID, viewerRole, viewerDept); err != nil {
		return nil, err
	}
	var doc model.Document
	if err := s.db.First(&doc, sub.DocumentID).Error; err != nil {
		return nil, err
	}
	dto := buildSubmissionDTO(s.db, &doc, &sub)
	var actions []model.SubmissionAction
	_ = s.db.Where("submission_id = ?", sub.ID).Order("id ASC").Find(&actions).Error
	if actions == nil {
		actions = []model.SubmissionAction{}
	}
	return &SubmissionDetailDTO{SubmissionDTO: dto, Document: doc, Actions: actions}, nil
}

func assertCanViewSubmission(sub *model.Submission, viewerID int64, viewerRole string, viewerDept *int64) error {
	switch viewerRole {
	case model.RoleSuper:
		return nil
	case model.RoleDept:
		if viewerDept != nil && sub.DepartmentID != nil && *viewerDept == *sub.DepartmentID {
			return nil
		}
		return errs.ErrForbidden
	default:
		if sub.UserID == viewerID {
			return nil
		}
		return errs.ErrForbidden
	}
}

// Return 退回上报。仅 super / 本部门 dept 可调用。
// 当前状态必须是 SUBMITTED 或 SUBMITTED_LATE,退回后变为 RETURNED。
func (s *SubmissionService) Return(id int64, reason string, operatorID int64, operatorRole string, operatorDept *int64) (*model.Submission, error) {
	reason = strings.TrimSpace(reason)
	if r := []rune(reason); len(r) < 5 {
		return nil, errs.New(http.StatusBadRequest, "BAD_REQUEST", "退回原因至少 5 字")
	}
	if len([]rune(reason)) > 500 {
		return nil, errs.New(http.StatusBadRequest, "BAD_REQUEST", "退回原因最长 500 字")
	}

	var result *model.Submission
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var sub model.Submission
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			First(&sub, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errs.ErrNotFound
			}
			return err
		}
		// 权限
		switch operatorRole {
		case model.RoleSuper:
			// ok
		case model.RoleDept:
			if operatorDept == nil || sub.DepartmentID == nil || *operatorDept != *sub.DepartmentID {
				return errs.ErrForbidden
			}
		default:
			return errs.ErrForbidden
		}
		// 状态
		if sub.CurrentStatus != model.SubStatusSubmitted && sub.CurrentStatus != model.SubStatusSubmittedLate {
			return errs.New(http.StatusBadRequest, "BAD_STATE", "仅已上报的记录可被退回")
		}
		now := time.Now()
		sub.CurrentStatus = model.SubStatusReturned
		sub.ReturnReason = reason
		sub.ReturnCount += 1
		sub.LastActionAt = now
		if err := tx.Save(&sub).Error; err != nil {
			return err
		}
		if err := tx.Create(&model.SubmissionAction{
			SubmissionID: sub.ID,
			ActionType:   model.ActionReturn,
			OperatorID:   operatorID,
			Reason:       reason,
		}).Error; err != nil {
			return err
		}
		result = &sub
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
