# Vueling CLI reference

The CLI is read-only. `flights` reads current offers from Vueling's rendered page through a local Chrome DevTools session. `search` and `link` construct the official flight-selection URL; `calendar` constructs the public price-calendar URL.

## Date syntax

All date options accept:

- `YYYY-MM-DD` or `DD/MM/YYYY`
- `today`, `tomorrow`, or `+N` days (`+0` means today)
- English weekdays: `sun`/`sunday`, `mon`/`monday`, `tue`/`tuesday`, `wed`/`wednesday`, `thu`/`thursday`, `fri`/`friday`, `sat`/`saturday`
- Spanish weekdays: `domingo`, `lunes`, `martes`, `miércoles`/`miercoles`, `jueves`, `viernes`, `sábado`/`sabado`

A weekday matching today means the following week.
Past calendar dates are rejected.

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

### `vueling flights`

```text
vueling flights <origin city-or-IATA> <destination city-or-IATA> --date <date> [--return <date>] [--adults N] [--children N] [--infants N] [--currency CODE] [--cdp-url http://127.0.0.1:9222] [--json|--toon]
```

Opens the search in a new tab in the Chrome profile exposed on the local DevTools endpoint, waits for Vueling to render offers, and reads the visible flight cards. Remote debugging must be enabled for the Chrome profile containing the Vueling session; keep the endpoint bound to loopback because it grants control over the browser. `--cdp-url` accepts only localhost or a loopback IP. Chrome 136+ ignores the command-line port switch for its default data directory, so do not copy the user's profile as a workaround; use Chrome's explicit remote-debugging controls when available or a dedicated profile the user signs into. See [Chrome's remote-debugging changes](https://developer.chrome.com/blog/remote-debugging-port). The command reads no cookies, storage, tokens, or network requests and does not select any fare. A CAPTCHA or missing/expired Vueling session prevents results from loading.

The JSON/TOON object contains `url` and `flights`; each flight has `direction`, `date`, `flight`, `departure`, `arrival`, `fare`, `currency`, and the visible `itinerary` text. Currency defaults to `EUR` and uses the same supported values as `search`/`link`.

### `vueling airports`

```text
vueling airports [city-or-code] [--json|--toon]
```

Searches a local snapshot based on [Vueling's airport-help pages](https://help.vueling.com/hc/es/categories/19798729418513-Aeropuertos) and public destination pages by city name or three-letter code. Quote multi-word city names. Omitting the query prints the whole catalogue. It can lag route changes and makes no network request.

### `vueling version` and help

`vueling version` (also `--version` or `-v`) prints the CLI version. Add `--json` or `--toon` for structured output. `vueling help`, `--help`, or `-h` prints command names. Use `vueling version --help`, `vueling flights --help`, `vueling search --help`, `vueling link --help`, `vueling calendar --help`, or `vueling airports --help` for command usage.

## Structured output

`search`, `link`, and `calendar` return a JSON object with one `url` field; `flights` returns `url` and `flights`; `version` returns a `version` field. Both use the same shape in TOON format:

```json
{"url":"https://tickets.vueling.com/booking?..."}
```

`airports` returns an array of `{ "code", "city" }` objects; no matches is an empty array. Human-readable text goes to stdout and usage errors go to stderr. Invalid arguments exit with code `2`.

`flights` returns offers in the order displayed by Vueling. Ranking and filters such as departure time or direct-only are not implemented.
