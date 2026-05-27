package api

import (
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	errs "github.com/ksm/docflow/internal/errors"
	"github.com/ksm/docflow/internal/middleware"
	"github.com/ksm/docflow/internal/model"
	"github.com/ksm/docflow/internal/service"
)

type AttachmentHandler struct {
	svc *service.AttachmentService
}

func NewAttachmentHandler(svc *service.AttachmentService) *AttachmentHandler {
	return &AttachmentHandler{svc: svc}
}

// Upload form fields:
//   owner_type:  DOCUMENT_DRAFT | SUBMISSION | INLINE
//   purpose:     READING | TEMPLATE        (DOCUMENT_DRAFT 必填)
//   owner_id:    SUBMISSION 时填写;其他情况由后端推断 (DRAFT/INLINE = uploaderID)
//   file:        <file>
func (h *AttachmentHandler) Upload(c *gin.Context) {
	ownerType := c.PostForm("owner_type")
	purpose := c.PostForm("purpose")
	uploaderID := middleware.CurrentUserID(c)

	var ownerID int64
	switch ownerType {
	case model.OwnerSubmission:
		v := c.PostForm("owner_id")
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			abortWithError(c, errs.New(http.StatusBadRequest, "BAD_REQUEST", "owner_id 非法"))
			return
		}
		ownerID = id
	case model.OwnerDocumentDraft, model.OwnerInline:
		ownerID = uploaderID
	default:
		abortWithError(c, errs.New(http.StatusBadRequest, "BAD_REQUEST", "owner_type 非法"))
		return
	}

	fh, err := c.FormFile("file")
	if err != nil {
		abortWithError(c, errs.New(http.StatusBadRequest, "BAD_REQUEST", "缺少 file 字段"))
		return
	}
	f, err := fh.Open()
	if err != nil {
		abortWithError(c, errs.New(http.StatusBadRequest, "BAD_REQUEST", "读取文件失败"))
		return
	}
	defer f.Close()

	att, err := h.svc.Upload(service.UploadInput{
		OwnerType:    ownerType,
		Purpose:      purpose,
		OwnerID:      ownerID,
		UploaderID:   uploaderID,
		OriginalName: fh.Filename,
		Size:         fh.Size,
		Reader:       f,
	})
	if err != nil {
		abortWithError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":           att.ID,
		"file_name":    att.FileName,
		"size_bytes":   att.SizeBytes,
		"mime_type":    att.MimeType,
		"preview_url":  fmt.Sprintf("/api/v1/attachments/%d/preview", att.ID),
		"download_url": fmt.Sprintf("/api/v1/attachments/%d/download", att.ID),
	})
}

func (h *AttachmentHandler) Download(c *gin.Context) {
	h.stream(c, false)
}

func (h *AttachmentHandler) Preview(c *gin.Context) {
	h.stream(c, true)
}

func (h *AttachmentHandler) stream(c *gin.Context, inline bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		abortWithError(c, errs.New(http.StatusBadRequest, "BAD_REQUEST", "id 非法"))
		return
	}
	att, err := h.svc.Get(id)
	if err != nil {
		abortWithError(c, err)
		return
	}
	if inline && !service.IsPDF(att) {
		abortWithError(c, errs.New(http.StatusUnsupportedMediaType, "NOT_PDF", "仅 PDF 支持在线预览"))
		return
	}
	f, err := h.svc.OpenStream(att)
	if err != nil {
		abortWithError(c, err)
		return
	}
	defer f.Close()

	mt := att.MimeType
	if mt == "" {
		mt = mime.TypeByExtension(filepath.Ext(att.FileName))
		if mt == "" {
			mt = "application/octet-stream"
		}
	}
	disp := "attachment"
	if inline {
		disp = "inline"
		mt = "application/pdf"
	}
	encName := url.PathEscape(att.FileName)
	c.Writer.Header().Set("Content-Type", mt)
	c.Writer.Header().Set("Content-Disposition",
		fmt.Sprintf(`%s; filename="%s"; filename*=UTF-8''%s`,
			disp, strings.ReplaceAll(att.FileName, `"`, `\"`), encName))
	c.Writer.Header().Set("Content-Length", strconv.FormatInt(att.SizeBytes, 10))
	http.ServeContent(c.Writer, c.Request, att.FileName, att.UploadedAt, f)
}

func (h *AttachmentHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		abortWithError(c, errs.New(http.StatusBadRequest, "BAD_REQUEST", "id 非法"))
		return
	}
	att, err := h.svc.Get(id)
	if err != nil {
		abortWithError(c, err)
		return
	}
	uid := middleware.CurrentUserID(c)
	role := middleware.CurrentRole(c)
	if att.UploaderID != uid && role != model.RoleSuper {
		abortWithError(c, errs.ErrForbidden)
		return
	}
	if err := h.svc.Delete(att); err != nil {
		abortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
