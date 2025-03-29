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
	"github.com/dosgo/wslPortForward/config"
	"github.com/dosgo/wslPortForward/proxy"
	"github.com/getlantern/systray"
)

const (
	configFile = "proxy-config.json"
)

var (
	conf       *config.Conf
	mainWindow *core.Body
	logData    string
	logText    *core.Text
	configList *CustomList
)

func newCustomList(body *core.Body) *CustomList {
	customList := &CustomList{data: &conf.Configs, body: body}
	// 主布局框架
	customList.Fr = core.NewFrame(customList.body)
	customList.Fr.Styler(func(s *styles.Style) {
		s.Direction = styles.Column // 垂直布局
	})
	customList.Update()
	return customList
}

type CustomList struct {
	data *[]*config.ProxyConfig
	body *core.Body
	Fr   *core.Frame
}

func (clist *CustomList) Update() {
	clist.Fr.DeleteChildren() // 清空现有内容
	// 动态生成列表项
	for i, item := range *clist.data {
		// 单项容器
		row := core.NewFrame(clist.Fr)
		row.Styler(func(s *styles.Style) {
			s.Direction = styles.Row
			s.Align.Items = styles.Center
			s.Border.Radius = styles.BorderRadiusMedium
			s.Min.Set(units.Dp(800), units.Dp(20))
		})

		core.NewText(row).SetText(fmt.Sprintf("0.0.0.0:%d → %s (%s)",
			item.ListenPort, item.TargetAddr, item.Protocol))
		statusCv := core.NewCanvas(row)
		statusCv.SetDraw(func(pc *paint.Context) {
			pc.DrawCircle(0.5, 0.5, 0.3)
			if item.Status {
				pc.FillStyle.Color = colors.Scheme.Success.Base
			} else {
				pc.FillStyle.Color = colors.Scheme.Error.Base
			}
			pc.Fill()
		})

		statusCv.Styler(func(s *styles.Style) {
			s.Min.Set(units.Dp(30), units.Dp(30))
		})

		// 编辑按钮
		core.NewButton(row).SetText(config.GetLang("Edit")).OnClick(func(e events.Event) {
			showEditDialog(item, clist.body, i)
		})
		// 删除按钮
		core.NewButton(row).SetText(config.GetLang("Delete")).OnClick(func(e events.Event) {
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
	initLog()
	conf = &config.Conf{}
	config.LoadConfigs(conf, configFile)
	proxy.StartPoxy(conf, false)
	ctx, _ := context.WithCancel(context.Background())
	cmd := config.StartWsl(ctx, conf)
	if cmd != nil {
		defer cmd.Process.Kill()
	}
	go func() { systray.Run(onReady, onExit) }()
	if !conf.HideWindow {
		buildUI()
	}
	system.TheApp.MainLoop()
}

func buildUI() {
	mainWindow = core.NewBody(config.GetLang("AppName"))
	mainWindow.Styles.Min.Set(units.Dp(800), units.Dp(600))
	mainWindow.Scene.ContextMenus = nil

	fr := core.NewFrame(mainWindow)
	core.NewFuncButton(fr).SetFunc(func() {
		showAddDialog(mainWindow)
	}).SetText(config.GetLang("AddSettings")) //.SetProperty("", "Add Settings")

	core.NewFuncButton(fr).SetFunc(func() {
		showGlobalSettings(mainWindow)
	}).SetText(config.GetLang("GlobalSettings"))
	core.NewText(mainWindow).SetText(config.GetLang("ProxyList"))
	configList = newCustomList(mainWindow)
	core.NewText(mainWindow).SetText(config.GetLang("Logs"))
	logText = core.NewText(mainWindow)
	logText.SetReadOnly(true)
	logText.SetText(logData)
	logText.Styler(func(s *styles.Style) {
		s.SetTextWrap(true) // 多行模式
		s.Background = colors.Uniform(colors.ToBase(color.RGBA{0xeb, 0xeb, 0xeb, 0x20}))
		s.Padding.Set(units.Dp(8), units.Dp(8))
		s.Border.Radius = styles.BorderRadiusExtraLarge
		s.Min.Set(units.Dp(800), units.Dp(20))
		s.Text.WhiteSpace = styles.WhiteSpacePre
		s.Max.Set(units.Dp(800))
	})
	mainWindow.RunWindow()

	//set icon
	reader := bytes.NewReader(config.ResourceIconPng)
	img, _, err := image.Decode(reader)
	if err == nil {
		window := system.TheApp.Window(0)
		if window != nil {
			window.SetIcon([]image.Image{img})
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

	// ------------------------- 添加菜单项 -------------------------
	// 普通菜单项
	mShow := systray.AddMenuItem(config.GetLang("ShowSettings"), config.GetLang("ShowSettings"))

	// 退出项
	mQuit := systray.AddMenuItem(config.GetLang("Quit"), config.GetLang("Quit"))

	// ------------------------- 处理菜单点击事件 -------------------------
	go func() {
		for {
			select {
			case <-mShow.ClickedCh:
				buildUI()
			case <-mQuit.ClickedCh:
				system.TheApp.Quit()
				return
			}
		}
	}()
}

func onExit() {
}
