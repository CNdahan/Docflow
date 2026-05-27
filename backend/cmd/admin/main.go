package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"syscall"

	"golang.org/x/term"
	"gorm.io/gorm"

	"github.com/ksm/docflow/internal/auth"
	"github.com/ksm/docflow/internal/config"
	"github.com/ksm/docflow/internal/db"
	"github.com/ksm/docflow/internal/model"
)

// 用法:
//   docflow-admin -config config.yaml create-super -username admin -realname 管理员
//   (密码通过 stdin 安全输入,或 -password 显式指定 — 后者仅适合自动化场景)
func main() {
	configPath := flag.String("config", "config.yaml", "配置文件路径")
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		usage()
		os.Exit(1)
	}
	subcmd := args[0]
	subArgs := args[1:]

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "加载配置失败:", err)
		os.Exit(1)
	}
	gdb, err := db.Open(cfg.Database)
	if err != nil {
		fmt.Fprintln(os.Stderr, "连接数据库失败:", err)
		os.Exit(1)
	}

	switch subcmd {
	case "create-super":
		if err := createSuper(gdb, cfg, subArgs); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "用法: docflow-admin -config <path> <subcommand> [...]")
	fmt.Fprintln(os.Stderr, "子命令:")
	fmt.Fprintln(os.Stderr, "  create-super -username <name> [-realname <name>] [-password <pw>]")
}

func createSuper(gdb *gorm.DB, cfg *config.Config, args []string) error {
	fs := flag.NewFlagSet("create-super", flag.ExitOnError)
	username := fs.String("username", "", "用户名")
	realname := fs.String("realname", "超级管理员", "真实姓名")
	password := fs.String("password", "", "密码 (留空走交互式输入)")
	_ = fs.Parse(args)

	if *username == "" {
		return errors.New("用户名不能为空")
	}

	var pw string
	if *password != "" {
		pw = *password
	} else {
		var err error
		pw, err = promptPassword()
		if err != nil {
			return err
		}
	}
	if len(pw) < 8 {
		return errors.New("密码至少 8 位")
	}

	hash, err := auth.HashPassword(pw, cfg.Auth.BcryptCost)
	if err != nil {
		return err
	}
	u := &model.User{
		Username:     strings.TrimSpace(*username),
		PasswordHash: hash,
		Role:         model.RoleSuper,
		RealName:     *realname,
	}
	if err := gdb.Create(u).Error; err != nil {
		return fmt.Errorf("创建用户失败: %w", err)
	}
	fmt.Printf("已创建顶级用户: id=%d username=%s\n", u.ID, u.Username)
	return nil
}

func promptPassword() (string, error) {
	fmt.Print("请输入密码: ")
	pw1, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		// 非交互式终端,尝试用 stdin 行模式 (适合 docker run 等场景)
		reader := bufio.NewReader(os.Stdin)
		line, _ := reader.ReadString('\n')
		return strings.TrimRight(line, "\r\n"), nil
	}
	fmt.Print("再次输入密码: ")
	pw2, _ := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if string(pw1) != string(pw2) {
		return "", errors.New("两次密码不一致")
	}
	return string(pw1), nil
}
