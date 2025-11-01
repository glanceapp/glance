package glance

import (
	"os"
	"path/filepath"
)

var dataPath string

func setDataPath(path string) {
	dataPath = path
}

func ensureDir(widgetType string) error {
	return os.MkdirAll(filepath.Join(dataPath, widgetType), 0755)
}

func loadFile(widgetType, key string) ([]byte, error) {
	if err := ensureDir(widgetType); err != nil {
		return nil, err
	}

	return os.ReadFile(filepath.Join(dataPath, widgetType, key))
}

func saveFile(widgetType, key string, data []byte) error {
	if err := ensureDir(widgetType); err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(dataPath, widgetType, key), data, 0644)
}
