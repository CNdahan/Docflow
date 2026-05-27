package service

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gabriel-vasile/mimetype"
	"gorm.io/gorm"

	"github.com/ksm/docflow/internal/config"
	errs "github.com/ksm/docflow/internal/errors"
	"github.com/ksm/docflow/internal/model"
	"github.com/ksm/docflow/internal/storage"
)

type AttachmentService struct {
	db      *gorm.DB
	cfg     *config.Config
	storage storage.Storage
}

func NewAttachmentService(db *gorm.DB, cfg *config.Config, s storage.Storage) *AttachmentService {
	return &AttachmentService{db: db, cfg: cfg, storage: s}
}

// 允许的 MIME / 扩展名白名单 (按用途区分)
var (
	allowedDocReadingExt = stringSet(".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx",
		".jpg", ".jpeg", ".png")
	allowedDocTemplateExt = stringSet(".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx", ".zip")
	allowedSubmissionExt  = stringSet(".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx",
		".jpg", ".jpeg", ".png")
	allowedInlineExt = stringSet(".jpg", ".jpeg", ".png", ".gif", ".webp")
)

func stringSet(items ...string) map[string]struct{} {
	m := make(map[string]struct{}, len(items))
	for _, it := range items {
		m[it] = struct{}{}
	}
	return m
}

type UploadInput struct {
	OwnerType    string  // DOCUMENT_DRAFT / SUBMISSION / INLINE / DOCUMENT (内部使用)
	Purpose      string  // 仅 DOCUMENT_DRAFT/DOCUMENT 时填 READING/TEMPLATE
	OwnerID      int64   // DOCUMENT_DRAFT 时 = uploaderID; SUBMISSION 时 = submissionID
	UploaderID   int64
	OriginalName string
	Size         int64
	Reader       io.Reader
}

// Upload 写入文件 + 创建 attachment 记录,事务内完成
func (s *AttachmentService) Upload(in UploadInput) (*model.Attachment, error) {
	if in.OriginalName == "" {
		return nil, errs.New(http.StatusBadRequest, "BAD_REQUEST", "缺少文件名")
	}
	if in.Size <= 0 {
		return nil, errs.New(http.StatusBadRequest, "BAD_REQUEST", "文件为空")
	}
	limit := s.cfg.Storage.MaxFileSize
	if in.OwnerType == model.OwnerInline {
		limit = s.cfg.Storage.MaxInlineImageSize
	}
	if in.Size > limit {
		return nil, errs.New(http.StatusRequestEntityTooLarge, "FILE_TOO_LARGE", "文件超过大小限制")
	}

	ext := strings.ToLower(filepath.Ext(in.OriginalName))
	if err := s.validateExt(in.OwnerType, in.Purpose, ext); err != nil {
		return nil, err
	}

	// 读取头部做 MIME magic 校验,同时把内容缓存供后续写入
	head := make([]byte, 512)
	buf := &bytes.Buffer{}
	tee := io.TeeReader(in.Reader, buf)
	n, _ := io.ReadFull(tee, head)
	mt := mimetype.Detect(head[:n])
	if err := s.validateMime(in.OwnerType, in.Purpose, mt, ext); err != nil {
		return nil, err
	}

	// 创建 DB 记录 (先占 ID),再写文件,失败回滚
	att := &model.Attachment{
		OwnerType:  in.OwnerType,
		OwnerID:    in.OwnerID,
		Purpose:    in.Purpose,
		FileName:   in.OriginalName,
		MimeType:   mt.String(),
		SizeBytes:  in.Size,
		UploaderID: in.UploaderID,
	}
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()
	if err := tx.Create(att).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// 实际写入磁盘:把已读的 head + 剩余流拼接
	combined := io.MultiReader(buf, in.Reader)
	stored, err := s.storage.Save(
		storage.OwnerType(in.OwnerType),
		storage.Purpose(in.Purpose),
		in.OwnerID, att.ID,
		in.OriginalName, combined,
	)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	if stored.SizeBytes != in.Size {
		_ = s.storage.Remove(stored.RelativePath)
		tx.Rollback()
		return nil, errs.New(http.StatusBadRequest, "SIZE_MISMATCH", "实际接收字节数与声明不符")
	}
	att.StoredPath = stored.RelativePath
	if err := tx.Save(att).Error; err != nil {
		_ = s.storage.Remove(stored.RelativePath)
		tx.Rollback()
		return nil, err
	}
	if err := tx.Commit().Error; err != nil {
		_ = s.storage.Remove(stored.RelativePath)
		return nil, err
	}
	return att, nil
}

func (s *AttachmentService) validateExt(ownerType, purpose, ext string) error {
	var allowed map[string]struct{}
	switch ownerType {
	case model.OwnerDocumentDraft, model.OwnerDocument:
		if purpose == model.PurposeTemplate {
			allowed = allowedDocTemplateExt
		} else {
			allowed = allowedDocReadingExt
		}
	case model.OwnerSubmission:
		allowed = allowedSubmissionExt
	case model.OwnerInline:
		allowed = allowedInlineExt
	default:
		return errs.New(http.StatusBadRequest, "BAD_REQUEST", "未知的 owner_type")
	}
	if _, ok := allowed[ext]; !ok {
		return errs.New(http.StatusUnsupportedMediaType, "BAD_EXT", "不支持的文件扩展名: "+ext)
	}
	return nil
}

func (s *AttachmentService) validateMime(ownerType, purpose string, mt *mimetype.MIME, ext string) error {
	// 简化版: 仅校验 magic number 不为可执行/脚本类。完整 ext/mime 映射可后续完善。
	suspicious := []string{
		"application/x-msdownload",
		"application/x-executable",
		"application/x-sh",
		"text/x-php",
	}
	for _, s := range suspicious {
		if mt.Is(s) {
			return errs.New(http.StatusUnsupportedMediaType, "BAD_MIME", "可疑文件类型: "+mt.String())
		}
	}
	return nil
}

func (s *AttachmentService) Get(id int64) (*model.Attachment, error) {
	var att model.Attachment
	if err := s.db.First(&att, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrNotFound
		}
		return nil, err
	}
	return &att, nil
}

func (s *AttachmentService) OpenStream(att *model.Attachment) (io.ReadSeekCloser, error) {
	return s.storage.Open(att.StoredPath)
}

func (s *AttachmentService) Delete(att *model.Attachment) error {
	if err := s.db.Delete(att).Error; err != nil {
		return err
	}
	return s.storage.Remove(att.StoredPath)
}

func (s *AttachmentService) ListByOwner(ownerType string, ownerID int64) ([]model.Attachment, error) {
	var atts []model.Attachment
	if err := s.db.Where("owner_type = ? AND owner_id = ?", ownerType, ownerID).
		Order("id ASC").Find(&atts).Error; err != nil {
		return nil, err
	}
	return atts, nil
}

// IsPDF 判断附件是否为 PDF,用于预览接口
func IsPDF(att *model.Attachment) bool {
	if att == nil {
		return false
	}
	if strings.ToLower(filepath.Ext(att.FileName)) == ".pdf" {
		return true
	}
	return att.MimeType == "application/pdf"
}
