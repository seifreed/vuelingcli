# vuelingcli

CLI no oficial para consultar precios y trayectos de Vueling desde la terminal usando la sesión local de Chrome, pensado también para agentes.

La herramienta es de solo lectura: `flights` lee las ofertas renderizadas en Vueling a través del endpoint local de Chrome; los demás comandos generan enlaces. No reserva, no paga ni modifica viajes.

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
go run ./cmd/vueling flights BCN MLA --date 14/01/2027 --return 28/01/2027 --json
```

`flights` requiere el acceso local de DevTools habilitado para el perfil de Chrome que contiene la sesión de Vueling. El endpoint debe permanecer ligado a `localhost`/loopback porque permite controlar el navegador. En Chrome 136 o posterior, el puerto de depuración por línea de comandos no funciona con el perfil de datos predeterminado; no copies ese perfil como workaround. El comando abre una pestaña de búsqueda y extrae solo el texto visible de las ofertas; no lee cookies, tokens ni APIs privadas y no selecciona tarifas. Usa `--cdp-url` para especificar otro endpoint local.

`calendar` builds Vueling's public price-calendar URL. It accepts `YYYY-MM-DD`, `DD/MM/YYYY`, `today`, `tomorrow`, `+N`, or a weekday name in English or Spanish; add `--open` to open it in your default browser. `flights` reads current offers and itinerary text from the rendered page. Use `--json` or `--toon` for structured output. `search` and `link` build the official flight-selection URL; routes accept IATA codes or unambiguous city names from the local catalogue (quote multi-word names). Add `--open` to open a link in your default browser. They support `--discount`, `--culture`, `--currency`, `--flight`, `--fare-basis`, `--return-flight`, `--return-fare-basis`, and `--token` for extended links. `airports` searches a local snapshot of Vueling's [airport-help pages](https://help.vueling.com/hc/es/categories/19798729418513-Aeropuertos) and destination pages; the list can lag route changes and supports `--json` or `--toon`.

## Skill para agentes

`.claude/skills/vueling-flights/` documenta cómo buscar en el navegador y usar la CLI para generar enlaces. La [referencia de CLI](.claude/skills/vueling-flights/references/cli-reference.md) lista cada opción, formato de fecha y esquema de salida.

Sin afiliación con Vueling ni respaldo por su parte.
