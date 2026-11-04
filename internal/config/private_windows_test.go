package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestSavedConfigHasPrivateWindowsACL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	cfg := Config{Server: DefaultServer, Secret: "test-secret", Core: Core{Mode: Connect}}
	for range 2 {
		if err := Save(path, cfg); err != nil {
			t.Fatal(err)
		}
		descriptor, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
		if err != nil {
			t.Fatal(err)
		}
		user, err := windows.GetCurrentProcessToken().GetTokenUser()
		if err != nil {
			t.Fatal(err)
		}
		control, _, err := descriptor.Control()
		if err != nil {
			t.Fatal(err)
		}
		expected, err := windows.SecurityDescriptorFromString("D:P(A;;FA;;;" + user.User.Sid.String() + ")")
		if err != nil {
			t.Fatal(err)
		}
		// Windows may serialize well-known account SIDs as aliases such as LA.
		wantACE := strings.TrimPrefix(expected.String(), "D:P")
		sddl := descriptor.String()
		if control&windows.SE_DACL_PROTECTED == 0 || strings.Count(sddl, "(") != 1 || !strings.Contains(sddl, wantACE) {
			t.Fatalf("private ACL = %s, want protected DACL with only %s", sddl, wantACE)
		}
		data, err := os.ReadFile(path)
		if err != nil || !strings.Contains(string(data), "test-secret") {
			t.Fatalf("saved file = %s, %v", data, err)
		}
	}
}
