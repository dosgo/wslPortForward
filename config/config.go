package config

import (
	_ "embed"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
)

type ProxyConfig struct {
	ID         string       `display:"-" json:"id"`
	Protocol   string       `json:"protocol" label:"Protocol:"`
	ListenPort int          `json:"listenPort"`
	TargetAddr string       `json:"targetAddr"`
	Listener   net.Listener `json:"-" display:"-"`
	UdpConn    *net.UDPConn `json:"-" display:"-"`
	Status     bool         `json:"-" display:"-"`
}

type Conf struct {
	Configs    []*ProxyConfig `display:"-" json:"configs"`
	StartWsl   bool           `json:"startWsl"`
	WslArgs    string         `json:"wslArgs"`
	ShowWsl    bool           `json:"showWsl"`
	HideWindow bool           `json:"HideWindow"`
}

func SaveConfigs(conf *Conf, configFile string) {
	path := filepath.Join(appDataDir(), configFile)
	data, _ := json.MarshalIndent(conf, "", "  ")
	os.MkdirAll(filepath.Dir(path), 0755)
	os.WriteFile(path, data, 0644)
}

func LoadConfigs(conf *Conf, configFile string) {
	path := filepath.Join(appDataDir(), configFile)
	data, _ := os.ReadFile(path)
	json.Unmarshal(data, &conf)
}

func appDataDir() string {
	dir, _ := os.UserConfigDir()
	return filepath.Join(dir, "wslPortForward")
}

var langMap = map[string]map[string]string{
	"en": {
		"Quit":           "Quit",
		"ShowSettings":   "ShowSettings",
		"AppName":        "WslPortForward",
		"Edit":           "Edit",
		"Delete":         "delete",
		"AddSettings":    "Add Settings",
		"EditSettings":   "Edit Settings",
		"GlobalSettings": "Global Settings",
		"Logs":           "Logs:",
		"ProxyList":      "Proxy List:",
		"Protocol":       "Protocol",
		"ListenAddr":     "Listen Addr",
		"TargetAddr":     "Target Addr",
		"Save":           "Save",
		"Cancel":         "Cancel",
		"PortErrMsg":     "The port can only be 1-65535",
		"PortErrUsed":    "Port is used",
		"WslStart":       "Start WSL",
		"WslShow":        "Show WSL Window",
		"WslArgs":        "WSL Start Args",
		"HideWindow":     "Hide Window",
	},
	"zh": {
		"Quit":           "退出",
		"ShowSettings":   "显示窗口",
		"AppName":        "Wsl端口转发",
		"Edit":           "编辑",
		"Delete":         "删除",
		"AddSettings":    "添加设置",
		"EditSettings":   "编辑设置",
		"GlobalSettings": "全局设置",
		"Logs":           "日志:",
		"ProxyList":      "代理列表:",
		"Protocol":       "协议",
		"ListenAddr":     "监听地址",
		"TargetAddr":     "目标地址",
		"Save":           "保存",
		"Cancel":         "取消",
		"PortErrMsg":     "端口号只能1-65535",
		"PortErrUsed":    "端口已被使用",
		"WslStart":       "启动WSL",
		"WslShow":        "显示WSL窗口",
		"WslArgs":        "WSL启动参数",
		"HideWindow":     "隐藏窗口",
	},
}

// 获取当前语言文本
func GetLang(key string) string {
	lang := os.Getenv("LANG")
	var currentLang = "en"
	if strings.Contains(lang, "zh_CN") || strings.Contains(lang, "zh_TW") {
		currentLang = "zh"
	}
	return langMap[currentLang][key]
}

//go:embed icon.png
var ResourceIconPng []byte
