package ui

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// openBrowserURL 在系统默认浏览器中打开链接（非阻塞）。
func openBrowserURL(raw string) error {
	u := strings.TrimSpace(raw)
	if u == "" {
		return fmt.Errorf("链接为空")
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", u)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", u)
	default:
		path, err := exec.LookPath("xdg-open")
		if err != nil {
			return fmt.Errorf("未找到 xdg-open，无法打开浏览器")
		}
		cmd = exec.Command(path, u)
	}
	return cmd.Start()
}
