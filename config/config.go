package config

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
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
	Configs  []*ProxyConfig `display:"-" json:"configs"`
	StartWsl bool           `json:"startWsl"`
	WslArgs  string         `json:"wslArgs"`
	ShowWsl  bool           `json:"showWsl"`
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
