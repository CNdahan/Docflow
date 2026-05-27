package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"

	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/ksm/docflow/internal/api"
	"github.com/ksm/docflow/internal/auth"
	"github.com/ksm/docflow/internal/config"
	"github.com/ksm/docflow/internal/db"
	"github.com/ksm/docflow/internal/service"
	"github.com/ksm/docflow/internal/storage"
)

func main() {
	var (
		configPath     string
		migrate        bool
		migrationsPath string
	)
	flag.StringVar(&configPath, "config", "config.yaml", "配置文件路径")
	flag.BoolVar(&migrate, "migrate", false, "启动时自动跑迁移")
	flag.StringVar(&migrationsPath, "migrations", "migrations", "迁移 SQL 目录")
	flag.Parse()

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "加载配置失败:", err)
		os.Exit(1)
	}

	setupLogger(cfg.Log)

	gdb, err := db.Open(cfg.Database)
	if err != nil {
		log.Fatal().Err(err).Msg("连接数据库失败")
	}

	if migrate {
		if err := runMigrations(cfg, migrationsPath); err != nil {
			log.Fatal().Err(err).Msg("数据库迁移失败")
		}
		log.Info().Msg("数据库迁移完成")
	}

	store, err := storage.NewLocal(cfg.Storage.Root)
	if err != nil {
		log.Fatal().Err(err).Msg("初始化本地存储失败")
	}

	tm := auth.NewTokenManager(cfg.Auth.JWTSecret, cfg.Auth.AccessTokenTTL, cfg.Auth.RefreshTokenTTL)

	authSvc := service.NewAuthService(gdb, cfg, tm)
	deptSvc := service.NewDepartmentService(gdb)
	userSvc := service.NewUserService(gdb, cfg)
	attSvc := service.NewAttachmentService(gdb, cfg, store)
	docSvc := service.NewDocumentService(gdb, cfg, store)
	subSvc := service.NewSubmissionService(gdb, cfg, store)
	statsSvc := service.NewStatsService(gdb)

	handlers := api.Handlers{
		Auth:       api.NewAuthHandler(authSvc, tm),
		Department: api.NewDepartmentHandler(deptSvc),
		User:       api.NewUserHandler(userSvc),
		Attachment: api.NewAttachmentHandler(attSvc),
		Document:   api.NewDocumentHandler(docSvc),
		Submission: api.NewSubmissionHandler(subSvc),
		Stats:      api.NewStatsHandler(statsSvc),
	}

	if cfg.InitialSuper.Enabled {
		if err := service.EnsureInitialSuper(gdb, cfg); err != nil {
			log.Error().Err(err).Msg("初始 super 创建失败")
		}
	}

	r := api.BuildRouter(cfg, tm, handlers)
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Info().Str("addr", addr).Msg("DocFlow server starting")
	if err := r.Run(addr); err != nil {
		log.Fatal().Err(err).Msg("server stopped with error")
	}
}

func runMigrations(cfg *config.Config, dir string) error {
	conn, err := sql.Open("postgres", cfg.Database.DSN())
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.Up(conn, dir)
}

func setupLogger(c config.LogConfig) {
	level, err := zerolog.ParseLevel(c.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)
	var w *os.File = os.Stdout
	if c.Output != "" {
		f, err := os.OpenFile(c.Output, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
		if err == nil {
			w = f
		}
	}
	if c.Format == "console" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: w})
	} else {
		log.Logger = zerolog.New(w).With().Timestamp().Logger()
	}
}
