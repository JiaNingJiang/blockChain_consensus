//some help function to handle path

package common

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
)

// MakeName creates a node name that follows the dpchain convention
// for such names. It adds the operation system name and Go runtime version
// the name.
func MakeName(name, version string) string {
	return fmt.Sprintf("%s/v%s/%s/%s", name, version, runtime.GOOS, runtime.Version())
}

func ExpandHomePath(p string) (path string) {
	path = p
	// sep := fmt.Sprintf("%s", os.PathSeparator)
	sep := string(os.PathSeparator)

	// Check in case of paths like "/something/~/something/"
	if len(p) > 1 && p[:1+len(sep)] == "~"+sep {
		usr, _ := user.Current()
		dir := usr.HomeDir

		path = strings.Replace(p, "~", dir, 1)
	}

	return
}

func FileExist(filePath string) bool {
	_, err := os.Stat(filePath)
	if err != nil && os.IsNotExist(err) {
		return false
	}

	return true
}

func AbsolutePath(Datadir string, filename string) string {
	if filepath.IsAbs(filename) {
		return filename
	}
	return filepath.Join(Datadir, filename)
}

func IsWindows() bool {
	return runtime.GOOS == "windows"
}

func WindonizePath(path string) string {
	if string(path[0]) == "/" && IsWindows() {
		path = path[1:]
	}
	return path
}

func GetExecutePath() string {
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(dir)

	return dir
}

func DefaultDataDir() string {
	usr, _ := user.Current()
	if runtime.GOOS == "darwin" {
		return filepath.Join(usr.HomeDir, "Library", "Ethereum")
	} else if runtime.GOOS == "windows" {
		return filepath.Join(usr.HomeDir, "AppData", "Roaming", "DP-Chain")
	} else {
		return filepath.Join(usr.HomeDir, ".dpchain")
	}
}
