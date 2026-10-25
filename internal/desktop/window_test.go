package desktop

import "testing"

func TestWorkspaceUsesOnlyWebUI(t *testing.T) {
	opts := workspaceOptions()
	if opts.URL != "/" || opts.Content != nil {
		t.Fatal("workspace must load the embedded WebUI directly, without NativeUI content")
	}
	if opts.TitleBarStyle != "" || opts.Frameless || opts.HideWindowButtons || opts.TitleBarHeight != 0 {
		t.Fatal("workspace must keep the standard OS window frame")
	}
	if opts.Vibrancy != "" || opts.Transparent {
		t.Fatal("workspace must not configure a native glass backdrop")
	}
	if opts.BackgroundColor == "" || opts.StateKey == "" {
		t.Fatal("workspace must retain its loading background and window state")
	}
}
