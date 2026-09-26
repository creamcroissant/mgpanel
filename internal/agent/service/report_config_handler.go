package service

import (
	"os"
)

func (a *Agent) readConfigFile() (string, error) {
	a.configFileMu.RLock()
	path := a.configFilePath
	a.configFileMu.RUnlock()
	if path == "" {
		return "", nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	a.configFileMu.Lock()
	a.configFileContent = string(data)
	a.configFileMu.Unlock()
	return a.configFileContent, nil
}

func (a *Agent) checkConfigFile() bool {
	if a == nil {
		return false
	}
	a.configFileMu.RLock()
	path := a.configFilePath
	a.configFileMu.RUnlock()
	if path == "" {
		return false
	}
	// Re-read file content and update cache
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	a.configFileMu.Lock()
	a.configFileContent = string(data)
	a.configFileMu.Unlock()
	return true
}

// currentConfigYAML 返回本次上报应携带的运行配置全文：优先重读磁盘（push_config
// 刚写盘的场景靠它透出最新值），失败则回退内存缓存，再失败返回空（panel跳过落库）。
func (a *Agent) currentConfigYAML() string {
	if a == nil {
		return ""
	}
	if fresh, err := a.readConfigFile(); err == nil && fresh != "" {
		return fresh
	}
	a.configFileMu.RLock()
	defer a.configFileMu.RUnlock()
	return a.configFileContent
}

func (a *Agent) SetConfigFilePath(path string) {
	a.configFileMu.Lock()
	a.configFilePath = path
	a.configFileMu.Unlock()
	// Read file content into cache so it's available immediately
	_, _ = a.readConfigFile()
}
