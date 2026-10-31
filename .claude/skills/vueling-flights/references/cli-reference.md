# Vueling CLI reference

The CLI is read-only. `search` and `link` construct the same official flight-selection URL; `calendar` constructs the public price-calendar URL. None of these commands fetch fares or availability. Use `--open` to view the generated URL in the system's default browser.

## Date syntax

All date options accept:

- `YYYY-MM-DD` or `DD/MM/YYYY`
- `today`, `tomorrow`, or `+N` days (`+0` means today)
- English weekdays: `sun`/`sunday`, `mon`/`monday`, `tue`/`tuesday`, `wed`/`wednesday`, `thu`/`thursday`, `fri`/`friday`, `sat`/`saturday`
- Spanish weekdays: `domingo`, `lunes`, `martes`, `miércoles`/`miercoles`, `jueves`, `viernes`, `sábado`/`sabado`

A weekday matching today means the following week.

## Commands

### `vueling search` and `vueling link`

```text
vueling search <origin city-or-IATA> <destination city-or-IATA> --date <date> [options]
vueling link <origin city-or-IATA> <destination city-or-IATA> --date <date> [options]
```

Both commands build `https://tickets.vueling.com/booking` links. Use different three-letter IATA codes or an unambiguous city in the bundled airport catalogue. Quote multi-word city names, such as `"Palma de Mallorca"`. If a city matches several airports, use its IATA code.

| Option | Default | Meaning |
| --- | --- | --- |
| `--date <date>` | Required | Departure date |
| `--return <date>` | One way | Return date; must be on or after departure |
| `--adults N` | `1` | Adult passengers |
| `--children N` | `0` | Child passengers |
| `--infants N` | `0` | Infant passengers; cannot exceed adults |
| `--discount 0-5` | `0` | Vueling discount type |
| `--culture <code>` | `es-ES` | Search locale, such as `en-GB` |
| `--currency <code>` | `EUR` | `EUR`, `SEK`, `NOK`, `DKK`, `CHF`, `GBP`, or `USD` |
| `--flight VY1234` | None | Select an outbound flight number |
| `--fare-basis CODE` | None | Outbound fare basis; requires `--flight` |
| `--return-flight VY1234` | None | Select a return flight; requires `--return` |
| `--return-fare-basis CODE` | None | Return fare basis; requires `--return-flight` |
| `--token VALUE` | None | Vueling's optional `MET_Token` |
| `--open` | Off | Open the generated link in the default browser |
| `--json` / `--toon` | Text | Return the URL as structured output |

Passenger counts must total 1–9, include at least one adult, and have no more infants than adults. Fare-basis values and `MET_Token` must be alphanumeric. Flight numbers use `VY` followed by three or four digits.

Discount types: `0` none, `1` island/Ceuta resident, `2` large family, `3` special large family, `4` resident large family, `5` resident special large family.

### `vueling calendar`

```text
vueling calendar <origin city-or-IATA> <destination city-or-IATA> --date <date> [--open] [--json|--toon]
```

Builds a URL for Vueling's public price calendar. It accepts IATA codes or unambiguous city names from the bundled catalogue (quote multi-word names), plus departure date, browser-open, and output-format options; it does not query prices itself.

### `vueling airports`

```text
vueling airports [city-or-code] [--json|--toon]
```

Searches a local snapshot based on [Vueling's airport-help pages](https://help.vueling.com/hc/es/categories/19798729418513-Aeropuertos) and public destination pages by city name or three-letter code. Quote multi-word city names. Omitting the query prints the whole catalogue. It can lag route changes and makes no network request.

### `vueling version` and help

`vueling version` (also `--version` or `-v`) prints the CLI version. `vueling help`, `--help`, or `-h` prints command names. Use `vueling search --help`, `vueling link --help`, `vueling calendar --help`, or `vueling airports --help` for command usage.

## Structured output

`search`, `link`, and `calendar` return a JSON object with one `url` field, or the same data in TOON format:

```json
{"url":"https://tickets.vueling.com/booking?..."}
```

`airports` returns an array of `{ "code", "city" }` objects; no matches is an empty array. Human-readable text goes to stdout and usage errors go to stderr. Invalid arguments exit with code `2`.

## Not implemented

Live fares, availability, result ranking, and filters such as departure time or direct-only are not implemented by this CLI. As of 2026-10-31, Vueling's former Flight Calendar deeplink reference URL returns 404; no accessible public contract for requesting its price responses has been found, and direct requests from the CLI returned HTTP 403.
