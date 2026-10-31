# vuelingcli

CLI no oficial para consultar el calendario público de precios de Vueling desde la terminal, pensado también para agentes.

La herramienta es de solo lectura: genera enlaces públicos, pero no reserva, no paga ni modifica viajes.

## Instalación

Requiere Go 1.24 o posterior.

```sh
go install github.com/seifreed/vuelingcli/cmd/vueling@latest
vueling help
```

## Desarrollo

```sh
go run ./cmd/vueling help
go run ./cmd/vueling version
go run ./cmd/vueling calendar BCN MLA --date 14/01/2027
```

`calendar` builds Vueling's public price-calendar URL. It accepts `YYYY-MM-DD`, `DD/MM/YYYY`, `today`, `tomorrow`, `+N`, or a weekday name in English or Spanish; add `--open` to open it in your default browser. Use `--json` or `--toon` for structured output. `search` and `link` build the same official flight-selection URL from route, date, and passenger options; routes accept IATA codes or unambiguous city names from the local catalogue (quote multi-word names). Add `--open` to open the link in your default browser and see current offers. They support `--discount`, `--culture`, `--currency`, `--flight`, `--fare-basis`, `--return-flight`, `--return-fare-basis`, and `--token` for extended links. These commands do not fetch or rank live prices. `airports` searches a local snapshot of Vueling's [airport-help pages](https://help.vueling.com/hc/es/categories/19798729418513-Aeropuertos) and destination pages; the list can lag route changes and supports `--json` or `--toon`.

## Skill para agentes

`.claude/skills/vueling-flights/` documenta cómo buscar en el navegador y usar la CLI para generar enlaces. La [referencia de CLI](.claude/skills/vueling-flights/references/cli-reference.md) lista cada opción, formato de fecha y esquema de salida.

Sin afiliación con Vueling ni respaldo por su parte.
