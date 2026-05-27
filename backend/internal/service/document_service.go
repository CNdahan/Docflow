package service

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/microcosm-cc/bluemonday"
	"gorm.io/gorm"

	"github.com/ksm/docflow/internal/config"
	errs "github.com/ksm/docflow/internal/errors"
	"github.com/ksm/docflow/internal/model"
	"github.com/ksm/docflow/internal/storage"
)

type DocumentService struct {
	db        *gorm.DB
	cfg       *config.Config
	storage   storage.Storage
	sanitizer *bluemonday.Policy
}

func NewDocumentService(db *gorm.DB, cfg *config.Config, s storage.Storage) *DocumentService {
	policy := bluemonday.UGCPolicy()
	// 允许富文本编辑器常用的样式属性
	policy.AllowAttrs("style").OnElements("p", "span", "h1", "h2", "h3", "h4", "td", "th", "div")
	policy.AllowAttrs("class").OnElements("p", "span", "h1", "h2", "h3", "div", "pre", "code")
	policy.AllowImages()
	return &DocumentService{db: db, cfg: cfg, storage: s, sanitizer: policy}
}

type PublishInput struct {
	Title                 string    `json:"title" binding:"required,min=1,max=200"`
	ContentHTML           string    `json:"content_html" binding:"required"`
	TargetScope           string    `json:"target_scope" binding:"required,oneof=DEPARTMENT ALL_USERS OWN_DEPARTMENT"`
	TargetDepartmentIDs   []int64   `json:"target_department_ids"`
	Deadline              *time.Time `json:"deadline"`
	ReadingAttachmentIDs  []int64   `json:"reading_attachment_ids"`
	TemplateAttachmentIDs []int64   `json:"template_attachment_ids"`
}

// Publish 发布公文,事务内物化目标接收人 + 转挂附件
func (s *DocumentService) Publish(publisherID int64, publisherRole string, publisherDept *int64, in PublishInput) (*model.Document, error) {
	if in.TargetScope == model.ScopeOwnDepartment {
		if publisherRole != model.RoleDept || publisherDept == nil {
			return nil, errs.New(http.StatusForbidden, "FORBIDDEN", "OWN_DEPARTMENT 仅部门用户可用")
		}
	}
	if in.TargetScope == model.ScopeAllUsers && publisherRole != model.RoleSuper {
		return nil, errs.New(http.StatusForbidden, "FORBIDDEN", "ALL_USERS 仅顶级用户可用")
	}
	if in.TargetScope == model.ScopeDepartment {
		if publisherRole != model.RoleSuper {
			return nil, errs.New(http.StatusForbidden, "FORBIDDEN", "DEPARTMENT 仅顶级用户可用")
		}
		if len(in.TargetDepartmentIDs) == 0 {
			return nil, errs.New(http.StatusBadRequest, "BAD_REQUEST", "DEPARTMENT 范围必须选择至少一个部门")
		}
	}
	if len(in.ReadingAttachmentIDs) > s.cfg.Storage.MaxAttachmentsPerDocument {
		return nil, errs.New(http.StatusBadRequest, "TOO_MANY_FILES", "阅读附件数量超出限制")
	}
	if len(in.TemplateAttachmentIDs) > s.cfg.Storage.MaxTemplatesPerDocument {
		return nil, errs.New(http.StatusBadRequest, "TOO_MANY_FILES", "模板附件数量超出限制")
	}

	cleanHTML := s.sanitizer.Sanitize(in.ContentHTML)

	doc := &model.Document{
		Title:         strings.TrimSpace(in.Title),
		ContentHTML:   cleanHTML,
		PublisherID:   publisherID,
		PublisherDept: publisherDept,
		TargetScope:   in.TargetScope,
		Deadline:      in.Deadline,
		Status:        model.DocumentStatusActive,
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		// 1. 解析接收人列表 (按 scope)
		recipientUserIDs, recipientDeptIDs, err := s.resolveRecipients(tx, in.TargetScope, in.TargetDepartmentIDs, publisherDept)
		if err != nil {
			return err
		}
		if len(recipientUserIDs) == 0 {
			return errs.New(http.StatusBadRequest, "NO_RECIPIENT", "目标范围内没有可接收的用户")
		}

		// 2. 落库 documents
		if err := tx.Create(doc).Error; err != nil {
			return err
		}

		// 3. 物化 document_targets + submissions
		targets := make([]model.DocumentTarget, 0, len(recipientUserIDs))
		subs := make([]model.Submission, 0, len(recipientUserIDs))
		now := time.Now()
		for _, uid := range recipientUserIDs {
			deptID := recipientDeptIDs[uid]
			targets = append(targets, model.DocumentTarget{
				DocumentID: doc.ID, UserID: uid, DepartmentID: deptID,
			})
			subs = append(subs, model.Submission{
				DocumentID:    doc.ID,
				UserID:        uid,
				DepartmentID:  deptID,
				CurrentStatus: model.SubStatusPending,
				LastActionAt:  now,
			})
		}
		if err := tx.Create(&targets).Error; err != nil {
			return err
		}
		if err := tx.CreateInBatches(&subs, 200).Error; err != nil {
			return err
		}

		// 4. 转挂附件 (draft → 正式),同时校验归属 (必须是发布者上传的 draft)
		if err := s.attachToDocument(tx, doc.ID, publisherID, in.ReadingAttachmentIDs, model.PurposeReading); err != nil {
			return err
		}
		if err := s.attachToDocument(tx, doc.ID, publisherID, in.TemplateAttachmentIDs, model.PurposeTemplate); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}
	return doc, nil
}

func (s *DocumentService) resolveRecipients(tx *gorm.DB, scope string, deptIDs []int64, publisherDept *int64) ([]int64, map[int64]*int64, error) {
	var users []model.User
	q := tx.Model(&model.User{}).Where("role = ? AND disabled = false", model.RoleUser)
	switch scope {
	case model.ScopeDepartment:
		q = q.Where("department_id IN ?", deptIDs)
	case model.ScopeOwnDepartment:
		q = q.Where("department_id = ?", *publisherDept)
	case model.ScopeAllUsers:
		// no extra
	}
	if err := q.Find(&users).Error; err != nil {
		return nil, nil, err
	}
	ids := make([]int64, 0, len(users))
	deptMap := make(map[int64]*int64, len(users))
	for _, u := range users {
		ids = append(ids, u.ID)
		deptMap[u.ID] = u.DepartmentID
	}
	return ids, deptMap, nil
}

func (s *DocumentService) attachToDocument(tx *gorm.DB, docID int64, publisherID int64, attIDs []int64, purpose string) error {
	if len(attIDs) == 0 {
		return nil
	}
	var atts []model.Attachment
	if err := tx.Where("id IN ?", attIDs).Find(&atts).Error; err != nil {
		return err
	}
	if len(atts) != len(attIDs) {
		return errs.New(http.StatusBadRequest, "ATT_NOT_FOUND", "存在不存在的附件 ID")
	}
	for i := range atts {
		a := &atts[i]
		if a.OwnerType != model.OwnerDocumentDraft || a.UploaderID != publisherID {
			return errs.New(http.StatusBadRequest, "BAD_OWNER", fmt.Sprintf("附件 %d 不是当前用户的草稿", a.ID))
		}
		newPath, err := s.storage.MoveDraft(a.StoredPath, storage.OwnerDocument, storage.Purpose(purpose), docID)
		if err != nil {
			return err
		}
		updates := map[string]any{
			"owner_type":  model.OwnerDocument,
			"owner_id":    docID,
			"purpose":     purpose,
			"stored_path": newPath,
		}
		if err := tx.Model(a).Updates(updates).Error; err != nil {
			return err
		}
	}
	return nil
}

// ---- 查询 ----

type DocumentSummary struct {
	Total     int64 `json:"total"`
	Submitted int64 `json:"submitted"`
	Late      int64 `json:"late"`
	Pending   int64 `json:"pending"`
	Overdue   int64 `json:"overdue"`
	Returned  int64 `json:"returned"`
}

type DocumentListItem struct {
	model.Document
	Publisher   *model.User      `json:"publisher,omitempty"`
	Stats       *DocumentSummary `json:"stats,omitempty"`
}

type ListDocsFilter struct {
	RoleView string // publish / inbox / all
	UserID   int64
	UserRole string
	UserDept *int64
	Page     int
	Size     int
}

type ListDocsResult struct {
	Items []DocumentListItem `json:"items"`
	Total int64              `json:"total"`
}

func (s *DocumentService) List(f ListDocsFilter) (*ListDocsResult, error) {
	if f.Page <= 0 {
		f.Page = 1
	}
	if f.Size <= 0 || f.Size > 100 {
		f.Size = 20
	}
	q := s.db.Model(&model.Document{}).Where("status = ?", model.DocumentStatusActive)

	switch f.RoleView {
	case "publish":
		q = q.Where("publisher_id = ?", f.UserID)
	case "inbox":
		q = q.Where("id IN (SELECT document_id FROM document_targets WHERE user_id = ?)", f.UserID)
	default:
		// super 全部;其他角色至少能看到自己相关的
		if f.UserRole != model.RoleSuper {
			q = q.Where("publisher_id = ? OR id IN (SELECT document_id FROM document_targets WHERE user_id = ?)",
				f.UserID, f.UserID)
		}
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}
	var docs []model.Document
	if err := q.Order("id DESC").
		Limit(f.Size).Offset((f.Page - 1) * f.Size).
		Find(&docs).Error; err != nil {
		return nil, err
	}

	items := make([]DocumentListItem, len(docs))
	for i := range docs {
		items[i] = DocumentListItem{Document: docs[i]}
	}
	// 批量补充发布者 + 统计 (有就补,失败不阻塞)
	if len(docs) > 0 {
		pubIDs := uniqueInts(docs, func(d model.Document) int64 { return d.PublisherID })
		var pubs []model.User
		_ = s.db.Where("id IN ?", pubIDs).Find(&pubs).Error
		pubMap := make(map[int64]model.User, len(pubs))
		for _, u := range pubs {
			pubMap[u.ID] = u
		}
		docIDs := uniqueInts(docs, func(d model.Document) int64 { return d.ID })
		statMap := s.computeSummaries(docIDs)
		for i := range items {
			if u, ok := pubMap[items[i].PublisherID]; ok {
				cp := u
				items[i].Publisher = &cp
			}
			if st, ok := statMap[items[i].ID]; ok {
				items[i].Stats = &st
			}
		}
	}
	return &ListDocsResult{Items: items, Total: total}, nil
}

func uniqueInts[T any](items []T, key func(T) int64) []int64 {
	m := make(map[int64]struct{}, len(items))
	out := make([]int64, 0, len(items))
	for _, it := range items {
		k := key(it)
		if _, ok := m[k]; ok {
			continue
		}
		m[k] = struct{}{}
		out = append(out, k)
	}
	return out
}

func (s *DocumentService) computeSummaries(docIDs []int64) map[int64]DocumentSummary {
	type row struct {
		DocumentID int64
		Status     string
		Cnt        int64
	}
	var rows []row
	_ = s.db.Model(&model.Submission{}).
		Select("document_id, current_status AS status, COUNT(*) AS cnt").
		Where("document_id IN ?", docIDs).
		Group("document_id, current_status").Scan(&rows).Error

	// 取出文档 deadline 用于实时算 overdue
	var docs []model.Document
	_ = s.db.Select("id, deadline").Where("id IN ?", docIDs).Find(&docs).Error
	ddlMap := make(map[int64]*time.Time, len(docs))
	for _, d := range docs {
		ddlMap[d.ID] = d.Deadline
	}

	now := time.Now()
	out := make(map[int64]DocumentSummary, len(docIDs))
	for _, r := range rows {
		s := out[r.DocumentID]
		s.Total += r.Cnt
		switch r.Status {
		case model.SubStatusSubmitted:
			s.Submitted += r.Cnt
		case model.SubStatusSubmittedLate:
			s.Late += r.Cnt
		case model.SubStatusReturned:
			s.Returned += r.Cnt
		case model.SubStatusPending:
			// 区分待办 vs 逾期未交
			if ddl := ddlMap[r.DocumentID]; ddl != nil && ddl.Before(now) {
				s.Overdue += r.Cnt
			} else {
				s.Pending += r.Cnt
			}
		}
		out[r.DocumentID] = s
	}
	return out
}

// GetDetail 取公文详情, 含附件 + 可选我的上报
type DocumentDetail struct {
	model.Document
	Publisher           *model.User         `json:"publisher,omitempty"`
	ReadingAttachments  []model.Attachment  `json:"reading_attachments"`
	TemplateAttachments []model.Attachment  `json:"template_attachments"`
	MySubmission        *SubmissionDTO      `json:"my_submission,omitempty"`
	TargetDepartmentIDs []int64             `json:"target_department_ids,omitempty"`
}

func (s *DocumentService) GetDetail(docID int64, viewerID int64, viewerRole string) (*DocumentDetail, error) {
	var doc model.Document
	if err := s.db.First(&doc, docID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrNotFound
		}
		return nil, err
	}
	// 权限: super 全开;发布者本人可看;接收人可看
	if viewerRole != model.RoleSuper && doc.PublisherID != viewerID {
		var sub model.Submission
		if err := s.db.Where("document_id = ? AND user_id = ?", doc.ID, viewerID).First(&sub).Error; err != nil {
			return nil, errs.ErrForbidden
		}
	}

	d := &DocumentDetail{Document: doc}
	var pub model.User
	if err := s.db.First(&pub, doc.PublisherID).Error; err == nil {
		d.Publisher = &pub
	}

	var atts []model.Attachment
	_ = s.db.Where("owner_type = ? AND owner_id = ?", model.OwnerDocument, doc.ID).
		Order("id ASC").Find(&atts).Error
	for _, a := range atts {
		switch a.Purpose {
		case model.PurposeTemplate:
			d.TemplateAttachments = append(d.TemplateAttachments, a)
		default:
			d.ReadingAttachments = append(d.ReadingAttachments, a)
		}
	}
	if d.ReadingAttachments == nil {
		d.ReadingAttachments = []model.Attachment{}
	}
	if d.TemplateAttachments == nil {
		d.TemplateAttachments = []model.Attachment{}
	}

	// 目标部门 (仅 DEPARTMENT scope)
	if doc.TargetScope == model.ScopeDepartment {
		type r struct{ DepartmentID *int64 }
		var rs []r
		_ = s.db.Model(&model.DocumentTarget{}).
			Select("DISTINCT department_id").
			Where("document_id = ? AND department_id IS NOT NULL", doc.ID).
			Scan(&rs).Error
		for _, x := range rs {
			if x.DepartmentID != nil {
				d.TargetDepartmentIDs = append(d.TargetDepartmentIDs, *x.DepartmentID)
			}
		}
	}

	// 当前 viewer 的上报 (如果是接收人)
	var mySub model.Submission
	if err := s.db.Where("document_id = ? AND user_id = ?", doc.ID, viewerID).First(&mySub).Error; err == nil {
		dto := buildSubmissionDTO(s.db, &doc, &mySub)
		d.MySubmission = &dto
	}

	return d, nil
}

// Recall 撤回公文,仅发布者或 super 可操作
func (s *DocumentService) Recall(docID, operatorID int64, operatorRole string) error {
	var doc model.Document
	if err := s.db.First(&doc, docID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errs.ErrNotFound
		}
		return err
	}
	if operatorRole != model.RoleSuper && doc.PublisherID != operatorID {
		return errs.ErrForbidden
	}
	if doc.Status == model.DocumentStatusRecalled {
		return nil
	}
	return s.db.Model(&doc).Update("status", model.DocumentStatusRecalled).Error
}
