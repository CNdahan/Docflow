package service

import (
	"errors"

	"github.com/rs/zerolog/log"
	"gorm.io/gorm"

	"github.com/ksm/docflow/internal/auth"
	"github.com/ksm/docflow/internal/config"
	"github.com/ksm/docflow/internal/model"
)

// EnsureInitialSuper 在首次启动时检查是否已有 super 账号,若无则按配置创建。
// 注意:此机制是兜底,推荐用 cmd/admin CLI 显式创建账号。
func EnsureInitialSuper(db *gorm.DB, cfg *config.Config) error {
	if !cfg.InitialSuper.Enabled {
		return nil
	}
	if cfg.InitialSuper.Username == "" || cfg.InitialSuper.Password == "" {
		log.Warn().Msg("initial_super 配置不完整,跳过")
		return nil
	}
	var count int64
	if err := db.Model(&model.User{}).Where("role = ?", model.RoleSuper).
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	hash, err := auth.HashPassword(cfg.InitialSuper.Password, cfg.Auth.BcryptCost)
	if err != nil {
		return err
	}
	u := &model.User{
		Username:     cfg.InitialSuper.Username,
		PasswordHash: hash,
		Role:         model.RoleSuper,
		RealName:     cfg.InitialSuper.RealName,
	}
	if err := db.Create(u).Error; err != nil {
		// 并发场景下可能已被另一进程创建
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil
		}
		return err
	}
	log.Info().Str("username", u.Username).Msg("已创建初始 super 账号")
	return nil
}
