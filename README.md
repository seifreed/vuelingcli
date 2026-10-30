# vuelingcli

CLI no oficial para consultar el calendario público de precios de Vueling desde la terminal, pensado también para agentes.

La herramienta es de solo lectura: genera enlaces públicos, pero no reserva, no paga ni modifica viajes.

## Desarrollo

```sh
go run ./cmd/vueling help
go run ./cmd/vueling version
go run ./cmd/vueling calendar BCN MLA --date 14/01/2027
```

`calendar` builds Vueling's public price-calendar URL. It accepts `YYYY-MM-DD`, `DD/MM/YYYY`, `today`, `tomorrow`, `+N`, or a weekday name in English or Spanish. Use `--json` or `--toon` for structured output. `link` builds the official flight-selection URL from route, date, and passenger options. It supports `--discount`, `--culture`, `--currency`, `--flight`, `--fare-basis`, `--return-flight`, `--return-fare-basis`, and `--token` for extended links. `airports` searches a local catalogue of Vueling airports; it is not the full route network. `search` is not available because the official Flight Calendar Prices API requires access from Vueling.

## Skill para agentes

`.claude/skills/vueling-flights/` documenta cómo usar la CLI y sus límites para agentes.

Sin afiliación con Vueling ni respaldo por su parte.
