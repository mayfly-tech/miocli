package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"miocli/internal/config"
)

func TestPersistentProfileActivation(t *testing.T) {
	path := isolatedConfig(t)
	dir := filepath.Join(filepath.Dir(path), "profiles")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	profilePath := filepath.Join(dir, "home.yaml")
	if err := os.WriteFile(filepath.Join(dir, "profiles.yaml"), []byte("- name: home\n  url: https://example.test/config\n"), 0600); err != nil {
		t.Fatal(err)
	}
	requests := 0
	status := http.StatusNoContent
	var applied []byte
	var effectivePath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if r.Method != http.MethodPut || r.URL.Path != "/configs" || filepath.Dir(body["path"]) != dir || body["path"] == profilePath {
			t.Errorf("invalid reload request: %s %s %#v", r.Method, r.URL, body)
		}
		effectivePath = body["path"]
		var err error
		applied, err = os.ReadFile(effectivePath)
		if err != nil {
			t.Error(err)
		}
		info, err := os.Stat(effectivePath)
		if err != nil || (runtime.GOOS != "windows" && info.Mode().Perm() != 0600) {
			t.Errorf("effective profile must be private: %v", err)
		}
		w.WriteHeader(status)
	}))
	defer server.Close()
	active := filepath.Join(t.TempDir(), "config.yaml")
	cfg := config.Config{Server: server.URL, ActiveProfilePath: active}
	if err := config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	data := []byte("mode: rule\nexternal-controller: 0.0.0.0:9090\nsecret: subscription-secret\n")
	if err := os.WriteFile(profilePath, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := commandTest(t, defaultDependencies(), "--config", path, "profile", "use", "home"); err != nil {
		t.Fatal(err)
	}
	saved, err := os.ReadFile(active)
	if err != nil || string(saved) != string(applied) || requests != 1 {
		t.Fatalf("persistent profile = %q, %v, requests %d", saved, err, requests)
	}
	connection, err := config.ConnectionFromYAML(saved)
	if err != nil || connection.Server != server.URL || connection.Secret != "" {
		t.Fatalf("controller was not retained: %#v, %v", connection, err)
	}
	if original, err := os.ReadFile(profilePath); err != nil || string(original) != string(data) {
		t.Fatal("original subscription changed")
	}
	if _, err := os.Stat(effectivePath); !os.IsNotExist(err) {
		t.Fatal("temporary effective profile was not removed")
	}
	status = http.StatusBadRequest
	if _, err := commandTest(t, defaultDependencies(), "--config", path, "profile", "use", "home"); err == nil {
		t.Fatal("reload failure hidden")
	}
	if retained, err := os.ReadFile(effectivePath); err != nil || len(retained) == 0 {
		t.Fatalf("uncertain reload must retain effective file: %v", err)
	}
	if err := os.Remove(effectivePath); err != nil {
		t.Fatal(err)
	}
	if current, err := os.ReadFile(active); err != nil || string(current) != string(saved) {
		t.Fatal("failed reload replaced restart configuration")
	}
	if err := os.WriteFile(profilePath, []byte("mode: [\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := commandTest(t, defaultDependencies(), "--config", path, "profile", "use", "home"); err == nil {
		t.Fatal("invalid YAML accepted")
	}
	if requests != 2 {
		t.Fatal("invalid persistent profile reached controller")
	}
	if err := os.WriteFile(profilePath, data, 0600); err != nil {
		t.Fatal(err)
	}
	status = http.StatusNoContent
	cfg.ActiveProfilePath = t.TempDir()
	if err := config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := commandTest(t, defaultDependencies(), "--config", path, "profile", "use", "home"); err == nil || !strings.Contains(err.Error(), "profile reloaded") {
		t.Fatalf("restart configuration save failure hidden: %v", err)
	}
	if requests != 3 {
		t.Fatal("save failure did not happen after API reload")
	}
	cfg.ActiveProfilePath = path
	if err := config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	t.Chdir(filepath.Dir(path))
	if _, err := commandTest(t, defaultDependencies(), "--config", filepath.Base(path), "profile", "use", "home"); err == nil || !strings.Contains(err.Error(), "must not overwrite") {
		t.Fatalf("relative settings path could be overwritten: %v", err)
	}
	if requests != 3 {
		t.Fatal("unsafe settings overwrite reached controller")
	}
	cfg.ActiveProfilePath = profilePath
	if err := config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := commandTest(t, defaultDependencies(), "--config", path, "profile", "use", "home"); err == nil || !strings.Contains(err.Error(), "original subscription") {
		t.Fatalf("raw subscription could be overwritten: %v", err)
	}
	cfg.ActiveProfilePath = active
	if err := config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	// A subscription update may omit all controller fields.
	if err := os.WriteFile(profilePath, []byte("mode: direct\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := commandTest(t, defaultDependencies(), "--config", path, "profile", "use", "home"); err != nil {
		t.Fatalf("updated subscription without a controller failed: %v", err)
	}
	current, err := os.ReadFile(active)
	if err != nil || !strings.Contains(string(current), "mode: direct") {
		t.Fatalf("updated policy was not persisted: %q, %v", current, err)
	}
	connection, err = config.ConnectionFromYAML(current)
	if err != nil || connection.Server != server.URL {
		t.Fatalf("controller lost after subscription update: %#v, %v", connection, err)
	}
	if files, err := filepath.Glob(filepath.Join(dir, ".active-*.yaml")); err != nil || len(files) != 0 {
		t.Fatalf("temporary effective profiles leaked: %v, %v", files, err)
	}
}
