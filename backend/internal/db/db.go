package db

import (
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/ksm/docflow/internal/config"
)

func Open(cfg config.DatabaseConfig) (*gorm.DB, error) {
	gormCfg := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	}
	gdb, err := gorm.Open(postgres.Open(cfg.DSN()), gormCfg)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	sdb, err := gdb.DB()
	if err != nil {
		return nil, err
	}
	sdb.SetMaxOpenConns(cfg.MaxOpenConns)
	sdb.SetMaxIdleConns(cfg.MaxIdleConns)
	sdb.SetConnMaxLifetime(time.Hour)
	if err := sdb.Ping(); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return gdb, nil
}
