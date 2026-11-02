package cli

import (
	"encoding/json"
	"fmt"

	toon "github.com/toon-format/toon-go"
)

func (cli *CLI) writeStructured(value any, format string) error {
	switch format {
	case "json":
		return json.NewEncoder(cli.stdout).Encode(value)
	case "toon":
		encoded, err := encodeTOON(value)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(cli.stdout, encoded)
		return err
	default:
		return fmt.Errorf("unsupported output format %q", format)
	}
}

func encodeTOON(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return encodeJSONTOON(raw)
}

func encodeJSONTOON(raw []byte) (string, error) {
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return "", err
	}
	return toon.MarshalString(generic)
}
