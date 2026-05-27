package storage

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

type LocalStorage struct {
	root string
}

func NewLocal(root string) (*LocalStorage, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(abs, 0o750); err != nil {
		return nil, err
	}
	return &LocalStorage{root: abs}, nil
}

func (s *LocalStorage) Root() string { return s.root }

// subdir 返回相对路径的目录部分(不含磁盘文件名)
func (s *LocalStorage) subdir(ownerType OwnerType, purpose Purpose, ownerID int64) string {
	switch ownerType {
	case OwnerDocument:
		switch purpose {
		case PurposeTemplate:
			return filepath.Join("documents", fmt.Sprint(ownerID), "template")
		default:
			return filepath.Join("documents", fmt.Sprint(ownerID), "reading")
		}
	case OwnerDocumentDraft:
		// draft 期间还没有正式 docID,挂在用户级 draft 区。ownerID 此时是 uploaderID。
		return filepath.Join("documents", "_draft", fmt.Sprint(ownerID))
	case OwnerSubmission:
		return filepath.Join("submissions", fmt.Sprint(ownerID))
	case OwnerInline:
		return filepath.Join("inline-images", time.Now().Format("2006-01"))
	default:
		return filepath.Join("misc", fmt.Sprint(ownerID))
	}
}

func (s *LocalStorage) Save(ownerType OwnerType, purpose Purpose, ownerID int64, attachmentID int64,
	originalName string, src io.Reader) (*StoredFile, error) {

	rel := s.subdir(ownerType, purpose, ownerID)
	absDir, err := s.safeJoin(rel)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(absDir, 0o750); err != nil {
		return nil, err
	}

	diskName := buildDiskName(ownerType, attachmentID, originalName)
	relPath := filepath.ToSlash(filepath.Join(rel, diskName))
	absPath := filepath.Join(absDir, diskName)

	out, err := os.OpenFile(absPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		return nil, err
	}
	defer out.Close()

	n, err := io.Copy(out, src)
	if err != nil {
		_ = os.Remove(absPath)
		return nil, err
	}
	return &StoredFile{RelativePath: relPath, SizeBytes: n}, nil
}

func (s *LocalStorage) Open(relativePath string) (io.ReadSeekCloser, error) {
	abs, err := s.safeJoin(relativePath)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(abs)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (s *LocalStorage) Remove(relativePath string) error {
	abs, err := s.safeJoin(relativePath)
	if err != nil {
		return err
	}
	err = os.Remove(abs)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (s *LocalStorage) MoveDraft(relativePath string, targetType OwnerType, purpose Purpose, newOwnerID int64) (string, error) {
	if !strings.Contains(relativePath, "/_draft/") {
		return relativePath, nil // 已不是 draft,跳过
	}
	oldAbs, err := s.safeJoin(relativePath)
	if err != nil {
		return "", err
	}
	diskName := filepath.Base(relativePath)
	newRel := filepath.ToSlash(filepath.Join(s.subdir(targetType, purpose, newOwnerID), diskName))
	newAbs, err := s.safeJoin(newRel)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(newAbs), 0o750); err != nil {
		return "", err
	}
	if err := os.Rename(oldAbs, newAbs); err != nil {
		return "", err
	}
	return newRel, nil
}

// safeJoin 把相对路径拼到 root 下,确保结果仍在 root 之内,防穿越
func (s *LocalStorage) safeJoin(rel string) (string, error) {
	abs := filepath.Join(s.root, filepath.FromSlash(rel))
	abs = filepath.Clean(abs)
	rootClean := filepath.Clean(s.root)
	if abs != rootClean && !strings.HasPrefix(abs, rootClean+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes storage root: %s", rel)
	}
	return abs, nil
}

// --- 文件名清洗 ---

var (
	reUnsafe         = regexp.MustCompile(`[\x00-\x1f\x7f<>:"/\\|?*]`)
	winReservedNames = map[string]bool{
		"con": true, "prn": true, "aux": true, "nul": true,
		"com1": true, "com2": true, "com3": true, "com4": true,
		"com5": true, "com6": true, "com7": true, "com8": true, "com9": true,
		"lpt1": true, "lpt2": true, "lpt3": true, "lpt4": true,
		"lpt5": true, "lpt6": true, "lpt7": true, "lpt8": true, "lpt9": true,
	}
)

func sanitizeFileName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "..", "_")
	name = reUnsafe.ReplaceAllString(name, "_")

	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	if winReservedNames[strings.ToLower(base)] {
		base = "_" + base
	}
	if base == "" {
		base = "unnamed"
	}

	// 长度截断到 ~180 字节,留出 ID 前缀空间
	out := base + ext
	for len(out) > 180 {
		base = base[:len(base)-1]
		out = base + ext
	}
	return out
}

// buildDiskName 生成磁盘文件名: <id>__<safe>(.ext) 或 <uuid>.ext (inline)
func buildDiskName(ownerType OwnerType, attachmentID int64, originalName string) string {
	if ownerType == OwnerInline {
		ext := strings.ToLower(filepath.Ext(originalName))
		if ext == "" {
			ext = ".bin"
		}
		return uuid.NewString() + ext
	}
	safe := sanitizeFileName(originalName)
	return fmt.Sprintf("%d__%s", attachmentID, safe)
}
