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
