package browser

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

func readPageWithAppleEvents(ctx context.Context, pageURL, pageScript string) ([]byte, error) {
	const script = `on run argv
	set searchURL to item 1 of argv
	set pageScript to item 2 of argv
	set lastError to ""
	tell application "Google Chrome"
		if (count of windows) is 0 then error "VUELING_NO_CHROME_WINDOW"
		set resultTab to make new tab at end of tabs of front window with properties {URL:searchURL}
		repeat 90 times
			try
				set resultJSON to execute resultTab javascript ("JSON.stringify(" & pageScript & ")")
				if resultJSON is not missing value and resultJSON is not "[]" then
					close resultTab
					return resultJSON
				end if
				set lastError to ""
			on error errorText
				if errorText contains "Executing JavaScript through AppleScript is turned off" then error "VUELING_JS_DISABLED"
				set lastError to errorText
			end try
			delay 0.5
		end repeat
		if lastError is not "" then error "VUELING_SCRIPT_ERROR:" & lastError
		error "VUELING_NO_OFFERS"
	end tell
end run`
	cmd := exec.CommandContext(ctx, "osascript", "-e", script, pageURL, pageScript)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, appleEventsFailure(output, err)
	}
	return []byte(strings.TrimSpace(string(output))), nil
}

func appleEventsFailure(output []byte, cause error) error {
	message := strings.ToLower(string(output))
	switch {
	case strings.Contains(message, "vueling_js_disabled"):
		return fmt.Errorf("Chrome blocks JavaScript from Apple Events; enable View > Developer > Allow JavaScript from Apple Events, then retry: %w", cause)
	case strings.Contains(message, "vueling_no_chrome_window"):
		return fmt.Errorf("Google Chrome has no open window: %w", cause)
	case strings.Contains(message, "vueling_no_offers"):
		return fmt.Errorf("Vueling did not render results; the tab was left open in Chrome so you can check it, and a manual search there shows whether Vueling is unavailable or the session needs attention. Retry later rather than in a loop: %w", cause)
	case strings.Contains(string(output), "VUELING_SCRIPT_ERROR:"):
		return fmt.Errorf("Vueling page script failed: %w", cause)
	default:
		return fmt.Errorf("could not read the Vueling tab through Chrome Apple Events: %w", cause)
	}
}
