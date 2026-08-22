package syncer

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/creamcroissant/mgpanel/internal/agent/api"
	"github.com/creamcroissant/mgpanel/internal/agent/config"
)

// Syncer 负责将配置渲染为核心可用文件，并触发热重载。
type Syncer struct {
	cfg config.CoreConfig
}

// New 创建同步器实例。
func New(cfg config.CoreConfig) *Syncer {
	return &Syncer{cfg: cfg}
}

// TemplateData 用于模板渲染的上下文数据。
type TemplateData struct {
	Config map[string]any
	Users  []api.User
}

// Sync 渲染模板、写入配置并在变更时触发重载。
func (s *Syncer) Sync(config map[string]any, users []api.User) error {
	// 1. 准备模板数据
	data := TemplateData{
		Config: config,
		Users:  users,
	}

	// 2. 读取模板内容
	tplContent, err := os.ReadFile(s.cfg.TemplatePath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}

	// 3. 解析模板，并注入 helper（json/indent）
	tmpl, err := template.New("config").Funcs(template.FuncMap{
		"json": func(v any) string {
			b, _ := json.Marshal(v)
			return string(b)
		},
		"indent": func(spaces int, v string) string {
			pad := strings.Repeat(" ", spaces)
			return strings.ReplaceAll(v, "\n", "\n"+pad)
		},
	}).Parse(string(tplContent))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	// 4. 确保输出目录存在
	if dir := filepath.Dir(s.cfg.OutputPath); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create output dir: %w", err)
		}
	}

	// 5. 先写入临时文件，避免半写状态
	tmpFile := s.cfg.OutputPath + ".tmp"
	f, err := os.OpenFile(tmpFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("create tmp file: %w", err)
	}

	if err := tmpl.Execute(f, data); err != nil {
		f.Close()
		os.Remove(tmpFile)
		return fmt.Errorf("execute template: %w", err)
	}

	if err := f.Close(); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("close tmp file: %w", err)
	}

	// 6. 与已有文件对比，避免无意义重载
	if changed, err := s.hasChanged(tmpFile, s.cfg.OutputPath); err != nil {
		// 出错时继续执行，宁可触发一次重载
		fmt.Printf("Warning: hash check failed: %v\n", err)
	} else if !changed {
		// 无变化直接清理临时文件
		os.Remove(tmpFile)
		return nil
	}

	// 7. 原子替换配置文件
	if err := os.Rename(tmpFile, s.cfg.OutputPath); err != nil {
		return fmt.Errorf("overwrite config: %w", err)
	}

	// 8. 执行重载命令
	if s.cfg.ReloadCmd != "" {
		parts := strings.Fields(s.cfg.ReloadCmd)
		if len(parts) > 0 {
			cmdCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			cmd := exec.CommandContext(cmdCtx, parts[0], parts[1:]...)
			if out, err := cmd.CombinedOutput(); err != nil {
				if cmdCtx.Err() != nil {
					return fmt.Errorf("reload cmd timeout after 30s: %w", cmdCtx.Err())
				}
				return fmt.Errorf("reload cmd failed: %s: %w", string(out), err)
			}
		}
	}

	return nil
}

// hasChanged 比较新旧文件内容哈希，判断是否需要替换。
func (s *Syncer) hasChanged(newFile, oldFile string) (bool, error) {
	if _, err := os.Stat(oldFile); os.IsNotExist(err) {
		return true, nil
	}

	h1, err := fileHash(newFile)
	if err != nil {
		return true, err
	}
	h2, err := fileHash(oldFile)
	if err != nil {
		return true, err
	}

	return h1 != h2, nil
}

// fileHash 计算文件内容的 MD5 哈希值。
func fileHash(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	hash := md5.Sum(content)
	return hex.EncodeToString(hash[:]), nil
}
