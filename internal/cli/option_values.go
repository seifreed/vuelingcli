package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	dateparse "github.com/seifreed/vuelingcli/internal/date"
)

func parseOptionalDate(value string, now time.Time) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	return dateparse.ParseDate(value, now)
}

func parseIntOption(value string, fallback int) (int, error) {
	if value == "" {
		return fallback, nil
	}
	return strconv.Atoi(value)
}

func setOutputFormat(current, option string) (string, error) {
	requested := option[2:]
	if current != "" && current != requested {
		return "", fmt.Errorf("--json and --toon cannot be used together")
	}
	return requested, nil
}

func parseLongOption(args []string, index int) (key, value string, next int, err error) {
	key, value, hasValue := strings.Cut(args[index][2:], "=")
	if hasValue {
		return key, value, index, nil
	}
	if index+1 == len(args) {
		return "", "", index, fmt.Errorf("--%s requires a value", key)
	}
	return key, args[index+1], index + 1, nil
}
