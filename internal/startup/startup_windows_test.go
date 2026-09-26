package startup

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows/registry"
)

func fixture(t *testing.T) registration {
	t.Helper()
	path := fmt.Sprintf(`Software\PasteWin-StartupTest-%d`, time.Now().UnixNano())
	t.Cleanup(func() { _ = registry.DeleteKey(registry.CURRENT_USER, path) })
	return registration{path, `C:\应用程序\Paste Win\pastewin.exe`}
}

func TestRegistrationLifecycle(t *testing.T) {
	r := fixture(t)
	assertEnabled := func(want bool) {
		t.Helper()
		got, err := r.enabled()
		if err != nil || got != want {
			t.Fatalf("enabled = %v, %v; want %v", got, err, want)
		}
	}
	assertEnabled(false)
	if err := r.setEnabled(false); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := r.setEnabled(true); err != nil {
			t.Fatal(err)
		}
		assertEnabled(true)
	}
	key, err := registry.OpenKey(registry.CURRENT_USER, r.keyPath, registry.ALL_ACCESS)
	if err != nil {
		t.Fatal(err)
	}
	defer key.Close()
	command, _, err := key.GetStringValue(valueName)
	if err != nil || command != `"C:\应用程序\Paste Win\pastewin.exe" --autostart` {
		t.Fatalf("invalid startup command %q: %v", command, err)
	}
	if err := key.SetStringValue("unrelated", "keep"); err != nil {
		t.Fatal(err)
	}
	// Reading again uses registry state, including external edits and moved EXEs.
	moved := registration{r.keyPath, `C:\new path\pastewin.exe`}
	if got, err := moved.enabled(); err != nil || got {
		t.Fatalf("moved executable reported enabled: %v, %v", got, err)
	}
	if err := moved.setEnabled(true); err != nil {
		t.Fatal(err)
	}
	assertEnabled(false)
	for i := 0; i < 2; i++ {
		if err := r.setEnabled(false); err != nil {
			t.Fatal(err)
		}
		assertEnabled(false)
	}
	if value, _, err := key.GetStringValue("unrelated"); err != nil || value != "keep" {
		t.Fatalf("unrelated value modified: %q, %v", value, err)
	}
}

func TestRejectedWritePreservesRegistration(t *testing.T) {
	r := fixture(t)
	if err := r.setEnabled(true); err != nil {
		t.Fatal(err)
	}
	long := registration{r.keyPath, `C:\` + strings.Repeat("😀", 130) + `\pastewin.exe`}
	if err := long.setEnabled(true); err == nil {
		t.Fatal("accepted oversized Run command")
	}
	if enabled, err := r.enabled(); err != nil || !enabled {
		t.Fatalf("old setting lost: %v, %v", enabled, err)
	}
	key, err := registry.OpenKey(registry.CURRENT_USER, r.keyPath, registry.SET_VALUE)
	if err != nil {
		t.Fatal(err)
	}
	defer key.Close()
	if err := key.SetDWordValue(valueName, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := r.enabled(); err == nil {
		t.Fatal("invalid value type was silently treated as disabled")
	}
}
