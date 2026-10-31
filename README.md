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

`calendar` builds Vueling's public price-calendar URL. It accepts `YYYY-MM-DD`, `DD/MM/YYYY`, `today`, `tomorrow`, `+N`, or a weekday name in English or Spanish. Use `--json` or `--toon` for structured output. `link` builds the official flight-selection URL from route, date, and passenger options. It supports `--discount`, `--culture`, `--currency`, `--flight`, `--fare-basis`, `--return-flight`, `--return-fare-basis`, and `--token` for extended links. `airports` searches a local catalogue of Vueling airports and supports `--json` or `--toon`; it is not the full route network. `search` does not fetch live prices; use the Vueling website in a browser for current results.

## Skill para agentes

`.claude/skills/vueling-flights/` documenta cómo buscar en el navegador y usar la CLI para generar enlaces.

Sin afiliación con Vueling ni respaldo por su parte.
