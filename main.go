package main

import (
	"context"
	"errors"
	"fmt"
	"image/color"
	"log"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
	"github.com/dosgo/wslPortForward/config"
	"github.com/dosgo/wslPortForward/proxy"
)

const (
	configFile = "proxy-config.json"
)

// 在buildUI函数中添加WSL参数控件
// 在全局变量区添加完整声明
var (
	conf       *config.Conf
	configList *widget.List
	mainWindow fyne.Window
	logData    *widget.TextGrid
)

// 在main函数中添加日志初始化
func main() {
	logData = widget.NewTextGrid()
	initLog()
	myApp := app.New()
	conf = &config.Conf{}
	config.LoadConfigs(conf, configFile)

	mainWindow = myApp.NewWindow(config.GetLang("AppName"))
	mainWindow.SetIcon(fyne.NewStaticResource("icon", config.ResourceIconPng))
	mainWindow.SetCloseIntercept(func() { mainWindow.Hide() }) // 点击关闭隐藏窗口
	proxy.StartPoxy(conf, false)
	buildUI()
	ctx, _ := context.WithCancel(context.Background())
	cmd := startWsl(ctx)
	if cmd != nil {
		defer cmd.Process.Kill()
	}
	// 系统托盘支持
	if desk, ok := myApp.(desktop.App); ok {
		menu := fyne.NewMenu("Proxy Manager",
			fyne.NewMenuItem(config.GetLang("ShowSettings"), func() { mainWindow.Show() }),
			// 修改系统托盘退出菜单项
			fyne.NewMenuItem(config.GetLang("Quit"), func() {
				myApp.Quit()
			}),
		)
		desk.SetSystemTrayMenu(menu)
		desk.SetSystemTrayIcon(fyne.NewStaticResource("icon", config.ResourceIconPng))
	}

	if !conf.HideWindow {
		mainWindow.Show()
	}
	myApp.Run()

}

// 在 buildUI 函数中修改主窗口布局
func buildUI() {
	// 配置列表
	configList = widget.NewList(
		func() int { return len(conf.Configs) },
		func() fyne.CanvasObject {
			return container.NewHBox(
				widget.NewLabel("Template"),
				container.NewCenter(container.NewGridWrap(
					fyne.NewSize(20, 20), // 设置圆形直径
					canvas.NewCircle(color.RGBA{R: 255, A: 255}),
				)),
				widget.NewButton(config.GetLang("Edit"), nil),
				widget.NewButton(config.GetLang("Delete"), nil),
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			box := obj.(*fyne.Container)
			cfg := conf.Configs[id]

			label := box.Objects[0].(*widget.Label)
			label.SetText(fmt.Sprintf("0.0.0.0:%d → %s (%s)",
				cfg.ListenPort, cfg.TargetAddr, cfg.Protocol))

			statusLabel := box.Objects[1].(*fyne.Container).Objects[0].(*fyne.Container).Objects[0].(*canvas.Circle)
			if cfg.Status {
				statusLabel.FillColor = color.RGBA{R: 0, G: 255, B: 00, A: 255}
			} else {
				statusLabel.FillColor = color.RGBA{R: 255, G: 0, B: 00, A: 255}
			}
			editBtn := box.Objects[2].(*widget.Button)
			editBtn.OnTapped = func() { showEditDialog(cfg) }

			delBtn := box.Objects[3].(*widget.Button)
			delBtn.OnTapped = func() { deleteConfig(cfg) }
		},
	)

	configScroll := container.NewScroll(configList)
	configScroll.SetMinSize(fyne.NewSize(0, 150)) // 设置最小高度300像素可显示更多条目

	logScroll := container.NewScroll(logData)
	logScroll.SetMinSize(fyne.NewSize(600, 300)) // 设置日志框最小尺寸

	// 在 buildUI 函数末尾添加全局设置按钮
	globalSettingsBtn := widget.NewButton(config.GetLang("GlobalSettings"), showGlobalSettings)
	addBtn := widget.NewButton(config.GetLang("AddSettings"), showAddDialog)

	// 修改主窗口顶部布局添加全局设置按钮
	mainWindow.SetContent(container.NewBorder(
		container.NewVBox(
			container.NewHBox(addBtn, globalSettingsBtn),
			widget.NewSeparator(),
		),
		container.NewVBox(
			widget.NewSeparator(),
			widget.NewLabel(config.GetLang("Logs")),
			logScroll,
		),
		nil, nil,
		container.NewVBox(
			widget.NewLabel(config.GetLang("ProxyList")),
			configScroll,
		),
	))
}

func deleteConfig(cfg *config.ProxyConfig) {
	for i, c := range conf.Configs {
		if c.ID == cfg.ID {
			conf.Configs = append(conf.Configs[:i], conf.Configs[i+1:]...)
			if cfg.Listener != nil {
				cfg.Listener.Close()
			}
			break
		}
	}
	config.SaveConfigs(conf, configFile)
	configList.Refresh()
}
func showAddDialog() {
	showConfigDialog(&config.ProxyConfig{
		Protocol:   "tcp",
		ListenPort: 8001,
		TargetAddr: "127.0.0.1:8080",
	}, func(cfg *config.ProxyConfig) {
		cfg.ID = fmt.Sprintf("%d", time.Now().UnixNano())
		conf.Configs = append(conf.Configs, cfg)
		config.SaveConfigs(conf, configFile)
		proxy.StartPoxy(conf, true)
		configList.Refresh()
	})
}

func showEditDialog(cfg *config.ProxyConfig) {
	showConfigDialog(cfg, func(updated *config.ProxyConfig) {
		*cfg = *updated
		config.SaveConfigs(conf, configFile)
		configList.Refresh()
	})
}

// 修改后的配置对话框
func showConfigDialog(cfg *config.ProxyConfig, onSave func(*config.ProxyConfig)) {
	protocol := widget.NewSelect([]string{"tcp", "udp"}, nil)
	listenAddr := widget.NewEntry()
	targetAddr := widget.NewEntry()

	// 初始化表单值（仅保留核心参数）
	protocol.SetSelected(cfg.Protocol)
	listenAddr.SetText(fmt.Sprintf("%d", cfg.ListenPort))
	targetAddr.SetText(cfg.TargetAddr)

	form := &widget.Form{
		Items: []*widget.FormItem{
			{Text: config.GetLang("Protocol"), Widget: protocol},
			{Text: config.GetLang("ListenAddr"), Widget: listenAddr},
			{Text: config.GetLang("TargetAddr"), Widget: targetAddr},
		},
	}

	var confDialog *dialog.ConfirmDialog
	confDialog = dialog.NewCustomConfirm(config.GetLang("EditSettings"), config.GetLang("Save"), config.GetLang("Cancel"), form, func(b bool) {
		if !b {
			return
		}
		num, _ := strconv.ParseInt(listenAddr.Text, 10, 64)

		if num < 1 || num > 65535 {
			ErrorDialog := dialog.NewError(errors.New(config.GetLang("PortErrMsg")), mainWindow)
			ErrorDialog.Show()
			ErrorDialog.SetOnClosed(func() {
				confDialog.Show()
			})
			return
		}

		for _, v := range conf.Configs {
			if v.Protocol == cfg.Protocol && v.ListenPort == cfg.ListenPort {
				if v.ID != cfg.ID {
					ErrorDialog := dialog.NewError(errors.New(config.GetLang("PortErrUsed")), mainWindow)
					ErrorDialog.Show()
					ErrorDialog.SetOnClosed(func() {
						confDialog.Show()
					})
					return
				}
			}
		}

		newCfg := &config.ProxyConfig{
			Protocol:   protocol.Selected,
			ListenPort: int(num),
			TargetAddr: targetAddr.Text,
		}
		onSave(newCfg)
	}, mainWindow)
	confDialog.Show()
}

// 新增全局设置对话框
func showGlobalSettings() {

	startWslCheck := widget.NewCheck(config.GetLang("WslStart"), func(b bool) { conf.StartWsl = b })
	wslCommandEntry := widget.NewEntry()
	showWslCheck := widget.NewCheck(config.GetLang("WslShow"), func(b bool) { conf.ShowWsl = b })

	hideWindowCheck := widget.NewCheck(config.GetLang("HideWindow"), func(b bool) { conf.HideWindow = b })

	startWslCheck.SetChecked(conf.StartWsl)
	wslCommandEntry.SetText(conf.WslArgs)
	showWslCheck.SetChecked(conf.ShowWsl)
	hideWindowCheck.SetChecked(conf.HideWindow)
	form := &widget.Form{
		Items: []*widget.FormItem{
			{Text: config.GetLang("WslStart"), Widget: startWslCheck},
			{Text: config.GetLang("WslArgs"), Widget: wslCommandEntry},
			{Text: config.GetLang("WslShow"), Widget: showWslCheck},
			{Text: config.GetLang("HideWindow"), Widget: hideWindowCheck},
		},
	}

	dialog.ShowCustomConfirm(config.GetLang("GlobalSettings"), config.GetLang("Save"), config.GetLang("Cancel"), form, func(b bool) {
		if b {
			conf.WslArgs = wslCommandEntry.Text
			config.SaveConfigs(conf, configFile)
		}
	}, mainWindow)
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
					logData.SetText(logData.Text() + string(buf[:n]))
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
		log.Printf("WSL start  Args:%s\r\n", conf.WslArgs)
		if err := cmd.Start(); err != nil {
			log.Printf("WSL start  command:%s err: %+v\r\n", conf.WslArgs, err)
			return nil
		}
		return cmd
	}
	return nil
}
