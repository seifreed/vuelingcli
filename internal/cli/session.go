package cli

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/seifreed/vuelingcli/internal/vueling"
)

const sessionUsage = "usage: vueling session [refresh|clear] [--json|--toon]"

type sessionStatus struct {
	Active      bool   `json:"active"`
	ExpiresAt   string `json:"expiresAt,omitempty"`
	MinutesLeft int    `json:"minutesLeft"`
}

func (cli *CLI) session(args []string) error {
	if isHelp(args) {
		_, err := fmt.Fprint(cli.stdout, "Show, extend, or end the anonymous Vueling session the CLI reuses between searches\n\nUsage:\n  vueling session [--json|--toon]           Show whether the session is active and when it expires\n  vueling session refresh [--json|--toon]   Extend it now, or open a new one\n  vueling session clear                     End it and forget it\n")
		return err
	}
	action, format := "", ""
	for _, a := range args {
		switch {
		case a == "--json" || a == "--toon":
			requested, err := setOutputFormat(format, a)
			if err != nil {
				return err
			}
			format = requested
		case action == "" && (a == "refresh" || a == "clear"):
			action = a
		default:
			return errors.New(sessionUsage)
		}
	}
	if err := cli.requireAPI(); err != nil {
		return err
	}
	session := vueling.Session{}
	switch action {
	case "refresh":
		refreshed, err := cli.api.RefreshSession(context.Background())
		if err != nil {
			return err
		}
		session = refreshed
	case "clear":
		if format != "" {
			return errors.New(sessionUsage)
		}
		if err := cli.api.ClearSession(context.Background()); err != nil {
			return err
		}
		_, err := fmt.Fprintln(cli.stdout, "Anonymous Vueling session cleared")
		return err
	default:
		stored, err := cli.api.StoredSession()
		if err != nil {
			return err
		}
		session = stored
	}
	status := describeSession(session, time.Now())
	if format != "" {
		return cli.writeStructured(status, format)
	}
	if !status.Active {
		_, err := fmt.Fprintln(cli.stdout, "No active anonymous Vueling session")
		return err
	}
	_, err := fmt.Fprintf(cli.stdout, "Anonymous Vueling session active until %s (%d min left)\n", session.Expires.Local().Format("15:04:05"), status.MinutesLeft)
	return err
}

func describeSession(session vueling.Session, now time.Time) sessionStatus {
	if !session.Active(now) {
		return sessionStatus{}
	}
	return sessionStatus{Active: true, ExpiresAt: session.Expires.Local().Format(time.RFC3339), MinutesLeft: int(session.Expires.Sub(now).Minutes())}
}
