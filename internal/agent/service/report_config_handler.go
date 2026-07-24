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

func (a *Agent) SetConfigFilePath(path string) {
	a.configFileMu.Lock()
	a.configFilePath = path
	a.configFileMu.Unlock()
	// Read file content into cache so it's available immediately
	_, _ = a.readConfigFile()
}
