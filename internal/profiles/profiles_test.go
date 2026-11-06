package profiles

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testManager(t *testing.T, handler http.HandlerFunc) (*Manager, *httptest.Server) {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	manager := New(filepath.Join(t.TempDir(), "config.yaml"))
	manager.client = server.Client()
	manager.client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return manager, server
}

func TestAddUpdateListAndRemoveProfile(t *testing.T) {
	responses := []string{
		"mixed-port: 7890\nmode: rule\n",
		"mixed-port: 7891\nmode: global\n",
	}
	manager, server := testManager(t, func(w http.ResponseWriter, _ *http.Request) {
		if len(responses) == 0 {
			t.Error("unexpected profile request")
			return
		}
		_, _ = w.Write([]byte(responses[0]))
		responses = responses[1:]
	})
	if err := manager.Add(context.Background(), "home", server.URL+"/sub?token=private"); err != nil {
		t.Fatal(err)
	}
	entries, err := manager.List()
	if err != nil || len(entries) != 1 || entries[0].Name != "home" || entries[0].URL == "" {
		t.Fatalf("List() = %#v, %v", entries, err)
	}
	path, err := manager.ConfigPath("home")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "mixed-port: 7890\nmode: rule\n" {
		t.Fatalf("profile data = %q, %v", data, err)
	}
	if err := manager.Add(context.Background(), "home", server.URL); err == nil {
		t.Fatal("Add accepted duplicate profile")
	}
	if err := manager.Update(context.Background(), "home"); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(path)
	if err != nil || string(data) != "mixed-port: 7891\nmode: global\n" {
		t.Fatalf("updated data = %q, %v", data, err)
	}
	if err := manager.Remove("home"); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.List(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("removed profile still exists: %v", err)
	}
}

func TestAddRejectsUnsafeNamesAndSources(t *testing.T) {
	manager := New(filepath.Join(t.TempDir(), "config.yaml"))
	for _, name := range []string{"", "../escape", "-profile", "a/b"} {
		if err := manager.Add(context.Background(), name, "https://example.test/config.yaml"); err == nil {
			t.Errorf("Add accepted invalid name %q", name)
		}
	}
	for _, source := range []string{"http://example.test/config.yaml", "https://user:pass@example.test/config.yaml", "https://example.test/config.yaml#fragment", "not a URL"} {
		if err := manager.Add(context.Background(), "profile", source); err == nil {
			t.Errorf("Add accepted invalid source %q", source)
		}
	}
}

func TestDownloadRejectsRedirectsAndInvalidYAML(t *testing.T) {
	t.Run("redirect", func(t *testing.T) {
		manager, server := testManager(t, func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "https://example.test/config.yaml", http.StatusFound)
		})
		err := manager.Add(context.Background(), "profile", server.URL)
		if err == nil || !strings.Contains(err.Error(), "HTTP 302") {
			t.Fatalf("Add error = %v, want redirect status", err)
		}
	})
	for _, body := range []string{
		"- not-a-config\n",
		"mixed-port: 7890\n---\nmode: rule\n",
		"mode: rule\nmode: global\n",
		"",
	} {
		t.Run(body, func(t *testing.T) {
			manager, server := testManager(t, func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(body))
			})
			if err := manager.Add(context.Background(), "profile", server.URL); err == nil {
				t.Fatal("Add accepted invalid profile")
			}
			entries, err := manager.List()
			if err != nil || len(entries) != 0 {
				t.Fatalf("invalid profile was persisted: %#v, %v", entries, err)
			}
		})
	}
}

func TestProfileFilesArePrivate(t *testing.T) {
	if os.PathSeparator == '\\' {
		t.Skip("Unix permission bits are not portable to Windows")
	}
	manager, server := testManager(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("mode: rule\n"))
	})
	if err := manager.Add(context.Background(), "profile", server.URL); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(manager.dir, "profiles.yaml"), manager.profilePath("profile")} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0600 {
			t.Errorf("%s permissions = %04o", filepath.Base(path), info.Mode().Perm())
		}
	}
}

func TestRegistryNameCannotBeUsedAsProfile(t *testing.T) {
	manager, server := testManager(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("mode: rule\n"))
	})
	for _, name := range []string{"profiles", "Profiles", "PROFILES"} {
		if err := manager.Add(context.Background(), name, server.URL); err == nil {
			t.Errorf("profile name %q would overwrite the registry", name)
		}
	}
	entries, err := manager.List()
	if err != nil || len(entries) != 0 {
		t.Fatalf("registry changed: %v, %v", entries, err)
	}
}
