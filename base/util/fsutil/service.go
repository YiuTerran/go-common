package fsutil

import (
	"github.com/YiuTerran/go-common/base/log"
	"os"
	"path/filepath"
)

/**
  *  @author tryao
  *  @date 2022/03/21 15:17
**/
const (
	ConfigDir   = "_config"
	ResourceDir = "resource"
)

// FindPathFrom 从某个目录开始逐级向上查找name指向的文件
func FindPathFrom(root string, name string) string {
	dir := root
	prev := ""
	if root == "" {
		return FindPath(name)
	}
	x := filepath.Join(dir, name)
	for !Exists(x) {
		if dir == prev {
			log.Error("can't find path from %s, it should be named `%s`", root, name)
			return ""
		}
		prev = dir
		dir = filepath.Dir(dir)
		x = filepath.Join(dir, name)
	}
	return x
}

// FindPath 从程序所在目录逐层往上找直到找到name指向的文件
//注意：go run的执行文件在临时目录，一般是(/var/folders/)，这种方法是找不到的
func FindPath(name string) string {
	dir, _ := os.Executable()
	root := dir
	return FindPathFrom(root, name)
}
