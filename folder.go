package rod_helper

import (
	"github.com/WQGroup/logger"
	"os"
	"path/filepath"
)

// GetRodTmpRootFolder 在程序的根目录新建，rod 缓存用文件夹
func GetRodTmpRootFolder(nowProcessRoot string) string {

	if nowProcessRoot == "" {
		nowProcessRoot = "."
	}
	nowProcessRoot = filepath.Join(nowProcessRoot, RodCacheFolder)
	err := os.MkdirAll(nowProcessRoot, os.ModePerm)
	if err != nil {
		logger.Panicln(err)
	}
	return nowProcessRoot
}

// ClearRodTmpRootFolder 清理 rod 缓存文件夹
func ClearRodTmpRootFolder(nowProcessRoot string) error {

	if nowProcessRoot == "" {
		nowProcessRoot = "."
	}

	nowTmpFolder := GetRodTmpRootFolder(nowProcessRoot)
	pathSep := string(os.PathSeparator)
	files, err := os.ReadDir(nowTmpFolder)
	if err != nil {
		return err
	}
	for _, curFile := range files {
		fullPath := nowTmpFolder + pathSep + curFile.Name()
		if curFile.IsDir() {
			err = os.RemoveAll(fullPath)
			if err != nil {
				return err
			}
		} else {
			// 这里就是文件了
			err = os.Remove(fullPath)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// GetTmpFolderByName 获取缓存的文件夹，没有则新建
func GetTmpFolderByName(nowProcessRoot, folderName string) string {
	rootPath := GetRodTmpRootFolder(nowProcessRoot)
	tmpFolderFullPath := filepath.Join(rootPath, folderName)
	_ = os.MkdirAll(tmpFolderFullPath, os.ModePerm)
	return tmpFolderFullPath
}

// GetADBlockFolder 在程序的根目录新建，adblock 缓存用文件夹
func GetADBlockFolder(nowProcessRoot string) string {

	if nowProcessRoot == "" {
		nowProcessRoot = "."
	}
	nowProcessRoot = filepath.Join(nowProcessRoot, PluginFolder, ADBlockFolder)
	err := os.MkdirAll(nowProcessRoot, os.ModePerm)
	if err != nil {
		logger.Panicln(err)
	}
	return nowProcessRoot
}

// GetProxyCacheFolder 代理索引缓存目录
func GetProxyCacheFolder(nowProcessRoot string) string {

	if nowProcessRoot == "" {
		nowProcessRoot = "."
	}
	nowProcessRoot = filepath.Join(nowProcessRoot, ProxyCacheFolder)
	err := os.MkdirAll(nowProcessRoot, os.ModePerm)
	if err != nil {
		logger.Panicln(err)
	}
	return nowProcessRoot
}

// GetADBlockUnZipFolder 在程序的根目录新建，adblock 缓存用文件夹
func GetADBlockUnZipFolder(nowProcessRoot string) string {

	if nowProcessRoot == "" {
		nowProcessRoot = "."
	}
	nowProcessRoot = GetADBlockFolder(nowProcessRoot)
	nowProcessRoot = filepath.Join(nowProcessRoot, ADBlockUnZipFolder)
	err := os.MkdirAll(nowProcessRoot, os.ModePerm)
	if err != nil {
		logger.Panicln(err)
	}
	return nowProcessRoot
}

// 缓存文件的位置信息，都是在程序的根目录下的 cache 中
const (
	RodCacheFolder     = "rod"           // rod 的缓存目录
	PluginFolder       = "Plugin"        // 插件的目录
	ADBlockFolder      = "adblock"       // adblock
	ADBlockUnZipFolder = "adblock_unzip" // adblock unzip
	ProxyCacheFolder   = "proxy_cache"   // 代理索引缓存目录
)
