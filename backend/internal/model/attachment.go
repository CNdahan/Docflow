package model

import "time"

const (
	OwnerDocument      = "DOCUMENT"
	OwnerDocumentDraft = "DOCUMENT_DRAFT"
	OwnerSubmission    = "SUBMISSION"
	OwnerInline        = "INLINE"

	PurposeReading  = "READING"
	PurposeTemplate = "TEMPLATE"
)

type Attachment struct {
	ID          int64     `gorm:"primaryKey" json:"id"`
	OwnerType   string    `gorm:"size:16;not null;index:idx_owner,priority:1" json:"owner_type"`
	OwnerID     int64     `gorm:"not null;index:idx_owner,priority:2" json:"owner_id"`
	Purpose     string    `gorm:"size:16" json:"purpose"`
	FileName    string    `gorm:"size:255;not null" json:"file_name"`
	StoredPath  string    `gorm:"size:500;not null;default:''" json:"stored_path"`
	MimeType    string    `gorm:"size:100;not null" json:"mime_type"`
	SizeBytes   int64     `gorm:"not null" json:"size_bytes"`
	UploaderID  int64     `gorm:"not null" json:"uploader_id"`
	UploadedAt  time.Time `gorm:"autoCreateTime" json:"uploaded_at"`
}

func (Attachment) TableName() string { return "attachments" }
