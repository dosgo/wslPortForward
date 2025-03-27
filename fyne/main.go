package main

import (
	_ "embed"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
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
	logData    binding.String
)

// 在main函数中添加日志初始化
func main() {
	initLog()
	logData = binding.NewString()
	myApp := app.New()
	conf = &config.Conf{}
	config.LoadConfigs(conf, configFile)
	// 系统托盘支持
	if desk, ok := myApp.(desktop.App); ok {
		menu := fyne.NewMenu("Proxy Manager",
			fyne.NewMenuItem("显示窗口", showMainWindow),
			// 修改系统托盘退出菜单项
			fyne.NewMenuItem("退出", func() {
				myApp.Quit()
			}),
		)
		desk.SetSystemTrayMenu(menu)
		//	desk.SetSystemTrayIcon(fyne.NewStaticResource("icon", resourceIconPng))
	}

	mainWindow = myApp.NewWindow("代理配置管理器")
	mainWindow.SetCloseIntercept(func() { mainWindow.Hide() }) // 点击关闭隐藏窗口
	buildUI()

	// 自动启动配置
	proxy.StartPoxy(conf, false)
	mainWindow.ShowAndRun()
}

// 在 buildUI 函数中修改主窗口布局
func buildUI() {
	// 配置列表
	configList = widget.NewList(
		func() int { return len(conf.Configs) },
		func() fyne.CanvasObject {
			return container.NewHBox(
				widget.NewLabel("Template"),
				widget.NewLabel(""),
				widget.NewButton("编辑", nil),
				widget.NewButton("删除", nil),
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			box := obj.(*fyne.Container)
			cfg := conf.Configs[id]

			label := box.Objects[0].(*widget.Label)
			label.SetText(fmt.Sprintf("0.0.0.0:%d → %s (%s)",
				cfg.ListenPort, cfg.TargetAddr, cfg.Protocol))

			statusLabel := box.Objects[1].(*widget.Label)
			if cfg.Status {
				statusLabel.SetText("代理中")
			} else {
				statusLabel.SetText("")
			}
			editBtn := box.Objects[2].(*widget.Button)
			editBtn.OnTapped = func() { showEditDialog(cfg) }

			delBtn := box.Objects[3].(*widget.Button)
			delBtn.OnTapped = func() { deleteConfig(cfg) }
		},
	)

	configScroll := container.NewScroll(configList)
	configScroll.SetMinSize(fyne.NewSize(0, 300)) // 设置最小高度300像素可显示更多条目

	logEntry := widget.NewMultiLineEntry()
	logEntry.Bind(logData)
	logEntry.Disable()

	logEntry.TextStyle = fyne.TextStyle{
		Monospace: false,
		TabWidth:  0,
	}
	//logEntry.Color = color.RGBA{R: 0, G: 0, B: 0, A: 255}
	logScroll := container.NewScroll(logEntry)
	logScroll.SetMinSize(fyne.NewSize(600, 400)) // 设置日志框最小尺寸

	// 在 buildUI 函数末尾添加全局设置按钮
	globalSettingsBtn := widget.NewButton("全局设置", showGlobalSettings)
	addBtn := widget.NewButton("添加配置", showAddDialog)

	// 修改主窗口顶部布局添加全局设置按钮
	mainWindow.SetContent(container.NewBorder(
		container.NewVBox(
			container.NewHBox(addBtn, globalSettingsBtn),
			widget.NewSeparator(),
		),
		container.NewVBox(
			widget.NewSeparator(),
			widget.NewLabel("运行日志:"),
			logScroll,
		),
		nil, nil,
		container.NewVBox(
			widget.NewLabel("代理列表"),
			configList,
		),
	))
}

func deleteConfig(cfg *config.ProxyConfig) {

	proxy.StartPoxy(conf, true)
	for i, c := range conf.Configs {
		if c.ID == cfg.ID {
			conf.Configs = append(conf.Configs[:i], conf.Configs[i+1:]...)
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
			{Text: "协议", Widget: protocol},
			{Text: "监听地址", Widget: listenAddr},
			{Text: "目标地址", Widget: targetAddr},
		},
	}

	dialog.ShowCustomConfirm("编辑配置", "保存", "取消", form, func(b bool) {
		if !b {
			return
		}
		num, _ := strconv.ParseInt(listenAddr.Text, 16, 64)
		newCfg := &config.ProxyConfig{
			Protocol:   protocol.Selected,
			ListenPort: int(num),
			TargetAddr: targetAddr.Text,
		}
		onSave(newCfg)
	}, mainWindow)
}

func showMainWindow() {
	mainWindow.Show()
}

// 嵌入图标资源（需准备icon.png）
//
////go:///embed icon.png
//var resourceIconPng []byte

// 新增全局设置对话框
func showGlobalSettings() {

	startWslCheck := widget.NewCheck("启动WSL服务", func(b bool) { conf.StartWsl = b })
	wslCommandEntry := widget.NewEntry()
	showWslCheck := widget.NewCheck("显示WSL窗口", func(b bool) { conf.ShowWsl = b })

	startWslCheck.SetChecked(conf.StartWsl)
	wslCommandEntry.SetText(conf.WslArgs)
	showWslCheck.SetChecked(conf.ShowWsl)

	form := &widget.Form{
		Items: []*widget.FormItem{
			{Text: "启动WSL服务", Widget: startWslCheck},
			{Text: "WSL启动命令", Widget: wslCommandEntry},
			{Text: "显示WSL窗口", Widget: showWslCheck},
		},
	}

	dialog.ShowCustomConfirm("全局设置", "保存", "取消", form, func(b bool) {
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
					logData.Set(string(buf[:n]))
				}
			}
		}
	}()
}
