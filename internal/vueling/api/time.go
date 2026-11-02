package api

import "time"

const apiTimeLayout = "2006-01-02T15:04:05"

func parseAPITime(value string) (time.Time, error) {
	return time.Parse(apiTimeLayout, value)
}
