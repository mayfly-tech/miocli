package profile

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
	"miocli/internal/config"
	"miocli/internal/core"
	"miocli/internal/mihomo"
)

func TestMain(m *testing.M) {
	if os.Getenv("MIOCLI_PROFILE_CHECKER") == "1" {
		for i, arg := range os.Args {
			if arg == "-f" && i+1 < len(os.Args) {
				data, err := os.ReadFile(os.Args[i+1])
				if err != nil || strings.Contains(string(data), "invalid-core") {
					os.Exit(2)
				}
				os.Exit(0)
			}
		}
		os.Exit(2)
	}
	os.Exit(m.Run())
}

type testClient struct {
	err         error
	readErr     error
	calls       int
	mode        mihomo.Mode
	proxies     mihomo.Proxies
	afterReload func()
}

func (c *testClient) Version(context.Context) (mihomo.Version, error) {
	return mihomo.Version{Version: "test"}, nil
}
func (c *testClient) Proxies(context.Context) (mihomo.Proxies, error)         { return c.proxies, c.readErr }
func (c *testClient) Mode(context.Context) (mihomo.Mode, error)               { return c.mode, c.readErr }
func (c *testClient) Select(context.Context, string, string) error            { return nil }
func (c *testClient) SetMode(context.Context, mihomo.Mode) error              { return nil }
func (c *testClient) Delay(context.Context, string, string, int) (int, error) { return 1, nil }
func (c *testClient) Reload(_ context.Context, path string) error {
	c.calls++
	if c.err != nil {
		return c.err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var cfg struct {
		Mode   mihomo.Mode `yaml:"mode"`
		Groups []struct {
			Name string `yaml:"name"`
		} `yaml:"proxy-groups"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return err
	}
	c.mode = cfg.Mode
	if c.mode == "" {
		c.mode = mihomo.ModeRule
	}
	c.proxies = mihomo.Proxies{Items: map[string]mihomo.Proxy{}}
	for _, group := range cfg.Groups {
		c.proxies.Items[group.Name] = mihomo.Proxy{Name: group.Name, Type: "Selector"}
	}
	if c.afterReload != nil {
		c.afterReload()
	}
	return nil
}

func fixture(t *testing.T) (*Service, *testClient, string) {
	t.Helper()
	t.Setenv("MIOCLI_PROFILE_CHECKER", "1")
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	cfg := config.Config{Server: "http://127.0.0.1:19090", Secret: "private", Core: config.Core{Mode: config.Managed, Binary: binary}}
	client := &testClient{mode: mihomo.ModeRule, proxies: mihomo.Proxies{Items: map[string]mihomo.Proxy{}}}
	s := New(filepath.Join(dir, "config.yaml"), cfg, true, client)
	source := filepath.Join(dir, "source.yaml")
	data := []byte("mode: rule\nproxies: []\nproxy-groups: []\nrules: ['MATCH,DIRECT']\n")
	if err := config.WritePrivate(source, data); err != nil {
		t.Fatal(err)
	}
	_, prepared, err := core.PrepareManagedConfig(data, cfg.Server, cfg.Secret)
	if err != nil {
		t.Fatal(err)
	}
	if err := config.WritePrivate(filepath.Join(s.dir, "config.yaml"), prepared); err != nil {
		t.Fatal(err)
	}
	return s, client, source
}

func TestStoreCRUDAndSourceReadonly(t *testing.T) {
	s, _, source := fixture(t)
	before, _ := os.ReadFile(source)
	ctx := context.Background()
	if err := s.Add(ctx, "One", "file", source); err != nil {
		t.Fatal(err)
	}
	index, err := s.List()
	if err != nil || len(index.Entries) != 1 || index.Entries[0].ID == "" {
		t.Fatalf("%#v %v", index, err)
	}
	id := index.Entries[0].ID
	if err := s.Edit(ctx, id, "Renamed", "file", source); err != nil {
		t.Fatal(err)
	}
	if err := s.Refresh(ctx, id); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(source)
	if string(before) != string(after) {
		t.Fatal("source file modified")
	}
	info, err := os.Stat(s.path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatal("profile index is not private")
	}
	if err := s.Delete(id); err != nil {
		t.Fatal(err)
	}
	index, _ = s.List()
	if len(index.Entries) != 0 {
		t.Fatal("profile not deleted")
	}
}

func TestFailedEditAndDownloadPreserveIndex(t *testing.T) {
	s, _, source := fixture(t)
	ctx := context.Background()
	if err := s.Add(ctx, "One", "file", source); err != nil {
		t.Fatal(err)
	}
	index, _ := s.List()
	before, _ := os.ReadFile(s.path)
	if err := s.Edit(ctx, index.Entries[0].ID, "New", "file", source+".missing"); err == nil {
		t.Fatal("missing source accepted")
	}
	if err := s.Add(ctx, "Bad", "https", "http://example.invalid/?token=do-not-show"); err == nil || strings.Contains(err.Error(), "do-not-show") {
		t.Fatal("download error missing or leaked URL")
	}
	after, _ := os.ReadFile(s.path)
	if string(before) != string(after) {
		t.Fatal("failed source edit modified index")
	}
}

func TestApplySuccessAndActiveDeletion(t *testing.T) {
	s, client, source := fixture(t)
	ctx := context.Background()
	if err := s.Add(ctx, "One", "file", source); err != nil {
		t.Fatal(err)
	}
	index, _ := s.List()
	id := index.Entries[0].ID
	if err := s.Apply(ctx, id); err != nil {
		t.Fatal(err)
	}
	index, _ = s.List()
	if index.Active != id || client.calls != 1 {
		t.Fatalf("active=%s calls=%d", index.Active, client.calls)
	}
	if err := s.Delete(id); err == nil {
		t.Fatal("active profile deleted")
	}
	if pending, err := s.Pending(); err != nil || pending {
		t.Fatalf("pending=%t %v", pending, err)
	}
}

func TestApplyFailureAndUnknownState(t *testing.T) {
	for _, definite := range []bool{false, true} {
		t.Run(map[bool]string{true: "rejected", false: "unknown"}[definite], func(t *testing.T) {
			s, client, source := fixture(t)
			if err := s.Add(context.Background(), "One", "file", source); err != nil {
				t.Fatal(err)
			}
			index, _ := s.List()
			before, _ := os.ReadFile(filepath.Join(s.dir, "config.yaml"))
			client.err = context.DeadlineExceeded
			if definite {
				client.err = &mihomo.APIError{Kind: mihomo.ErrorAPI, Status: 400}
			}
			if err := s.Apply(context.Background(), index.Entries[0].ID); err == nil {
				t.Fatal("failed apply succeeded")
			}
			after, _ := os.ReadFile(filepath.Join(s.dir, "config.yaml"))
			if string(before) != string(after) || client.calls != 1 {
				t.Fatal("failure changed disk or replayed request")
			}
			pending, _ := s.Pending()
			if pending == definite {
				t.Fatal("incorrect transaction state")
			}
			if !definite {
				if err := s.Delete(index.Entries[0].ID); err == nil {
					t.Fatal("pending transaction allowed writes")
				}
				client.err = nil
				if err := s.RecoverOwnedStartup(context.Background()); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestOwnershipValidationAndReadbackFailure(t *testing.T) {
	s, client, source := fixture(t)
	ctx := context.Background()
	if err := s.Add(ctx, "One", "file", source); err != nil {
		t.Fatal(err)
	}
	index, _ := s.List()
	s.owned = false
	if err := s.Apply(ctx, index.Entries[0].ID); err == nil || client.calls != 0 {
		t.Fatal("unowned core applied config")
	}
	if err := s.Add(ctx, "Other", "file", source); err == nil {
		t.Fatal("unowned mutation accepted")
	}
	s.owned = true
	data, _ := os.ReadFile(source)
	if err := os.WriteFile(source, append(data, []byte("mixed-port: 19999\n")...), 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.Refresh(ctx, index.Entries[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Apply(ctx, index.Entries[0].ID); err == nil || client.calls != 0 {
		t.Fatal("listener changes were applied")
	}
	if err := os.WriteFile(source, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.Refresh(ctx, index.Entries[0].ID); err != nil {
		t.Fatal(err)
	}
	client.readErr = errors.New("read failed")
	if err := s.Apply(ctx, index.Entries[0].ID); err == nil {
		t.Fatal("readback failure reported success")
	}
	if pending, _ := s.Pending(); !pending {
		t.Fatal("readback failure lost transaction")
	}
}

func TestInvalidConfigAndIndex(t *testing.T) {
	s, client, source := fixture(t)
	if err := os.WriteFile(source, []byte("tun:\n  enable: true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.Add(context.Background(), "Bad", "file", source); err == nil {
		t.Fatal("TUN enabled")
	}
	if client.calls != 0 {
		t.Fatal("invalid configuration reached core")
	}
	if err := config.WritePrivate(s.path, []byte(`{"active":"missing","entries":[]}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.List(); err == nil {
		t.Fatal("missing active ID accepted")
	}
}

func TestIsolatedProfileApplication(t *testing.T) {
	binary := os.Getenv("MIOCLI_TEST_BINARY")
	if binary == "" {
		t.Skip("set MIOCLI_TEST_BINARY to test profile application against an isolated core")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	cfg := config.Config{Server: "http://" + address, Secret: "profile-isolated", Core: config.Core{Mode: config.Managed, Binary: binary}}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	raw := []byte("log-level: silent\nmode: rule\nproxies: []\nproxy-groups: []\nrules: ['MATCH,DIRECT']\n")
	_, prepared, err := core.PrepareManagedConfig(raw, cfg.Server, cfg.Secret)
	if err != nil {
		t.Fatal(err)
	}
	paths, err := core.PathsFor(filepath.Join(dir, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := core.SetupManaged(ctx, cfg, paths, prepared); err != nil {
		t.Fatal(err)
	}
	manager, err := core.EnsureRunning(ctx, cfg, paths)
	if err != nil || manager == nil {
		t.Fatalf("owned startup = %v, %v", manager, err)
	}
	t.Cleanup(func() {
		if err := manager.Close(); err != nil {
			t.Error(err)
		}
	})
	client, err := mihomo.NewHTTPClient(cfg.Server, cfg.Secret)
	if err != nil {
		t.Fatal(err)
	}
	s := New(paths.AppConfig, cfg, true, client)
	source := filepath.Join(dir, "subscription.yaml")
	if err := config.WritePrivate(source, raw); err != nil {
		t.Fatal(err)
	}
	if err := s.Register("Initial", "file", source, raw); err != nil {
		t.Fatal(err)
	}
	index, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	raw = []byte(strings.Replace(string(raw), "mode: rule", "mode: direct", 1))
	if err := config.WritePrivate(source, raw); err != nil {
		t.Fatal(err)
	}
	if err := s.Refresh(ctx, index.Active); err != nil {
		t.Fatal(err)
	}
	if mode, err := client.Mode(ctx); err != nil || mode != mihomo.ModeDirect {
		t.Fatalf("mode=%q %v", mode, err)
	}
	if pending, err := s.Pending(); err != nil || pending {
		t.Fatalf("pending=%t %v", pending, err)
	}
}

func TestIndexCommitFailureKeepsRecoveryJournalAndOriginalDisk(t *testing.T) {
	s, client, source := fixture(t)
	ctx := context.Background()
	if err := s.Add(ctx, "One", "file", source); err != nil {
		t.Fatal(err)
	}
	index, _ := s.List()
	before, _ := os.ReadFile(filepath.Join(s.dir, "config.yaml"))
	indexPath := s.path
	client.afterReload = func() { s.path = filepath.Join(source, "not-a-directory", "index.json") }
	if err := s.Apply(ctx, index.Entries[0].ID); err == nil {
		t.Fatal("index failure reported success")
	}
	s.path = indexPath
	current, _ := os.ReadFile(filepath.Join(s.dir, "config.yaml"))
	if string(before) != string(current) {
		t.Fatal("index failure did not restore saved config")
	}
	if pending, _ := s.Pending(); !pending {
		t.Fatal("commit failure lost recovery journal")
	}
	saved, _ := s.List()
	if saved.Active != "" {
		t.Fatal("failed commit changed active profile")
	}
}

func TestInactiveProfilesRequireCoreValidation(t *testing.T) {
	s, client, source := fixture(t)
	if err := os.WriteFile(source, []byte("invalid-core: true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.Add(context.Background(), "Invalid", "file", source); err == nil {
		t.Fatal("invalid Mihomo config was saved")
	}
	index, err := s.List()
	if err != nil || len(index.Entries) != 0 || client.calls != 0 {
		t.Fatal("validation failure changed state")
	}
}
