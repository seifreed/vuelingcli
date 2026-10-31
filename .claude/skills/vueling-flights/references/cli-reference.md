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
vueling calendar <origin city-or-IATA> <destination city-or-IATA> --date <date> [--return <date>] [--prices [--month]] [--open] [--json|--toon]
```

Builds a URL for Vueling's public price calendar. With `--prices`, reads the requested outbound date's fare and, when `--return` is supplied, the return fare from the rendered calendar in the current Chrome profile (macOS) or local DevTools session. It can switch among the return calendar's visible month tabs; if the requested month is outside those tabs, it reports that instead of guessing a fare. Calendar price lookup limits return dates to 330 days after departure to avoid confusing the same month in another year. Structured output contains `route`, `date`, `fare`, `returnRoute`, `returnDate`, `returnFare`, and `url`; return fields are empty when no return date is requested. Add `--month` (only with `--prices`) to list every visible day fare of the outbound month, and of the return month with `--return`, sorted by date; structured output adds `days` and `cheapest` (each day is `{ "date", "fare" }`, dates as `DD/MM/YYYY`) plus `returnDays` and `returnCheapest` with `--return`, and `fare` is omitted when the requested day has no fare. Text output prints one `DATE: FARE` line per day under each route and marks the lowest with `[cheapest]` (the earliest day on ties). `--month` without `--prices` is a usage error. Calendar fares are not final booking quotes.

### `vueling flights`

```text
vueling flights <origin city-or-IATA> <destination city-or-IATA> --date <date> [--return <date>] [--adults N] [--children N] [--infants N] [--currency CODE] [--cdp-url http://127.0.0.1:9222] [--cheapest] [--direct] [--after HH:MM] [--before HH:MM] [--return-after HH:MM] [--return-before HH:MM] [--limit N] [--nearby] [--json|--toon]
```

On macOS, opens a search tab in the current Chrome profile, submits the visible search form, and reads its rendered flight cards through Apple Events. Chrome requires **View > Developer > Allow JavaScript from Apple Events**; that setting permits scripts to run in pages, so enable it only with the user's explicit approval. On other systems, the command uses Chrome's local DevTools endpoint. That endpoint grants browser control and must stay on loopback; `--cdp-url` accepts only localhost or a loopback IP. Chrome 136+ ignores the command-line port switch for its default data directory, so do not copy the user's profile as a workaround. See [Chrome's remote-debugging changes](https://developer.chrome.com/blog/remote-debugging-port). The command reads no cookies, storage, tokens, or network requests and does not select any fare. A CAPTCHA, expired session, or Vueling service outage can prevent results from loading.

The JSON/TOON object contains `url` and `flights`; each flight has `direction`, `date`, `flight`, `departure`, `arrival`, `fare`, `currency`, the visible `itinerary` text, and connection legs derived from that text: `stops` (number of connections, `0` for direct), `flights` (every VY flight number on the card, in order, deduplicated; the first equals `flight`), and `via` (connection airports the card names, e.g. `["BCN"]` from "Conexión en BCN"; omitted when none is named). Layover duration and per-leg times are not parsed; read them from `itinerary`. Human-readable output adds `1 escala vía BCN · VY1383+VY1050 |` before the itinerary of connecting offers and leaves direct offers unchanged. Currency defaults to `EUR` and uses the same supported values as `search`/`link`.

Ranking and filters apply to outbound and return offers independently:

| Option | Meaning |
| --- | --- |
| `--cheapest` | Sort by numeric fare, lowest first; ties keep Vueling's order and unparseable fares go last. |
| `--direct` | Keep offers with `stops` = 0: no `escala`/`conexión`/`stop` text and a single VY flight number. This is a heuristic over the rendered card text. |
| `--after HH:MM` | Keep departures at or after this time. |
| `--before HH:MM` | Keep departures at or before this time; must not be earlier than `--after`. |
| `--return-after HH:MM` `--return-before HH:MM` | Replace `--after`/`--before` for return offers; need `--return`. |
| `--limit N` | Keep the first `N` offers per direction after filtering and sorting (`N` ≥ 1). |

Example: `vueling flights BCN MLA --date +7 --return +14 --direct --after 08:00 --cheapest --limit 3 --json`.

`--nearby` also reads the public price calendar (like `calendar --prices --month`) after the offers and reports the cheapest day of the outbound month, and of the return month with `--return` (earliest day on ties). Text output ends with `cheapest nearby: DD/MM/YYYY from FARE` and `cheapest nearby return: DD/MM/YYYY from FARE`; JSON/TOON adds `nearby` and `returnNearby` objects `{ "date", "fare" }`. Without `--nearby` the output is unchanged. If the calendar cannot be read, a warning goes to stderr, those fields are omitted, and the command still exits 0 with the offers. Calendar fares are not final booking quotes.

### `vueling status`

```text
vueling status <VY flight number> [--date <date>] [--watch <minutes>] [--cdp-url http://127.0.0.1:9222] [--json|--toon]
vueling status <origin city-or-IATA> <destination city-or-IATA> [--date <date>] [--watch <minutes>] [--cdp-url http://127.0.0.1:9222] [--json|--toon]
```

Reads the live status of one Vueling flight, or of every Vueling flight on a route that day, from the public [flight-status page](https://www.vueling.com/es/servicios-vueling/informacion-de-vuelos/estado-de-vuelos). That page has no deep link, so the command fills its flight-number or origin/destination form in a new tab of the current Chrome profile (macOS Apple Events) or local DevTools session, reads the rendered cards, and closes the tab. `--date` defaults to `today`; the page only offers a few days around today and the command reports dates it disables. A search with no result for that day is reported as an error. Flight numbers may carry a letter suffix, such as `VY392D`, both in the output and as the query. The JSON/TOON value is an array of objects with `flight`, `date`, `status` (Vueling's text, such as `En Hora`), `origin`, `destination`, `departure`, `arrival`, and the visible card text in `details`. Vueling notes that this information is indicative and refreshes every 10 minutes.

`--watch <minutes>` keeps reading every N minutes (a whole number, at least `5`). It prints the first read, then only cards whose `status`, `departure`, or `arrival` changed. Text lines carry the local time (`HH:MM`) before the usual line; with `--json` each card is printed as one JSON object per line (NDJSON) instead of an array. It stops once every watched card is `Aterrizado`/`Landed` or `Cancelado`/`Cancelled` (case-insensitive), or on Ctrl-C/SIGTERM, exiting `0`. A failed first read exits as usual; later failed reads print a warning to stderr and watching continues. `--watch` with `--toon` or a value below 5 is a usage error.

### `vueling airports`

```text
vueling airports [city-or-code] [--json|--toon]
```

Searches a local snapshot based on [Vueling's airport-help pages](https://help.vueling.com/hc/es/categories/19798729418513-Aeropuertos) and public destination pages by city name or three-letter code. Quote multi-word city names. Omitting the query prints the whole catalogue. It can lag route changes and makes no network request.

### `vueling version` and help

`vueling version` (also `--version` or `-v`) prints the CLI version. Add `--json` or `--toon` for structured output. `vueling help`, `--help`, or `-h` prints command names. Use `vueling version --help`, `vueling flights --help`, `vueling status --help`, `vueling search --help`, `vueling link --help`, `vueling calendar --help`, or `vueling airports --help` for command usage.

## Structured output

`search`, `link`, and `calendar` return a JSON object with one `url` field; `flights` returns `url` and `flights`; `status` returns an array of flight statuses; `version` returns a `version` field. Both use the same shape in TOON format:

```json
{"url":"https://tickets.vueling.com/booking?..."}
```

`airports` returns an array of `{ "code", "city" }` objects; no matches is an empty array. Human-readable text goes to stdout and usage errors go to stderr. Invalid arguments exit with code `2`.

`flights` returns offers in the order displayed by Vueling unless `--cheapest` is given; `--direct`, `--after`, `--before`, and `--limit` narrow each direction independently.
