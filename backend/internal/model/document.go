package model

import "time"

const (
	ScopeDepartment     = "DEPARTMENT"
	ScopeAllUsers       = "ALL_USERS"
	ScopeOwnDepartment  = "OWN_DEPARTMENT"
	DocumentStatusActive   = "ACTIVE"
	DocumentStatusRecalled = "RECALLED"

	RevisionContent     = "CONTENT"
	RevisionAttachment  = "ATTACHMENT"
	RevisionDeadline    = "DEADLINE"
	RevisionMeta        = "META"
)

type Document struct {
	ID            int64      `gorm:"primaryKey" json:"id"`
	Title         string     `gorm:"size:200;not null" json:"title"`
	ContentHTML   string     `gorm:"type:text;not null" json:"content_html"`
	PublisherID   int64      `gorm:"not null" json:"publisher_id"`
	PublisherDept *int64     `json:"publisher_dept"`
	TargetScope   string     `gorm:"size:16;not null" json:"target_scope"`
	Deadline      *time.Time `json:"deadline"`
	Status        string     `gorm:"size:16;not null;default:'ACTIVE'" json:"status"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

func (Document) TableName() string { return "documents" }

type DocumentTarget struct {
	DocumentID   int64  `gorm:"primaryKey" json:"document_id"`
	DepartmentID *int64 `json:"department_id"`
	UserID       int64  `gorm:"primaryKey" json:"user_id"`
}

func (DocumentTarget) TableName() string { return "document_targets" }

type DocumentRevision struct {
	ID           int64     `gorm:"primaryKey" json:"id"`
	DocumentID   int64     `gorm:"not null" json:"document_id"`
	EditorID     int64     `gorm:"not null" json:"editor_id"`
	ChangeType   string    `gorm:"size:16;not null" json:"change_type"`
	DiffSummary  string    `gorm:"type:text;not null" json:"diff_summary"`
	CreatedAt    time.Time `json:"created_at"`
}

func (DocumentRevision) TableName() string { return "document_revisions" }
