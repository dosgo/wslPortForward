package main

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"log"
	"os"
	"runtime"
	"time"

	"cogentcore.org/core/colors"
	"cogentcore.org/core/core"
	"cogentcore.org/core/system"

	"cogentcore.org/core/events"
	"cogentcore.org/core/paint"
	"cogentcore.org/core/styles"
	"cogentcore.org/core/styles/units"
	"fyne.io/systray"
	"github.com/dosgo/wslPortForward/config"
	"github.com/dosgo/wslPortForward/proxy"
)

const (
	configFile = "proxy-config.json"
)

var (
	conf       *config.Conf
	logData    string
	logText    *core.Text
	configList *CustomList
	iconImg    image.Image
)

func newCustomList(body *core.Body) {
	configList = &CustomList{data: &conf.Configs, body: body}
	// 主布局框架
	configList.Fr = core.NewFrame(configList.body)
	configList.Fr.Styler(func(s *styles.Style) {
		s.Direction = styles.Column // 垂直布局
	})
	configList.Update()
}

type CustomList struct {
	data *[]*config.ProxyConfig
	body *core.Body
	Fr   *core.Frame
}

func (clist *CustomList) Destroy() {
	for _, item := range clist.Fr.Children {
		for _, item1 := range item.AsTree().Children {
			item1.Destroy()
		}
		item.Destroy()
	}
	clist.Fr.Destroy()
	clist.body.Destroy()
}
func (clist *CustomList) Update() {
	for i, item := range clist.Fr.Children {
		if i >= len(*clist.data) {
			item.Destroy()
		}
	}
	// 动态生成列表项
	for i, item := range *clist.data {
		var row *core.Frame
		var text *core.Text
		var statusCv *core.Canvas
		var editBt *core.Button
		var delBt *core.Button
		if i < len(clist.Fr.Children) {
			row = clist.Fr.Children[i].(*core.Frame)
			text = row.Children[0].(*core.Text)
			statusCv = row.Children[1].(*core.Canvas)
			editBt = row.Children[2].(*core.Button)
			delBt = row.Children[3].(*core.Button)
		} else {
			row = core.NewFrame(clist.Fr)
			text = core.NewText(row)
			statusCv = core.NewCanvas(row)
			editBt = core.NewButton(row)
			delBt = core.NewButton(row)
		}

		row.Styler(func(s *styles.Style) {
			s.Direction = styles.Row
			s.Align.Items = styles.Center
			s.Border.Radius = styles.BorderRadiusMedium
			s.Min.Set(units.Dp(600), units.Dp(20))
		})

		text.SetText(fmt.Sprintf("0.0.0.0:%d → %s (%s)",
			item.ListenPort, item.TargetAddr, item.Protocol))
		statusCv.SetDraw(func(pc *paint.Painter) {
			pc.Circle(0.5, 0.5, 0.3)
			if item.Status {
				pc.Fill.Color = colors.Scheme.Success.Base
			} else {
				pc.Fill.Color = colors.Scheme.Error.Base
			}
			pc.PathDone()
		})
		statusCv.Styler(func(s *styles.Style) {
			s.Min.Set(units.Dp(30), units.Dp(30))
		})
		// 编辑按钮
		editBt.SetText(config.GetLang("Edit")).OnClick(func(e events.Event) {
			showEditDialog(item, clist.body, i)
		})
		// 删除按钮
		delBt.SetText(config.GetLang("Delete")).OnClick(func(e events.Event) {
			if conf.Configs[i].Listener != nil {
				conf.Configs[i].Listener.Close()
			}
			conf.Configs = append(conf.Configs[:i], conf.Configs[i+1:]...)
			config.SaveConfigs(conf, configFile)
			clist.Update()
		})
	}
	clist.Fr.Update()
	clist.body.Update()
}

func main() {
	//set icon
	reader := bytes.NewReader(config.ResourceIconPng)
	iconImg, _, _ = image.Decode(reader)

	initLog()
	config.SetLang("en")
	conf = &config.Conf{}
	config.LoadConfigs(conf, configFile)
	proxy.StartPoxy(conf, false)
	ctx, _ := context.WithCancel(context.Background())
	cmd := config.StartWsl(ctx, conf)
	if cmd != nil {
		defer cmd.Process.Kill()
	}
	systray.RunWithExternalLoop(onReady, onExit)
	if !conf.HideWindow {
		buildUI()
	}
	system.TheApp.MainLoop()
}

func buildUI() {
	mainWindow := core.NewBody(config.GetLang("AppName"))
	mainWindow.Styles.Max.Set(units.Dp(600), units.Dp(500))
	mainWindow.Scene.ContextMenus = nil
	fr := core.NewFrame(mainWindow)
	core.NewFuncButton(fr).SetFunc(func() {
		showAddDialog(mainWindow)
	}).SetText(config.GetLang("AddSettings")) //.SetProperty("", "Add Settings")
	core.NewFuncButton(fr).SetFunc(func() {
		showGlobalSettings(mainWindow)
	}).SetText(config.GetLang("GlobalSettings"))
	core.NewText(mainWindow).SetText(config.GetLang("ProxyList"))
	newCustomList(mainWindow)
	core.NewText(mainWindow).SetText(config.GetLang("Logs"))
	logText = core.NewText(mainWindow)
	logText.SetReadOnly(true)
	logText.SetText(logData)
	logText.Styler(func(s *styles.Style) {
		s.SetTextWrap(true) // 多行模式
		s.Background = colors.Uniform(colors.ToBase(color.RGBA{0xeb, 0xeb, 0xeb, 0x20}))
		s.Padding.Set(units.Dp(8), units.Dp(8))
		s.Border.Radius = styles.BorderRadiusExtraLarge
		s.Min.Set(units.Dp(600), units.Dp(20))
		s.Max.Set(units.Dp(600))
	})
	mainWindow.RunWindow()
	mainWindow.OnClose(func(e events.Event) {
		fr.DeleteChildren()
		fr.Destroy()
		fr = nil
		logText.Destroy()
		logText = nil
		configList.Destroy()
		configList = nil
		mainWindow.DeleteChildren()
		mainWindow.Destroy()
		mainWindow = nil
		go func() {
			time.Sleep(time.Second * 1)
			runtime.GC()
		}()
	})
	if iconImg != nil {
		window := system.TheApp.Window(0)
		if window != nil {
			window.SetIcon([]image.Image{iconImg})
		}
	}
}

func showAddDialog(b *core.Body) {
	cfg := &config.ProxyConfig{
		Protocol:   "tcp",
		ListenPort: 8001,
		TargetAddr: "127.0.0.1:8080",
	}
	showEditDialog(cfg, b, -1)
}

func showEditDialog(cfg *config.ProxyConfig, b *core.Body, index int) {
	var title = config.GetLang("AddSettings")
	if index > -1 {
		title = config.GetLang("EditSettings")
	}
	d := core.NewBody(title)
	d.Scene.ContextMenus = nil
	form := core.NewForm(d)
	form.SetStruct(cfg)
	form.Styles.Min.Set(units.Dp(400), units.Dp(600))
	form.Styles.Max.Set(units.Dp(400), units.Dp(600))
	d.AddBottomBar(func(bar *core.Frame) {
		d.AddCancel(bar)
		d.AddOK(bar).OnClick(func(e events.Event) {
			if cfg.ListenPort < 1 || cfg.ListenPort > 65535 {
				core.MessageSnackbar(d, config.GetLang("PortErrMsg"))
				e.SetHandled()
				return
			}
			for _, v := range conf.Configs {
				if v.Protocol == cfg.Protocol && v.ListenPort == cfg.ListenPort {
					if v.ID != cfg.ID {
						core.MessageSnackbar(d, config.GetLang("PortErrUsed"))
						e.SetHandled()
						return
					}
				}
			}
			cfg.ID = fmt.Sprintf("%d", time.Now().UnixNano())
			if index == -1 {
				conf.Configs = append(conf.Configs, cfg)
			}
			config.SaveConfigs(conf, configFile)
			proxy.StartPoxy(conf, true)
			configList.Update()
		})
	})
	d.OnClose(func(e events.Event) {
		form.DeleteChildren()
		form.Destroy()
		d.DeleteChildren()
		d.Destroy()
	})
	d.RunWindowDialog(b)
}
func showGlobalSettings(b *core.Body) {
	d := core.NewBody(config.GetLang("GlobalSettings"))
	d.Scene.ContextMenus = nil
	form := core.NewForm(d)
	form.SetStruct(conf)
	form.Styles.Min.Set(units.Dp(400), units.Dp(600))
	d.AddBottomBar(func(bar *core.Frame) {
		d.AddCancel(bar)
		d.AddOK(bar).OnClick(func(e events.Event) {
			config.SaveConfigs(conf, configFile)
		})
	})
	d.OnClose(func(e events.Event) {
		form.DeleteChildren()
		form.Destroy()
		d.DeleteChildren()
		d.Destroy()
	})
	d.RunWindowDialog(b)
}
func initLog() {
	r, w, _ := os.Pipe()
	log.SetOutput(w)
	// 启动goroutine实时读取输出
	go func() {
		buf := make([]byte, 1024)
		for {
			n, _ := r.Read(buf)
			if n > 0 {
				// 主线程更新UI
				logData = logData + string(buf[:n])
				if logText != nil {
					logText.SetText(logData)
				}
			}
		}
	}()
}

func onReady() {
	// ------------------------- 设置图标和提示 -------------------------
	if runtime.GOOS == "windows" {
		systray.SetIcon(config.ResourceIconIco) // 使用内嵌的图标数据
	} else {
		systray.SetIcon(config.ResourceIconPng) // 使用内嵌的图标数据
	}
	systray.SetTitle(config.GetLang("AppName"))   // 设置标题（部分平台显示）
	systray.SetTooltip(config.GetLang("AppName")) // 鼠标悬停提示
	mShow := systray.AddMenuItem(config.GetLang("ShowSettings"), config.GetLang("ShowSettings"))
	mQuit := systray.AddMenuItem(config.GetLang("Quit"), config.GetLang("Quit"))

	go func() {
		for {
			select {
			case <-mShow.ClickedCh:
				window := system.TheApp.Window(0)
				if window == nil {
					buildUI()
				}
			case <-mQuit.ClickedCh:
				system.TheApp.Quit()
			}
		}
	}()
}
func onExit() {
}
