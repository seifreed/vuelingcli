package cli

import "testing"

func TestParseLongOption(t *testing.T) {
	for _, test := range []struct {
		args       []string
		key, value string
		next       int
	}{
		{[]string{"--date=2026-12-31"}, "date", "2026-12-31", 0},
		{[]string{"--return", "+10"}, "return", "+10", 1},
	} {
		key, value, next, err := parseLongOption(test.args, 0)
		if err != nil || key != test.key || value != test.value || next != test.next {
			t.Errorf("parseLongOption(%v) = %q, %q, %d, %v; want %q, %q, %d, nil", test.args, key, value, next, err, test.key, test.value, test.next)
		}
	}
	if _, _, _, err := parseLongOption([]string{"--return"}, 0); err == nil || err.Error() != "--return requires a value" {
		t.Fatalf("missing option value error = %v", err)
	}
}

func TestParseLongOptionRejectsEmptyValues(t *testing.T) {
	for _, args := range [][]string{{"--return="}, {"--return", "--json"}} {
		if _, _, _, err := parseLongOption(args, 0); err == nil || err.Error() != "--return requires a value" {
			t.Errorf("parseLongOption(%v) error = %v; want a missing-value error", args, err)
		}
	}
}

func TestSetOutputFormat(t *testing.T) {
	format, err := setOutputFormat("", "--json")
	if err != nil {
		t.Fatal(err)
	}
	format, err = setOutputFormat(format, "--json")
	if err != nil || format != "json" {
		t.Fatalf("repeating the same format = %q, %v; want json, nil", format, err)
	}
	if _, err := setOutputFormat(format, "--toon"); err == nil {
		t.Fatal("conflicting output format accepted")
	}
}

func TestParsePositivePrice(t *testing.T) {
	for value, want := range map[string]float64{"39.99": 39.99, "39,99": 39.99, "1": 1} {
		got, err := parsePositivePrice(value, "--max")
		if err != nil || got != want {
			t.Errorf("parsePositivePrice(%q) = %v, %v; want %v, nil", value, got, err, want)
		}
	}
	for _, value := range []string{"0", "-1", "NaN", "Inf", "-Inf", "cheap", "1e999"} {
		if _, err := parsePositivePrice(value, "--max"); err == nil || err.Error() != "--max must be a positive price" {
			t.Errorf("parsePositivePrice(%q) error = %v", value, err)
		}
	}
	if _, err := parsePositivePrice("0", "--below"); err == nil || err.Error() != "--below must be a positive price" {
		t.Fatalf("--below error = %v", err)
	}
}

func TestParseNonNegativeLimit(t *testing.T) {
	for _, test := range []struct {
		value    string
		fallback int
		want     int
	}{
		{value: "", fallback: 0, want: 0},
		{value: "", fallback: 10, want: 10},
		{value: "0", fallback: 10, want: 0},
		{value: "3", fallback: 10, want: 3},
	} {
		got, err := parseNonNegativeLimit(test.value, test.fallback)
		if err != nil || got != test.want {
			t.Errorf("parseNonNegativeLimit(%q, %d) = %d, %v; want %d, nil", test.value, test.fallback, got, err, test.want)
		}
	}
	for _, value := range []string{"-1", "many"} {
		if _, err := parseNonNegativeLimit(value, 0); err == nil || err.Error() != "--limit must be a non-negative whole number" {
			t.Errorf("parseNonNegativeLimit(%q) error = %v", value, err)
		}
	}
}
