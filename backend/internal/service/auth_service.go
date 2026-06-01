package service

import (
	"errors"
	"net/http"
	"strings"

	"gorm.io/gorm"

	"github.com/ksm/docflow/internal/auth"
	"github.com/ksm/docflow/internal/config"
	errs "github.com/ksm/docflow/internal/errors"
	"github.com/ksm/docflow/internal/model"
)

type AuthService struct {
	db  *gorm.DB
	cfg *config.Config
	tm  *auth.TokenManager
}

func NewAuthService(db *gorm.DB, cfg *config.Config, tm *auth.TokenManager) *AuthService {
	return &AuthService{db: db, cfg: cfg, tm: tm}
}

type LoginResult struct {
	AccessToken  string     `json:"access_token"`
	RefreshToken string     `json:"refresh_token"`
	ExpiresIn    int        `json:"expires_in"`
	User         model.User `json:"user"`
}

func (s *AuthService) Login(username, password string) (*LoginResult, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return nil, errs.New(http.StatusBadRequest, "BAD_REQUEST", "用户名或密码不能为空")
	}
	var u model.User
	if err := s.db.Where("username = ?", username).First(&u).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.New(http.StatusUnauthorized, "INVALID_CREDENTIAL", "用户名或密码错误")
		}
		return nil, err
	}
	if u.Disabled {
		return nil, errs.New(http.StatusForbidden, "ACCOUNT_DISABLED", "账号已被禁用")
	}
	if !auth.VerifyPassword(u.PasswordHash, password) {
		return nil, errs.New(http.StatusUnauthorized, "INVALID_CREDENTIAL", "用户名或密码错误")
	}
	var deptID *int64
	if u.Role == model.RoleDept {
		deptID = u.DepartmentID
	}
	tokens, err := s.tm.Issue(u.ID, u.Role, deptID)
	if err != nil {
		return nil, err
	}
	return &LoginResult{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresIn:    tokens.ExpiresIn,
		User:         u,
	}, nil
}

func (s *AuthService) Refresh(refreshToken string) (*LoginResult, error) {
	claims, err := s.tm.Parse(refreshToken)
	if err != nil {
		return nil, errs.New(http.StatusUnauthorized, "UNAUTHORIZED", "refresh token 无效")
	}
	if claims.Subject != "refresh" {
		return nil, errs.New(http.StatusUnauthorized, "UNAUTHORIZED", "不是 refresh token")
	}
	var u model.User
	if err := s.db.First(&u, claims.UserID).Error; err != nil {
		return nil, errs.New(http.StatusUnauthorized, "UNAUTHORIZED", "用户已不存在")
	}
	if u.Disabled {
		return nil, errs.New(http.StatusForbidden, "ACCOUNT_DISABLED", "账号已被禁用")
	}
	var deptID2 *int64
	if u.Role == model.RoleDept {
		deptID2 = u.DepartmentID
	}
	tokens, err := s.tm.Issue(u.ID, u.Role, deptID2)
	if err != nil {
		return nil, err
	}
	return &LoginResult{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresIn:    tokens.ExpiresIn,
		User:         u,
	}, nil
}
