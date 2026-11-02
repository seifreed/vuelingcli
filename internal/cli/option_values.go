package cli

import (
	"fmt"
	"math"
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

func parseNonNegativeLimit(value string, fallback int) (int, error) {
	limit, err := parseIntOption(value, fallback)
	if err != nil || limit < 0 {
		return 0, fmt.Errorf("--limit must be a non-negative whole number")
	}
	return limit, nil
}

func parsePositivePrice(value, option string) (float64, error) {
	price, err := strconv.ParseFloat(strings.Replace(value, ",", ".", 1), 64)
	if err != nil || !(price > 0) || math.IsInf(price, 0) {
		return 0, fmt.Errorf("%s must be a positive price", option)
	}
	return price, nil
}

func setOutputFormat(current, option string) (string, error) {
	requested := option[2:]
	if current != "" && current != requested {
		return "", fmt.Errorf("--json and --toon cannot be used together")
	}
	return requested, nil
}

func parseSearchTerms(args []string) (format string, terms []string, err error) {
	for _, arg := range args {
		switch {
		case arg == "--json" || arg == "--toon":
			format, err = setOutputFormat(format, arg)
			if err != nil {
				return "", nil, err
			}
		case strings.HasPrefix(arg, "-"):
			return "", nil, fmt.Errorf("unknown option %s", arg)
		default:
			terms = append(terms, arg)
		}
	}
	return format, terms, nil
}

func parseLongOption(args []string, index int) (key, value string, next int, err error) {
	key, value, hasValue := strings.Cut(args[index][2:], "=")
	if hasValue {
		if strings.TrimSpace(value) == "" {
			return "", "", index, fmt.Errorf("--%s requires a value", key)
		}
		return key, value, index, nil
	}
	if index+1 == len(args) || strings.HasPrefix(args[index+1], "--") {
		return "", "", index, fmt.Errorf("--%s requires a value", key)
	}
	return key, args[index+1], index + 1, nil
}
