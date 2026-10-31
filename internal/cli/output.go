package cli

import (
	"encoding/json"
	"fmt"

	toon "github.com/toon-format/toon-go"
)

func (cli *CLI) writeTOON(value any) error {
	encoded, err := encodeTOON(value)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(cli.stdout, encoded)
	return err
}

func encodeTOON(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return "", err
	}
	return toon.MarshalString(generic)
}
