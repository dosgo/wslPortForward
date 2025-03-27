package main

import (
	"context"
	"fmt"
	"image/color"
	"log"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"cogentcore.org/core/colors"
	"cogentcore.org/core/core"

	"cogentcore.org/core/events"
	"cogentcore.org/core/paint"
	"cogentcore.org/core/styles"
	"cogentcore.org/core/styles/units"
	"github.com/dosgo/wslPortForward/config"
	"github.com/dosgo/wslPortForward/proxy"
)

const (
	configFile = "proxy-config.json"
)

var (
	conf       *config.Conf
	mainWindow *core.Body
	logData    *core.Text
	configList *CustomList
	logFile    *os.File
	logMutex   sync.Mutex
)

func newCustomList() *CustomList {
	customList := &CustomList{data: &conf.Configs, body: mainWindow}

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

		// 标题
		core.NewText(row).SetText(fmt.Sprintf("0.0.0.0:%d → %s (%s)",
			item.ListenPort, item.TargetAddr, item.Protocol)).Styler(func(s *styles.Style) {
			//s.FlexGrow = 1 // 标题部分自动填充空间

		})

		statusCv := core.NewCanvas(row)
		statusCv.SetDraw(func(pc *paint.Context) {
			pc.DrawCircle(0.5, 0.5, 0.1)
			if item.Status {
				pc.FillStyle.Color = colors.Scheme.Success.Base
			} else {
				pc.FillStyle.Color = colors.Scheme.Error.Base
			}
			pc.Fill()
		})

		statusCv.Styler(func(s *styles.Style) {
			s.Min.Set(units.Dp(60), units.Dp(60))
		})

		// 编辑按钮
		core.NewButton(row).SetText("Edit").OnClick(func(e events.Event) {
			showEditDialog(item, clist.body, i)
		})
		// 删除按钮
		core.NewButton(row).SetText("remove").OnClick(func(e events.Event) {
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
	mainWindow = core.NewBody("wslPortForward")
	mainWindow.Styles.Min.Set(units.Dp(800), units.Dp(600))
	mainWindow.Scene.ContextMenus = nil
	conf = &config.Conf{}
	config.LoadConfigs(conf, configFile)
	buildUI(mainWindow)
	proxy.StartPoxy(conf, false)
	ctx, _ := context.WithCancel(context.Background())
	cmd := startWsl(ctx)
	if cmd != nil {
		defer cmd.Process.Kill()
	}
	mainWindow.RunMainWindow()
}

func buildUI(b *core.Body) {

	fr := core.NewFrame(b)

	core.NewFuncButton(fr).SetFunc(func() {
		showAddDialog(b)
	}).SetText("Add Settings") //.SetProperty("", "Add Settings")

	core.NewFuncButton(fr).SetFunc(func() {
		showGlobalSettings(b)
	}).SetText("Global Settings")
	core.NewText(b).SetText("Proxy List:")
	configList = newCustomList()
	core.NewText(b).SetText("Logs:")
	logData = core.NewText(b)
	logData.SetText("")
	logData.SetReadOnly(true)
	logData.Styler(func(s *styles.Style) {
		s.SetTextWrap(true) // 多行模式
		s.Background = colors.Uniform(colors.ToBase(color.RGBA{0xeb, 0xeb, 0xeb, 0x20}))
		s.Padding.Set(units.Dp(8), units.Dp(8))
		s.Border.Radius = styles.BorderRadiusExtraLarge
		s.Min.Set(units.Dp(800), units.Dp(20))
		s.Text.WhiteSpace = styles.WhiteSpacePre
		s.Max.Set(units.Dp(800))
	})
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
	var title = "Add Settings"
	if index > -1 {
		title = "Edit Settings"
	}
	d := core.NewBody(title)
	form := core.NewForm(d)
	form.SetStruct(cfg)
	form.Styles.Min.Set(units.Dp(400), units.Dp(600))
	d.AddBottomBar(func(bar *core.Frame) {
		d.AddCancel(bar).OnClick(func(e events.Event) {

		})
		d.AddOK(bar).OnClick(func(e events.Event) {
			if cfg.ListenPort < 1 || cfg.ListenPort > 65535 {
				core.MessageSnackbar(b, "The port can only be 1-65535")
				//showEditDialog(cfg, b, index)
				return
			}

			for _, v := range conf.Configs {
				if v.Protocol == cfg.Protocol && v.ListenPort == cfg.ListenPort {
					if v.ID != cfg.ID {
						core.MessageSnackbar(b, "Port is used")
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
	d := core.NewBody("Global Settings")
	form := core.NewForm(d)
	form.SetStruct(conf)
	form.Styles.Min.Set(units.Dp(400), units.Dp(600))
	d.AddBottomBar(func(bar *core.Frame) {
		d.AddCancel(bar).OnClick(func(e events.Event) {

		})
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
				if logData != nil {
					logData.SetText(logData.Text + string(buf[:n]))
				}
			}
		}
	}()
}

func startWsl(ctx context.Context) *exec.Cmd {
	if conf.StartWsl {
		cmd := exec.CommandContext(ctx, "wsl", conf.WslArgs)
		if !conf.ShowWsl {
			cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		}
		fmt.Printf("WSL start  command:%s\r\n", conf.WslArgs)
		if err := cmd.Start(); err != nil {
			log.Printf("WSL start  command:%s err: %+v\r\n", conf.WslArgs, err)
			return nil
		}
		return cmd
	}
	return nil
}
