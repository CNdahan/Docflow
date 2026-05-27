package model

import "time"

const (
	SubStatusPending       = "PENDING"
	SubStatusSubmitted     = "SUBMITTED"
	SubStatusSubmittedLate = "SUBMITTED_LATE"
	SubStatusReturned      = "RETURNED"
	SubStatusOverdue       = "OVERDUE" // 虚拟,仅展示用

	ActionSubmit    = "SUBMIT"
	ActionReturn    = "RETURN"
	ActionResubmit  = "RESUBMIT"
)

type Submission struct {
	ID             int64      `gorm:"primaryKey" json:"id"`
	DocumentID     int64      `gorm:"not null;uniqueIndex:idx_doc_user,priority:1" json:"document_id"`
	UserID         int64      `gorm:"not null;uniqueIndex:idx_doc_user,priority:2" json:"user_id"`
	DepartmentID   *int64     `json:"department_id"`
	CurrentStatus  string     `gorm:"size:20;not null;default:'PENDING'" json:"current_status"`
	SubmittedAt    *time.Time `json:"submitted_at"`
	ReturnReason   string     `gorm:"type:text" json:"return_reason"`
	ReturnCount    int        `gorm:"not null;default:0" json:"return_count"`
	Note           string     `gorm:"type:text" json:"note"`
	LastActionAt   time.Time  `gorm:"not null" json:"last_action_at"`
}

func (Submission) TableName() string { return "submissions" }

type SubmissionAction struct {
	ID           int64     `gorm:"primaryKey" json:"id"`
	SubmissionID int64     `gorm:"not null" json:"submission_id"`
	ActionType   string    `gorm:"size:16;not null" json:"action_type"`
	OperatorID   int64     `gorm:"not null" json:"operator_id"`
	Reason       string    `gorm:"type:text" json:"reason"`
	CreatedAt    time.Time `json:"created_at"`
}

func (SubmissionAction) TableName() string { return "submission_actions" }
