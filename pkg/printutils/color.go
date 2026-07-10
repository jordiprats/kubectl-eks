package printutils

import (
	"fmt"
	"os"

	"golang.org/x/term"
)

const (
	ColorReset   = "\033[0m"
	ColorBold    = "\033[1m"
	ColorRed     = "\033[31m"
	ColorGreen   = "\033[32m"
	ColorYellow  = "\033[33m"
	ColorMagenta = "\033[35m"
)

var isTTY *bool

func IsTTY() bool {
	if isTTY == nil {
		result := term.IsTerminal(int(os.Stdout.Fd()))
		isTTY = &result
	}
	return *isTTY
}

func Colorize(text, color string) string {
	if !IsTTY() {
		return text
	}
	return color + text + ColorReset
}

func ClearScreen() {
	fmt.Print("\033[2J\033[H")
}
