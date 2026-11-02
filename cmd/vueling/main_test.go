package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/seifreed/vuelingcli/internal/vueling/api"
)

func TestMainConfiguresAPIAndExitsWithCommandStatus(t *testing.T) {
	if newPageReader("") == nil {
		t.Fatal("newPageReader returned nil")
	}

	oldArgs, oldStdout := os.Args, os.Stdout
	oldNewAPIClient, oldExitProcess := newAPIClient, exitProcess
	t.Cleanup(func() {
		os.Args, os.Stdout = oldArgs, oldStdout
		newAPIClient, exitProcess = oldNewAPIClient, oldExitProcess
	})

	for _, tc := range []struct {
		name string
		err  error
	}{
		{name: "API client created"},
		{name: "API client unavailable", err: errors.New("offline")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reader, writer, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = reader.Close() }()
			os.Stdout = writer
			os.Args = []string{"vueling", "version"}
			created := false
			newAPIClient = func() (*api.Client, error) {
				created = true
				if tc.err == nil {
					return &api.Client{}, nil
				}
				return nil, tc.err
			}
			exitCode := -1
			exitProcess = func(code int) { exitCode = code }

			main()
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			os.Stdout = oldStdout
			var output bytes.Buffer
			if _, err := io.Copy(&output, reader); err != nil {
				t.Fatal(err)
			}
			if !created || exitCode != 0 || output.String() != "vueling 0.1.0\n" {
				t.Fatalf("main: created=%v exit=%d output=%q", created, exitCode, output.String())
			}
		})
	}
}

func TestMainReportsInvalidLoggerConfigAndFailedExit(t *testing.T) {
	t.Setenv("VUELING_ENV", "production")
	t.Setenv("VUELING_LOG_LEVEL", "trace-private-value")
	t.Setenv("VUELING_LOG_FORMAT", "xml-private-value")
	oldArgs, oldStderr := os.Args, os.Stderr
	oldNewAPIClient, oldExitProcess := newAPIClient, exitProcess
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = writer
	os.Args = []string{"vueling", "not-a-command"}
	newAPIClient = func() (*api.Client, error) { return &api.Client{}, nil }
	exitCode := -1
	exitProcess = func(code int) { exitCode = code }
	t.Cleanup(func() {
		os.Args, os.Stderr = oldArgs, oldStderr
		newAPIClient, exitProcess = oldNewAPIClient, oldExitProcess
		_ = reader.Close()
		_ = writer.Close()
	})

	main()
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stderr = oldStderr
	var output bytes.Buffer
	if _, err := io.Copy(&output, reader); err != nil {
		t.Fatal(err)
	}
	if exitCode != 2 {
		t.Fatalf("exit code = %d, want 2; stderr=%s", exitCode, output.String())
	}
	for _, event := range []string{`"event_name":"logging.configuration.invalid"`, `"event_name":"process.stopped"`, `"outcome":"error"`} {
		if !bytes.Contains(output.Bytes(), []byte(event)) {
			t.Errorf("stderr is missing %q: %s", event, output.String())
		}
	}
	for _, value := range []string{"trace-private-value", "xml-private-value"} {
		if bytes.Contains(output.Bytes(), []byte(value)) {
			t.Errorf("stderr exposed invalid config value %q: %s", value, output.String())
		}
	}
}
