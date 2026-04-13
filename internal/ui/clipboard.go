package ui

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// copyToClipboard 写入系统剪贴板（macOS: pbcopy；Linux: wl-copy / xclip；Windows: clip）。
func copyToClipboard(text string) error {
	text = strings.TrimRight(normalizeCRLF(text), "\n")
	switch runtime.GOOS {
	case "darwin":
		cmd := exec.Command("pbcopy")
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("pbcopy: %w", err)
		}
		return nil
	case "windows":
		cmd := exec.Command("clip")
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("clip: %w", err)
		}
		return nil
	default:
		if path, err := exec.LookPath("wl-copy"); err == nil {
			cmd := exec.Command(path)
			cmd.Stdin = strings.NewReader(text)
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("wl-copy: %w", err)
			}
			return nil
		}
		if path, err := exec.LookPath("xclip"); err == nil {
			cmd := exec.Command(path, "-selection", "clipboard")
			cmd.Stdin = strings.NewReader(text)
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("xclip: %w", err)
			}
			return nil
		}
		return fmt.Errorf("未找到 wl-copy 或 xclip，请安装其一以使用复制")
	}
}
