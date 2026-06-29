package output

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

var useJSON bool

func SetJSON(v bool) { useJSON = v }
func IsJSON() bool   { return useJSON }

func Success(msg string) {
	if !useJSON {
		fmt.Fprintf(os.Stdout, "\033[32m✓\033[0m %s\n", msg)
	}
}

func Step(msg string) {
	if !useJSON {
		fmt.Fprintf(os.Stdout, "  %s\n", msg)
	}
}

func Error(msg string) {
	fmt.Fprintf(os.Stderr, "\033[31m✗\033[0m %s\n", msg)
}

func Info(msg string) {
	if !useJSON {
		fmt.Fprintf(os.Stdout, "%s\n", msg)
	}
}

func Header(msg string) {
	if !useJSON {
		fmt.Fprintf(os.Stdout, "\n%s\n\n", msg)
	}
}

func JSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func FormatDuration(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	default:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
}
