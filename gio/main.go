package main

import (
	"fmt"
	"image/color"
	"log"
	"os"
	"strings"
	"sync"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/dosgo/wslPortForward/config"
	"github.com/dosgo/wslPortForward/proxy"
)

type UIState struct {
	conf          *config.Conf
	th            *material.Theme
	wslCommand    widget.Editor
	showAddDialog bool
	editConfig    *config.ProxyConfig
	logBuffer     []string
	logMutex      sync.Mutex

	// Widgets
	configList layout.List
	addBtn     widget.Clickable
	globalBtn  widget.Clickable
	deleteBtns []widget.Clickable
	editBtns   []widget.Clickable
}

const (
	configFile = "proxy-config.json"
)

func main() {
	ui := &UIState{
		conf:       &config.Conf{},
		configList: layout.List{Axis: layout.Vertical},
		th:         material.NewTheme(),
	}

	config.LoadConfigs(ui.conf, configFile)

	go func() {

		w := new(app.Window)
		w.Option(app.Title(config.GetLang("AppName")))
		ui.initLog()
		proxy.StartPoxy(ui.conf, false)
		if err := ui.Loop(w); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}

func (ui *UIState) Loop(w *app.Window) error {
	var ops op.Ops

	for {
		switch e := w.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			ui.Layout(gtx)
			e.Frame(gtx.Ops)
		}

	}
}

func (ui *UIState) Layout(gtx layout.Context) layout.Dimensions {
	// 处理按钮点击事件
	if ui.addBtn.Clicked(gtx) {
		ui.showAddDialog = true
	}
	if ui.globalBtn.Clicked(gtx) {
		ui.showGlobalSettings()
	}

	// 主界面布局
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(ui.renderToolbar),
		layout.Rigid(ui.renderConfigList),
		layout.Rigid(ui.renderLogs),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if ui.showAddDialog {
				return ui.renderAddDialog(gtx)
			}
			return layout.Dimensions{}
		}),
	)
}

func (ui *UIState) renderToolbar(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Rigid(material.Button(ui.th, &ui.addBtn, config.GetLang("AddSettings")).Layout),
		layout.Rigid(material.Button(ui.th, &ui.globalBtn, config.GetLang("GlobalSettings")).Layout),
	)
}

func (ui *UIState) renderConfigList(gtx layout.Context) layout.Dimensions {
	ui.deleteBtns = make([]widget.Clickable, len(ui.conf.Configs))
	ui.editBtns = make([]widget.Clickable, len(ui.conf.Configs))

	return ui.configList.Layout(gtx, len(ui.conf.Configs), func(gtx layout.Context, i int) layout.Dimensions {
		cfg := ui.conf.Configs[i]
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				color1 := color.NRGBA{R: 255, A: 255}
				if cfg.Status {
					color1 = color.NRGBA{R: 0, G: 255, A: 255}
				}
				return widget.Border{
					Color: color1,
					Width: unit.Dp(2),
				}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Dimensions{Size: gtx.Constraints.Max}
				})
			}),
			layout.Rigid(material.Label(ui.th, unit.Sp(14),
				fmt.Sprintf("%d → %s (%s)", cfg.ListenPort, cfg.TargetAddr, cfg.Protocol)).Layout),
			layout.Rigid(material.Button(ui.th, &ui.editBtns[i], config.GetLang("Edit")).Layout),
			layout.Rigid(material.Button(ui.th, &ui.deleteBtns[i], config.GetLang("Delete")).Layout),
		)
	})
}

func (ui *UIState) renderAddDialog(gtx layout.Context) layout.Dimensions {
	// 实现类似原showConfigDialog的功能
	// 使用Gio的输入组件构建表单
	// 处理验证和保存逻辑
	return layout.Dimensions{}
}

func (ui *UIState) showGlobalSettings() {
	// 实现全局设置逻辑
	// 使用Gio的组件构建表单
}

func (ui *UIState) initLog() {
	r, w, _ := os.Pipe()
	log.SetOutput(w)
	go func() {
		buf := make([]byte, 1024)
		for {
			n, _ := r.Read(buf)
			if n > 0 {
				ui.logMutex.Lock()
				ui.logBuffer = append(ui.logBuffer, string(buf[:n]))
				if len(ui.logBuffer) > 100 {
					ui.logBuffer = ui.logBuffer[1:]
				}
				ui.logMutex.Unlock()
				// 触发界面刷新
				//app.Invalidate()

			}
		}
	}()
}

func (ui *UIState) renderLogs(gtx layout.Context) layout.Dimensions {
	ui.logMutex.Lock()
	defer ui.logMutex.Unlock()

	return widget.Border{
		Width: unit.Dp(1),
		Color: color.NRGBA{A: 100},
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return material.Editor(ui.th, &widget.Editor{
			ReadOnly:   true,
			WrapPolicy: text.WrapGraphemes,
		}, strings.Join(ui.logBuffer, "")).Layout(gtx)
	})
}
