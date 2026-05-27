package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID       int64  `json:"uid"`
	Role         string `json:"role"`
	DepartmentID *int64 `json:"dept,omitempty"`
	jwt.RegisteredClaims
}

type TokenManager struct {
	secret           []byte
	accessTokenTTL   time.Duration
	refreshTokenTTL  time.Duration
	revokedIDs       map[string]time.Time // 简易内存黑名单 (M3 可换成 redis)
}

func NewTokenManager(secret string, accessTTL, refreshTTL int) *TokenManager {
	return &TokenManager{
		secret:          []byte(secret),
		accessTokenTTL:  time.Duration(accessTTL) * time.Second,
		refreshTokenTTL: time.Duration(refreshTTL) * time.Second,
		revokedIDs:      make(map[string]time.Time),
	}
}

type IssuedTokens struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int
}

func (m *TokenManager) Issue(userID int64, role string, deptID *int64) (*IssuedTokens, error) {
	now := time.Now()
	jti := generateJTI()

	access := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		UserID: userID, Role: role, DepartmentID: deptID,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessTokenTTL)),
			Subject:   "access",
			ID:        jti,
		},
	})
	accessStr, err := access.SignedString(m.secret)
	if err != nil {
		return nil, err
	}

	refresh := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		UserID: userID, Role: role, DepartmentID: deptID,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.refreshTokenTTL)),
			Subject:   "refresh",
			ID:        jti + "r",
		},
	})
	refreshStr, err := refresh.SignedString(m.secret)
	if err != nil {
		return nil, err
	}

	return &IssuedTokens{
		AccessToken:  accessStr,
		RefreshToken: refreshStr,
		ExpiresIn:    int(m.accessTokenTTL.Seconds()),
	}, nil
}

func (m *TokenManager) Parse(tokenStr string) (*Claims, error) {
	parsed, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := parsed.Claims.(*Claims)
	if !ok || !parsed.Valid {
		return nil, errors.New("invalid token")
	}
	if _, revoked := m.revokedIDs[claims.ID]; revoked {
		return nil, errors.New("token revoked")
	}
	return claims, nil
}

func (m *TokenManager) Revoke(jti string, exp time.Time) {
	m.revokedIDs[jti] = exp
	m.gc()
}

// gc 清理过期黑名单项,避免无限增长
func (m *TokenManager) gc() {
	now := time.Now()
	for id, exp := range m.revokedIDs {
		if exp.Before(now) {
			delete(m.revokedIDs, id)
		}
	}
}

func generateJTI() string {
	return time.Now().Format("20060102150405.000000")
}
