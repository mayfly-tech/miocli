package theme

import (
	"strings"
	"testing"
)

func TestNoColorPolicy(t *testing.T) {
	for _, tc := range []struct {
		flag      bool
		env, term string
		want      bool
	}{
		{false, "", "xterm", false}, {true, "", "xterm", true},
		{false, "1", "xterm", true}, {false, "", "dumb", true},
		{false, "0", "xterm", true},
	} {
		t.Setenv("NO_COLOR", tc.env)
		t.Setenv("TERM", tc.term)
		if got := NoColor(tc.flag); got != tc.want {
			t.Fatalf("%+v: %t", tc, got)
		}
	}
}

func TestNoColorInputHasNoANSIStyles(t *testing.T) {
	input := Input(true)
	input.Placeholder = "Enter a name"
	input.Focus()
	for _, value := range []string{"", "hello", "中文👩‍💻"} {
		input.SetValue(value)
		if strings.Contains(input.View(), "\x1b") {
			t.Fatalf("no-color input has ANSI: %q", input.View())
		}
	}
}

func TestSafeTextRemovesTerminalAndDirectionControls(t *testing.T) {
	if got := SafeText("中文👩‍💻\x1b\r\n\u202e\u2066"); strings.ContainsAny(got, "\x1b\r\n\u202e\u2066") || !strings.Contains(got, "中文👩‍💻") {
		t.Fatalf("unsafe or corrupted text: %q", got)
	}
}
