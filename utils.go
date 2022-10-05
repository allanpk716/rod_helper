package rod_helper

import (
	"os"
	"path/filepath"
)

// IsDir 存在且是文件夹
func IsDir(path string) bool {
	s, err := os.Stat(path)
	if err != nil {
		return false
	}
	return s.IsDir()
}

// IsFile 存在且是文件
func IsFile(filePath string) bool {
	s, err := os.Stat(filePath)
	if err != nil {
		return false
	}
	return !s.IsDir()
}

// WriteFile 写文件
func WriteFile(desFileFPath string, bytes []byte) error {
	var err error
	nowDesPath := desFileFPath
	if filepath.IsAbs(nowDesPath) == false {
		nowDesPath, err = filepath.Abs(nowDesPath)
		if err != nil {
			return err
		}
	}
	// 创建对应的目录
	nowDirPath := filepath.Dir(nowDesPath)
	err = os.MkdirAll(nowDirPath, os.ModePerm)
	if err != nil {
		return err
	}
	file, err := os.Create(nowDesPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	_, err = file.Write(bytes)
	if err != nil {
		return err
	}

	return nil
}
