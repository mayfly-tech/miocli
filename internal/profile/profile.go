package profile

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode"

	"gopkg.in/yaml.v3"

	"miocli/internal/config"
	"miocli/internal/core"
	"miocli/internal/mihomo"
)

type Entry struct {
	ID      string    `json:"id"`
	Name    string    `json:"name"`
	Kind    string    `json:"kind"`
	Source  string    `json:"source"`
	Content []byte    `json:"content"`
	Updated time.Time `json:"updated"`
}

type Index struct {
	Active  string  `json:"active"`
	Entries []Entry `json:"entries"`
}

type Client interface {
	mihomo.Client
	Reload(context.Context, string) error
}

type Service struct {
	mu     sync.Mutex
	path   string
	dir    string
	cfg    config.Config
	owned  bool
	client Client
}

func New(configPath string, cfg config.Config, owned bool, client Client) *Service {
	base := filepath.Dir(configPath)
	return &Service{path: filepath.Join(base, "profiles", "index.json"), dir: filepath.Join(base, "mihomo"), cfg: cfg, owned: owned && cfg.Core.Mode == config.Managed, client: client}
}

func (s *Service) List() (Index, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read()
}

func (s *Service) read() (Index, error) {
	info, err := os.Stat(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return Index{}, nil
	}
	if err != nil {
		return Index{}, fmt.Errorf("inspect profile index: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() > 64<<20 {
		return Index{}, errors.New("profile index must be a regular file below 64 MiB")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0 {
		return Index{}, errors.New("profile index contains private configuration and requires chmod 600")
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		return Index{}, fmt.Errorf("read profile index: %w", err)
	}
	var index Index
	if err := json.Unmarshal(data, &index); err != nil {
		return Index{}, errors.New("profile index is invalid; inspect the protected index before modifying profiles")
	}
	ids := make(map[string]bool)
	for _, entry := range index.Entries {
		if entry.ID == "" || ids[entry.ID] {
			return Index{}, errors.New("profile index contains invalid or duplicate IDs")
		}
		ids[entry.ID] = true
	}
	if index.Active != "" && !ids[index.Active] {
		return Index{}, errors.New("profile index references a missing active profile")
	}
	return index, nil
}

func encodeIndex(index Index) ([]byte, error) {
	data, err := json.Marshal(index)
	if err != nil {
		return nil, fmt.Errorf("encode profile index: %w", err)
	}
	if len(data) > 64<<20 {
		return nil, errors.New("profile storage exceeds 64 MiB")
	}
	return data, nil
}

func (s *Service) save(index Index) error {
	data, err := encodeIndex(index)
	if err != nil {
		return err
	}
	return config.WritePrivate(s.path, data)
}

func (s *Service) journalPath() string { return filepath.Join(s.dir, "profile-transaction.json") }

func (s *Service) Pending() (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := os.Stat(s.journalPath())
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

func (s *Service) writable() error {
	if !s.owned {
		return errors.New("完整档案写操作仅允许本次由 miocli 启动的托管实例")
	}
	_, err := os.Stat(s.journalPath())
	if err == nil {
		return errors.New("配置应用结果未确认：存在待恢复事务；退出并重新启动自有内核以恢复已保存配置")
	}
	if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect pending profile transaction: %w", err)
	}
	return nil
}

func fetch(ctx context.Context, kind, source string) ([]byte, error) {
	switch kind {
	case "https":
		data, err := core.DownloadConfig(ctx, source)
		if err != nil {
			return nil, errors.New("HTTPS 订阅下载失败；检查地址、网络及服务状态（地址已隐藏）")
		}
		return data, nil
	case "file":
		return core.ReadConfigFile(source)
	default:
		return nil, errors.New("来源类型必须为 https 或 file")
	}
}

func (s *Service) prepare(data []byte) ([]byte, error) {
	_, prepared, err := core.PrepareManagedConfig(data, s.cfg.Server, s.cfg.Secret)
	if err != nil {
		return nil, err
	}
	var fields map[string]any
	if err := yaml.Unmarshal(prepared, &fields); err != nil {
		return nil, errors.New("无效 Mihomo YAML")
	}
	if tun, ok := fields["tun"].(map[string]any); ok {
		if value, exists := tun["enable"]; exists && value != false {
			return nil, errors.New("托管档案不能启用 TUN")
		}
	}
	if value, exists := fields["listeners"]; exists && value != nil {
		if listeners, ok := value.([]any); !ok || len(listeners) != 0 {
			return nil, errors.New("托管档案不能启用自定义 listeners")
		}
	}
	return prepared, nil
}

func validName(name string) error {
	if strings.TrimSpace(name) == "" || len([]rune(name)) > 128 || strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return errors.New("档案名称须为 1–128 个字符，且不能包含换行或控制序列")
	}
	return nil
}

func normalizeSource(kind, source string) (string, error) {
	source = strings.TrimSpace(source)
	if kind == "file" {
		path, err := core.ExpandHome(source)
		if err != nil {
			return "", err
		}
		return filepath.Abs(path)
	}
	return source, nil
}

func (s *Service) Add(ctx context.Context, name, kind, source string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.writable(); err != nil {
		return err
	}
	if err := validName(name); err != nil {
		return err
	}
	source, err := normalizeSource(kind, source)
	if err != nil {
		return err
	}
	data, err := fetch(ctx, kind, source)
	if err != nil {
		return err
	}
	if err := s.validateInactive(ctx, data); err != nil {
		return err
	}
	index, err := s.read()
	if err != nil {
		return err
	}
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return fmt.Errorf("create profile ID: %w", err)
	}
	index.Entries = append(index.Entries, Entry{ID: hex.EncodeToString(id), Name: strings.TrimSpace(name), Kind: kind, Source: source, Content: data, Updated: time.Now().UTC()})
	return s.save(index)
}

func find(index Index, id string) (int, error) {
	for i, entry := range index.Entries {
		if entry.ID == id {
			return i, nil
		}
	}
	return 0, errors.New("档案不存在，请刷新后重试")
}

func (s *Service) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.writable(); err != nil {
		return err
	}
	index, err := s.read()
	if err != nil {
		return err
	}
	i, err := find(index, id)
	if err != nil {
		return err
	}
	if index.Active == id {
		return errors.New("不能删除启用档案；请先切换到另一个档案")
	}
	index.Entries = append(index.Entries[:i], index.Entries[i+1:]...)
	return s.save(index)
}

func (s *Service) Edit(ctx context.Context, id, name, kind, source string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.writable(); err != nil {
		return err
	}
	if err := validName(name); err != nil {
		return err
	}
	source, err := normalizeSource(kind, source)
	if err != nil {
		return err
	}
	index, err := s.read()
	if err != nil {
		return err
	}
	i, err := find(index, id)
	if err != nil {
		return err
	}
	entry := index.Entries[i]
	if entry.Kind != kind || entry.Source != source {
		data, err := fetch(ctx, kind, source)
		if err != nil {
			return err
		}
		if _, err := s.prepare(data); err != nil {
			return err
		}
		entry.Kind, entry.Source, entry.Content, entry.Updated = kind, source, data, time.Now().UTC()
	}
	entry.Name = strings.TrimSpace(name)
	index.Entries[i] = entry
	if index.Active == id {
		return s.apply(ctx, index, entry)
	}
	if err := s.validateInactive(ctx, entry.Content); err != nil {
		return err
	}
	return s.save(index)
}

func (s *Service) Refresh(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.writable(); err != nil {
		return err
	}
	index, err := s.read()
	if err != nil {
		return err
	}
	i, err := find(index, id)
	if err != nil {
		return err
	}
	entry := index.Entries[i]
	data, err := fetch(ctx, entry.Kind, entry.Source)
	if err != nil {
		return err
	}
	if _, err := s.prepare(data); err != nil {
		return err
	}
	entry.Content, entry.Updated = data, time.Now().UTC()
	index.Entries[i] = entry
	if index.Active == id {
		return s.apply(ctx, index, entry)
	}
	if err := s.validateInactive(ctx, entry.Content); err != nil {
		return err
	}
	return s.save(index)
}

func (s *Service) Apply(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.writable(); err != nil {
		return err
	}
	index, err := s.read()
	if err != nil {
		return err
	}
	i, err := find(index, id)
	if err != nil {
		return err
	}
	index.Active = id
	return s.apply(ctx, index, index.Entries[i])
}

func (s *Service) validateInactive(ctx context.Context, data []byte) error {
	candidate, err := s.prepare(data)
	if err != nil {
		return err
	}
	path, err := s.validate(ctx, candidate)
	if path != "" {
		defer os.Remove(path)
	}
	return err
}

func (s *Service) validate(ctx context.Context, candidate []byte) (string, error) {
	path := filepath.Join(s.dir, "profile-candidate.yaml")
	if err := config.WritePrivate(path, candidate); err != nil {
		return "", err
	}
	checkCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	logPath := filepath.Join(s.dir, "profile-check.log")
	if err := config.WritePrivate(logPath, nil); err != nil {
		return path, err
	}
	log, err := os.OpenFile(logPath, os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return path, fmt.Errorf("open profile validation log: %w", err)
	}
	cmd := exec.CommandContext(checkCtx, s.cfg.Core.Binary, "-t", "-d", s.dir, "-f", path)
	cmd.Stdout, cmd.Stderr = log, log
	runErr, closeErr := cmd.Run(), log.Close()
	if runErr != nil || closeErr != nil {
		return path, fmt.Errorf("档案验证失败；检查受保护日志 %s", logPath)
	}
	return path, nil
}

type transaction struct {
	Previous  []byte `json:"previous"`
	Candidate []byte `json:"candidate"`
	Next      Index  `json:"next"`
}

func (s *Service) apply(ctx context.Context, index Index, entry Entry) error {
	if s.client == nil {
		return errors.New("缺少托管内核配置客户端")
	}
	if _, err := encodeIndex(index); err != nil {
		return err
	}
	candidate, err := s.prepare(entry.Content)
	if err != nil {
		return err
	}
	currentPath := filepath.Join(s.dir, "config.yaml")
	current, err := core.ReadConfigFile(currentPath)
	if err != nil {
		return err
	}
	if err := sameListeners(current, candidate); err != nil {
		return err
	}
	candidatePath, err := s.validate(ctx, candidate)
	if candidatePath != "" {
		defer os.Remove(candidatePath)
	}
	if err != nil {
		return err
	}
	tx := transaction{Previous: current, Candidate: candidate, Next: index}
	journal, err := json.Marshal(tx)
	if err != nil {
		return err
	}
	if err := config.WritePrivate(s.journalPath(), journal); err != nil {
		return err
	}
	if err := s.client.Reload(ctx, candidatePath); err != nil {
		var apiErr *mihomo.APIError
		if errors.As(err, &apiErr) && apiErr.Status >= 400 && apiErr.Status < 500 && apiErr.Status != 408 {
			if removeErr := os.Remove(s.journalPath()); removeErr != nil {
				return fmt.Errorf("加载被拒绝，清理事务失败: %w", removeErr)
			}
			return errors.New("内核拒绝加载档案，原配置和启用记录保持不变")
		}
		return errors.New("配置加载结果未确认；未自动重试或重启，原磁盘配置已保留。退出后重新启动自有内核恢复")
	}
	if err := s.verify(ctx, candidate); err != nil {
		return err
	}
	if err := config.WritePrivate(currentPath, candidate); err != nil {
		return fmt.Errorf("运行配置已应用，但保存失败；事务已保留: %w", err)
	}
	if err := s.save(index); err != nil {
		restoreErr := config.WritePrivate(currentPath, current)
		return errors.Join(fmt.Errorf("运行配置已应用，但档案记录保存失败；事务已保留: %w", err), restoreErr)
	}
	if err := os.Remove(s.journalPath()); err != nil {
		return fmt.Errorf("配置已保存，但清理事务失败: %w", err)
	}
	return nil
}

func (s *Service) verify(ctx context.Context, candidate []byte) error {
	var expected struct {
		Mode   mihomo.Mode `yaml:"mode"`
		Groups []struct {
			Name string `yaml:"name"`
		} `yaml:"proxy-groups"`
	}
	if err := yaml.Unmarshal(candidate, &expected); err != nil {
		return errors.New("无法读取候选配置摘要")
	}
	mode, err := s.client.Mode(ctx)
	if err != nil {
		return errors.New("内核接受加载，但模式读取失败；应用结果未确认，事务已保留")
	}
	if expected.Mode == "" {
		expected.Mode = mihomo.ModeRule
	}
	if mode != expected.Mode {
		return errors.New("内核接受加载，但模式与候选不一致；事务已保留")
	}
	proxies, err := s.client.Proxies(ctx)
	if err != nil {
		return errors.New("内核接受加载，但代理读取失败；应用结果未确认，事务已保留")
	}
	for _, group := range expected.Groups {
		if _, ok := proxies.Items[group.Name]; !ok {
			return errors.New("内核接受加载，但候选策略组未确认；事务已保留")
		}
	}
	return nil
}

func sameListeners(current, candidate []byte) error {
	var a, b map[string]any
	if err := yaml.Unmarshal(current, &a); err != nil {
		return errors.New("当前运行配置无效")
	}
	if err := yaml.Unmarshal(candidate, &b); err != nil {
		return errors.New("候选配置无效")
	}
	for _, key := range []string{"external-controller", "secret", "external-controller-tls", "external-controller-unix", "external-controller-pipe", "external-doh-server", "port", "mixed-port", "socks-port", "redir-port", "tproxy-port", "allow-lan", "bind-address", "ipv6", "dns", "tun", "listeners", "authentication"} {
		if !reflect.DeepEqual(a[key], b[key]) {
			return fmt.Errorf("档案改变 %s；本轮热加载不修改监听/网络设置，请通过 setup 配置", key)
		}
	}
	return nil
}

// Register records the configuration just validated and installed by setup.
func (s *Service) Register(name, kind, source string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := validName(name); err != nil {
		return err
	}
	if _, err := s.prepare(data); err != nil {
		return err
	}
	index, err := s.read()
	if err != nil {
		return err
	}
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return err
	}
	entry := Entry{ID: hex.EncodeToString(id), Name: name, Kind: kind, Source: source, Content: data, Updated: time.Now().UTC()}
	index.Entries = append(index.Entries, entry)
	index.Active = entry.ID
	return s.save(index)
}

// RecoverOwnedStartup runs only after a fresh owned process loaded the saved config.
func (s *Service) RecoverOwnedStartup(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.owned {
		return errors.New("不能恢复非自有内核的档案事务")
	}
	data, err := os.ReadFile(s.journalPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var tx transaction
	if err := json.Unmarshal(data, &tx); err != nil {
		return errors.New("待恢复档案事务无效，不能自动清理")
	}
	current, err := core.ReadConfigFile(filepath.Join(s.dir, "config.yaml"))
	if err != nil {
		return err
	}
	if err := s.verify(ctx, current); err != nil {
		return err
	}
	if string(current) == string(tx.Candidate) {
		if err := s.save(tx.Next); err != nil {
			return err
		}
	} else if string(current) != string(tx.Previous) {
		return errors.New("保存配置与待恢复事务不匹配，不能自动清理")
	}
	return os.Remove(s.journalPath())
}
