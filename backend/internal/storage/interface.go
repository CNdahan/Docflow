package storage

import "io"

// OwnerType 决定附件存放的子目录与权限语义
type OwnerType string

const (
	OwnerDocument      OwnerType = "DOCUMENT"
	OwnerDocumentDraft OwnerType = "DOCUMENT_DRAFT" // 公文发布前的临时挂载
	OwnerSubmission    OwnerType = "SUBMISSION"
	OwnerInline        OwnerType = "INLINE"
)

// Purpose 仅对 DOCUMENT 类型有意义
type Purpose string

const (
	PurposeReading  Purpose = "READING"
	PurposeTemplate Purpose = "TEMPLATE"
)

// StoredFile 表示一次成功写入磁盘后的元信息
type StoredFile struct {
	RelativePath string // 相对 storage.root
	SizeBytes    int64
	MimeType     string
}

// Storage 抽象出来,便于后期换 OSS / S3 / MinIO
type Storage interface {
	// Save 把上传流写入磁盘。
	// ownerType + ownerID 决定子目录;attachmentID 用作磁盘文件名前缀防冲突
	Save(ownerType OwnerType, purpose Purpose, ownerID int64, attachmentID int64,
		originalName string, src io.Reader) (*StoredFile, error)
	// Open 读出文件流供下载/预览
	Open(relativePath string) (io.ReadSeekCloser, error)
	// Remove 物理删除
	Remove(relativePath string) error
	// MoveDraft 把 DOCUMENT_DRAFT 临时附件搬到正式目录。
	// targetType 决定目标分类(DOCUMENT/SUBMISSION),purpose 仅对 DOCUMENT 生效。
	MoveDraft(relativePath string, targetType OwnerType, purpose Purpose, newOwnerID int64) (string, error)
}
