package desktop

import (
	"context"
	"errors"
	"io/fs"
	"os"

	"github.com/egoist/mygo"

	"ohmymc/internal/app"
)

type Bridge struct{ service *app.Service }

// Invoke is the only WebUI binding. Services and extensions validate every request.
func (b *Bridge) Invoke(ctx context.Context, method string, payload map[string]any) (any, error) {
	switch method {
	case "desktop.pickFolder":
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: mygo.CallerWindow(ctx), Title: "选择 .minecraft 或单个版本目录", Directory: true})
		if err != nil {
			return nil, err
		}
		if len(paths) == 0 {
			return "", nil
		}
		return paths[0], nil
	case "desktop.openFolder":
		id, _ := payload["id"].(string)
		v, err := b.service.Store.Get(id)
		if err != nil {
			return nil, err
		}
		if err = os.MkdirAll(v.GameDirectory, 0700); err != nil {
			return nil, err
		}
		return nil, mygo.Shell.OpenPath(v.GameDirectory)
	default:
		return b.service.Invoke(ctx, method, payload)
	}
}

// The page owns the complete workspace. The OS supplies only the standard
// window frame; there is no NativeUI view, embedded child WebView, or backdrop.
func workspaceOptions() mygo.WindowOptions {
	return mygo.WindowOptions{
		Title:           "OhMyMC · 你的世界，由你定义",
		URL:             "/",
		Width:           1280,
		Height:          850,
		MinWidth:        900,
		MinHeight:       640,
		StateKey:        "workspace",
		BackgroundColor: "#f5f6f2",
	}
}

func Run(service *app.Service, frontend fs.FS) error {
	mygo.App.SetName("OhMyMC")
	mygo.SetFrontend(frontend)
	mygo.Bind(&Bridge{service: service})
	var startupErr error
	mygo.App.WhenReady(func() {
		mygo.Theme.SetSource(mygo.ThemeLight)
		if !mygo.WebViewAvailable() {
			startupErr = errors.New("未检测到系统 WebView。Windows 请安装 Microsoft Edge WebView2 Runtime 后重新打开启动器。")
			_, _ = mygo.Dialog.Message(mygo.MessageOptions{
				Type:    mygo.MessageError,
				Message: "无法打开 OhMyMC 工作区",
				Detail:  startupErr.Error(),
				Buttons: []string{"退出"},
			})
			mygo.App.Quit()
			return
		}
		mygo.NewWindow(workspaceOptions())
	})
	if err := mygo.App.Run(); err != nil {
		return err
	}
	return startupErr
}
