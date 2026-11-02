# Vueling CLI reference

The CLI is read-only. `flights`, `calendar --prices`, `flights --nearby`, `deals`, `routes`, `alerts`, `fees`, and `status` call the JSON APIs behind Vueling's website (`apiw.vueling.com`, `ams.vueling.com`) over HTTPS with Chrome's TLS fingerprint and headers; they open no browser and read no cookies (`flights` uses the anonymous session Vueling gives any visitor). These APIs are undocumented and may change; `--browser` falls back to reading the rendered pages in the user's Chrome (see below). `search` and `link` construct the official flight-selection URL; `calendar` constructs the public price-calendar URL.

## Date syntax

All date options accept:

- `YYYY-MM-DD` or `DD/MM/YYYY`
- `today`/`hoy`, `tomorrow`/`mañana`, or `+N` days (`+0` means today)
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
vueling calendar <origin city-or-IATA> <destination city-or-IATA> --date <date> [--return <date>] [--prices [--month] [--currency CODE | --browser]] [--open] [--json|--toon]
vueling calendar <origin> <destination> --date <date> [--return <date>] --prices [--currency CODE] --watch <minutes> [--below <price>] [--json]
```

Builds a URL for Vueling's public price calendar. With `--prices`, reads the requested outbound date's fare and, when `--return` is supplied, the return fare from Vueling's fare-calendar API (about 11 months ahead, one cheapest fare per day; a day without a fare is an error). Route labels read `IDA: <CITY>` / `VUELTA: <CITY>`. `--currency` (same values as `search`/`link`, default `EUR`) prices the calendar in another currency, e.g. `57 GBP`. A route Vueling does not sell fails with `Vueling sells no fares from XXX to YYY`. `--browser` reads the rendered calendar in Chrome instead; it can only switch among the calendar's visible month tabs and reports months outside them. Calendar price lookup limits return dates to 330 days after departure to avoid confusing the same month in another year. Structured output contains `route`, `date`, `fare`, `returnRoute`, `returnDate`, `returnFare`, and `url`; return fields are empty when no return date is requested. Add `--month` (only with `--prices`) to list every visible day fare of the outbound month, and of the return month with `--return`, sorted by date; structured output adds `days` and `cheapest` (each day is `{ "date", "fare" }`, dates as `DD/MM/YYYY`) plus `returnDays` and `returnCheapest` with `--return`, and `fare` is omitted when the requested day has no fare. Text output prints one `DATE: FARE` line per day under each route and marks the lowest with `[cheapest]` (the earliest day on ties). `--month` without `--prices` is a usage error. Calendar fares are not final booking quotes.

Use `--open` to open the generated calendar URL in the default browser; it can be combined with `--prices` and `--month`.

**Price watch.** `--watch <minutes>` re-reads the fare every N minutes (at least 30; lower is a usage error). It watches the outbound fare, or outbound + return with `--return`, prints the first reading and then only changes as `HH:MM ROUTE DATES: old → new`, and stops on Ctrl-C. `--below <price>` marks the first reading at or under the price with `BELOW <price> <currency>` and exits 0, so it can drive scripts (`vueling calendar BCN BIO --date 2026-11-10 --prices --watch 60 --below 40 && notify`). `--json` prints one object per reading (`time`, `route`, `date`, `fare`, `returnDate`, `returnFare`, `total`, `currency`, `previous`, `threshold`, `below`, `url`). `--watch` needs `--prices` and cannot be combined with `--toon`, `--month` or `--browser`; `--below` requires `--watch`. Later read failures are printed to stderr and the watch continues.

### `vueling deals`

```text
vueling deals <origin city-or-IATA> [--to city-or-IATA[,...]] [--from <date>] [--months 1-12] [--max PRICE] [--direct] [--limit N] [--currency CODE] [--json|--toon]
```

Answers "where can I fly cheaply from here?". Reads the best-price API behind Vueling's "Ofertas desde" section (`apiw.vueling.com/api/v1/bestPrices`) and returns the cheapest one-way fare to every destination Vueling flies to from the origin, cheapest first. It queries every catalogued station by default (Vueling drops the ones it does not serve); `--to` restricts it to a comma-separated list of cities or codes. The search window starts at `--from` (default today) and spans `--months` (default 4, as on the website). `--max` keeps fares at or below a price, `--direct` keeps direct routes, and `--limit` caps the list. Text output prints `CODE City DD/MM/YYYY PRICE CURRENCY`; structured output is an array of `{ "destination", "city", "date" (YYYY-MM-DD), "price" (number), "currency", "direct", "calendarUrl" }`. Follow up with `vueling flights ORIGIN DEST --date DATE` for that day's flights. These are calendar fares, not final booking quotes.

### `vueling trips`

```text
vueling trips <origin> <destination> --nights N[-M] [--from <date>] [--to <date>] [--max PRICE] [--limit N] [--currency CODE] [--json|--toon]
```

Answers "which days are cheapest for a 3–5 night trip?". For each departure day from `--from` (default today) to `--to` (default `--from` + 30 days) it pairs the return day N…M nights later and prices it as the outbound day's cheapest calendar fare plus the return day's (per adult), cheapest total first (ties: earlier date, fewer nights). Two calls to the fare calendar, one per direction. Days without a fare are skipped; `--max` keeps totals up to a price; `--limit` defaults to 10 (0 = all). Returns must fall within the calendar horizon (about 11 months). There is no `--direct` (the calendar does not say). Text: `24/11/2026 → 29/11/2026  5 noches  out 25 + ret 31 = 56 EUR`. Structured: array of `{ "departure", "return" (DD/MM/YYYY), "nights", "outbound", "returnFare", "total", "currency", "url" }` (booking link with both dates). Calendar fares, not a quote: confirm a pair with `vueling select --date D --cheapest --return R --return-cheapest`.

### `vueling schedule`

```text
vueling schedule <origin> <destination> [--from <date>] [--months 1-12] [--days] [--currency CODE] [--json|--toon]
```

Shows on which days Vueling flies a route: for each month from `--from` (default today) over `--months` (default 3) the days with a one-way flight on sale, their weekdays ("mié sáb"), and the first/last flight dates. `--days` adds each day's cheapest flight number, departure time and "desde" fare, marking connections. Reads the date-picker availability (`POST ams.vueling.com/avy/v3/AvailabilityServices/allFlights`, anonymous session): one row per day (the cheapest flight), not a full timetable, and only up to Vueling's sales horizon (~11 months; later months show "sin vuelos"); sold-out days may not appear. A route Vueling doesn't fly prints "sin vuelos a la venta en estas fechas" (check `routes`). Structured: `{ origin, destination, from, to, firstDate, lastDate, weekdays, months: [{ month, days, weekdays }], dates: [{ date, weekday, flight, departure, fare, connection }] }`.

### `vueling routes`

```text
vueling routes <origin city-or-IATA> [--direct] [--country CODE-or-name] [--json|--toon]
```

Answers "where does Vueling fly from here?". Reads the route network the website's search form uses (`static.vueling.com/corporativeCMS/Stations/es-ES.json`) and lists every destination sold from the origin, sorted by code, marking those reached only with a connection. City groups such as `PAR` or `LON` are left out (their airports are listed). `--direct` keeps direct routes and `--country` filters by ISO code (`IT`) or Spanish country name (`Italia`). Text output prints `CODE Name Country directo|conexión`; structured output is an array of `{ "code", "name", "countryCode", "country", "direct", "residentDiscount", "largeFamilyDiscount" }`, where the last two say whether Spanish resident or large-family discounts apply on that route. The network has no prices or schedules; a listed route may be seasonal, so check `calendar --prices --month` or `deals --to` for fares.

### `vueling flights`

```text
vueling flights <origin city-or-IATA> <destination city-or-IATA> --date <date> [--return <date>] [--adults N] [--children N] [--infants N] [--discount 0-5] [--currency CODE] [--browser [--cdp-url http://127.0.0.1:9222]] [--cheapest] [--direct] [--after HH:MM] [--before HH:MM] [--return-after HH:MM] [--return-before HH:MM] [--limit N] [--nearby] [--json|--toon]
```

Reads every flight of the day (and of the return day with `--return`) from Vueling's availability API with an anonymous session token. `fare` is the cheapest price for one adult with two decimals (e.g. `47.99`), and `fares` maps each fare bundle to its cheapest price, e.g. `{"Basic": "47.99", "Optima": "100.99"}` (Basic is the seat with a small under-seat bag; Optima adds a checked bag, seat choice and flexibility, so use it to answer "how much with luggage?"; omitted with `--browser`). Text output shows pricier bundles after the fare, e.g. `47.99 EUR (Optima 100.99)`. The Optima price is only visible on the website after selecting a flight, so never click a flight to check it; `--children` and `--infants` are sent to the search (infants do not change the adult fare). `--discount` applies a Spanish discount, as the website's "¿Eres residente o familia numerosa?" selector does: `1` resident of the islands, Ceuta or Melilla, `2` large family, `3` special large family, `4` resident and large family, `5` resident and special large family; it needs EUR, and Vueling rejects it on routes without that discount (`Service RESO not valid for market BCN - BIO`) — `routes ORIGIN --json` shows `residentDiscount`/`largeFamilyDiscount` per destination. The traveller must hold the right to the discount; Vueling checks it at booking. `itinerary` is synthesized, e.g. `13:30 BCN → 14:50 BIO VY1464 Directo` or `… VY2213+VY1464 Conexión en BCN`. Sold-out flights are omitted. The command never selects a fare.

`--browser` (implied by `--cdp-url`) instead opens a search tab in the user's Chrome and reads the rendered cards: on macOS through Apple Events (Chrome needs **View > Developer > Allow JavaScript from Apple Events**, which permits scripts to run in pages, so enable it only with the user's explicit approval), elsewhere through a local DevTools endpoint (`--cdp-url`, localhost or loopback only; Chrome 136+ ignores the port switch for its default data directory, so do not copy the user's profile — see [Chrome's remote-debugging changes](https://developer.chrome.com/blog/remote-debugging-port)). A CAPTCHA, expired session, or outage can prevent results from loading in that mode; fares there are whole euros as rendered.

The JSON/TOON object contains `url` and `flights`; each flight has `direction`, `date`, `flight`, `departure`, `arrival` (local `HH:MM`), `arrivalDate` (`YYYY-MM-DD`, only when the flight lands on a later day than `date`; text output marks it `00:45 +1`; not set with `--browser`), `fare`, `currency`, the `itinerary` text, and connection legs: `stops` (number of connections, `0` for direct), `flights` (every VY flight number on the card, in order, deduplicated; the first equals `flight`), and `via` (connection airports the card names, e.g. `["BCN"]` from "Conexión en BCN"; omitted when none is named). Layover duration and per-leg times are not parsed; read them from `itinerary`. Human-readable output adds `1 escala vía BCN · VY1383+VY1050 |` before the itinerary of connecting offers and leaves direct offers unchanged. Currency defaults to `EUR` and uses the same supported values as `search`/`link`. When Vueling has no flights that day, `flights` exits `2` with `Vueling shows no flights for that route on the requested date` (or `… no return flights …` for the return day); this is not a session problem.

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

`--nearby` also reads the fare calendar (like `calendar --prices --month`, and through Chrome with `--browser`) after the offers and reports the cheapest day of the outbound month, and of the return month with `--return` (earliest day on ties). Text output ends with `cheapest nearby: DD/MM/YYYY from FARE` and `cheapest nearby return: DD/MM/YYYY from FARE`; JSON/TOON adds `nearby` and `returnNearby` objects `{ "date", "fare" }`. Without `--nearby` the output is unchanged. If the calendar cannot be read, a warning goes to stderr, those fields are omitted, and the command still exits 0 with the offers. Calendar fares are not final booking quotes.

### `vueling select`

```text
vueling select <origin city-or-IATA> <destination city-or-IATA> --date <date> (--flight VY1234 | --cheapest) [--return <date> (--return-flight VY1234 | --return-cheapest)] [--direct] [--adults N] [--children N] [--infants N] [--discount 0-5] [--currency CODE] [--json|--toon]
```

Prices one flight the way vueling.com does when its fare is clicked, without booking: it reads availability and the price breakdown of the flight's cheapest fare (`ams.vueling.com/bkd/v4/breakdown/Availability`) and the bundle offers the website shows next. `--flight` takes the flight number, or every number of a connection joined with `+` (`VY1383+VY1050`); `--cheapest` picks the day's cheapest bookable flight instead, and `--direct` limits every cheapest pick to direct flights. With `--return`, pick the return flight the same way (`--return-flight` or `--return-cheapest`): each direction gets its own breakdown and bundle totals (the website picks a bundle per journey) and a final `trip total` line adds both cheapest fares. Vueling's breakdown rejects keys from two one-way searches in one request, so each direction is priced separately. Text output:

```text
VY1460 2026-11-10 07:00 BCN → 08:20 BIO
fare 40.48 + taxes and fees 24.51 (ES 5.91, JD 13.98, OG 0.63, QV 3.99) = 64.99 EUR
Fly Light 64.99 · Fly 109.99 · Fly (25 kg bag) 119.99 · Fly Pro 132.99 · Fly Grande 166.99
Nothing was booked. Confirm on vueling.com: https://tickets.vueling.com/booking?...
```

Totals cover all passengers. Bundles: `Fly Light` (VYBA, under-seat bag only), `Fly` (VCB2, cabin bag, standard seat), `Fly (25 kg bag)` (VCB1, checked bag instead), `Fly Pro` (VPRO, cabin bag, seat, change or advance the flight, priority), `Fly Grande` (VCB3, cabin bag + 25 kg bag, seat, changes and cancellation, priority). The website hides Optima/TimeFlex offers, so `select` does too. With `--infants N`, the infant fare and its taxes (about 25 EUR per flight plus taxes) show as their own `+ infant fee X (N infant)` part, already included in `total` and every bundle total; lap infants take no bundle. Overnight arrivals print `+N` after the time. Structured output is `{ "quote": { "flight", "date", "origin", "destination", "departure", "arrival", "arrivalDate" (only when later than `date`), "currency", "fare", "taxes", "taxDetail", "infants", "infantFee", "total", "bundles": [{ "code", "name", "pricePerPerson", "total" }] }, "url", "booked": false }`, plus `returnQuote` (same shape) and `total` (both directions) for return trips; the link selects both flights. It never sells, holds or pays: the user confirms on the printed link.

### `vueling extras`

```text
vueling extras <origin> <destination> --date <date> (--flight VY1234 | --cheapest) [--bundle VYBA|VCB2|VCB1|VPRO|VCB3] [--adults N] [--children N] [--infants N] [--currency CODE] [--json|--toon]
```

Exact prices Vueling charges for extras on one flight: each seat type (Space One, Space Plus, Space, Front, Back) with min–max price and free seats, checked bags by weight (15/20/25/30 kg), overhead cabin bag, gate fees, sports equipment, pets and Flex Pack. Prices are per passenger unless marked "sold for all passengers"; `--bundle` (default Fly Light, VYBA) sets which bundle they assume, because bundles include some extras. Outbound only. Structured: `{ "extras": { flight, date, origin, destination, bundle, currency, seats[], bags[], services[] }, "booked": false, "sessionEnded": true, "note" }`, each item `{ code, name, segment?, price, maxPrice?, available?, currency, perPassenger? }`.

**Guardrails.** Vueling only prices a flight's extras once it is in a cart, so this command, and only this one, opens its **own** fresh anonymous session (never the cached search session, browser cookies or an account), puts the cheapest fare in that cart (`POST bkj/v2/Sell/selltrip/BSV`), reads `GET bst/v2/SmartSeatMap` and `GET bpv/v3/BookingServices/Services/BPV`, and always ends the session (`DELETE asm/v1/Auth`), also on errors, so no cart is left. It never adds passengers or contact data, never selects seats or services, never pays, commits or books; a test fails on any other endpoint. Insurance is not shown (Vueling prices it only with passenger data). If ending the session fails the command says so; the cart then expires with the session within 20 minutes.

### `vueling session`

```text
vueling session [refresh|clear] [--json|--toon]
```

`flights` and `select` reuse one anonymous Vueling session across runs, cached in the user cache directory (`~/Library/Caches/vuelingcli/session.json` on macOS, `~/.cache/vuelingcli/session.json` on Linux; folder 0700, file 0600). A session idles out after 20 minutes; the CLI extends it the way the website does (`PUT ams.vueling.com/asm/v1/Auth`) when less than 2 minutes remain, opens a new one after the token's 1-hour limit, and retries once with a new session if Vueling rejects the cached token. `vueling session` shows `{ "active", "expiresAt", "minutesLeft" }`; `session refresh` extends it now (or opens one) and prints the new expiry; `session clear` ends it on Vueling's side and deletes the file (no `--json`/`--toon`). The session is the CLI's own anonymous one, never the user's browser login.

### `vueling status`

```text
vueling status <VY flight number> [--date <date>] [--watch <minutes>] [--browser [--cdp-url http://127.0.0.1:9222]] [--json|--toon]
vueling status <origin city-or-IATA> <destination city-or-IATA> [--date <date>] [--watch <minutes>] [--browser [--cdp-url http://127.0.0.1:9222]] [--json|--toon]
```

Reads the status of one Vueling flight, or of every Vueling flight on a route that day, from the JSON API behind Vueling's [flight-status page](https://www.vueling.com/es/servicios-vueling/informacion-de-vuelos/estado-de-vuelos). `--date` defaults to `today` and may be in the past, including `yesterday`/`ayer` (e.g. to check how a flight went). Vueling's lookup by flight number misses some flights that the route search lists (e.g. VY1460 BCN→BIO); when a number finds nothing, search the route. A search with no result for that day is reported as an error. Flight numbers may carry a letter suffix, such as `VY392D`. The JSON/TOON value is an array of objects with `flight`, `date`, `status` (Vueling's Spanish label: `En Hora`, `Nueva hora`, `En vuelo`, `Aterrizado`, `No operado` (cancelled), `Desviado a XXX`, `Reprogramado`, …; unknown values are printed raw), `origin`, `destination`, `departure` and `arrival` (latest local times `HH:MM`), and `details` (scheduled times, terminal, gate, baggage belt, connection). `--browser` (implied by `--cdp-url`) instead fills the page's form in a Chrome tab and reads the rendered cards; that mode only accepts dates the page offers (today onwards). Vueling notes that this information is indicative and refreshes every 10 minutes.

`--watch <minutes>` keeps reading every N minutes (a whole number, at least `5`). It prints the first read, then only cards whose `status`, `departure`, or `arrival` changed. Text lines carry the local time (`HH:MM`) before the usual line; with `--json` each card is printed as one JSON object per line (NDJSON) instead of an array. It stops once every watched card has landed (`Aterrizado…`/`Landed`) or is cancelled (`No operado`/`Cancelado`/`Cancelled`), case-insensitive, or on Ctrl-C/SIGTERM, exiting `0`. A failed first read exits as usual; later failed reads print a warning to stderr and watching continues. `--watch` with `--toon` or a value below 5 is a usage error.

### `vueling alerts`

```text
vueling alerts [--culture es-ES|en-GB|…] [--json|--toon]
```

Reads the disruption notice Vueling shows across its website during strikes, storms or outages (`www.vueling.com/v1/api/AlertsApi/getPaeAlert`). Text output is `No active Vueling disruption notice` or the notice text followed by its link; structured output is `{ "active", "text", "url" }` (`text`/`url` omitted when there is none). `--culture` picks the language (default `es-ES`). Check it when a flight shows delays or cancellations, before telling the user a disruption is isolated.

### `vueling fees`

```text
vueling fees [search text] [--json|--toon]
```

Reads Vueling's published price ranges for extras (seats, cabin and checked bags by weight, special luggage, pets, date/name changes, infant fee, insurance, price lock, Flex Pack…) from the table on `www.vueling.com/es/servicios-vueling/tarifas-de-los-servicios-adicionales-al-vuelo`. The optional search text keeps services whose name contains it (case- and accent-insensitive, Spanish names: `maleta`, `asiento`, `mascota`, `cambio`, `bebé`). Text output prints each service and its online, customer-service and airport prices (`—` when that channel does not sell it); structured output is an array of `{ "service", "online", "customerService", "airport" }` with the website's text (empty when not sold). These are ranges that depend on route, date and demand, not a quote; for a specific flight run `vueling select` to see the total with each bundle.

### `vueling airports`

```text
vueling airports [city-or-code] [--json|--toon]
```

Searches a local snapshot based on [Vueling's airport-help pages](https://help.vueling.com/hc/es/categories/19798729418513-Aeropuertos) and public destination pages by city name or three-letter code. Quote multi-word city names. Omitting the query prints the whole catalogue. It can lag route changes and makes no network request.

### `vueling version` and help

`vueling version` (also `--version` or `-v`) prints the CLI version. Add `--json` or `--toon` for structured output. `vueling help`, `--help`, or `-h` prints command names. Every command accepts `--help` (e.g. `vueling select --help`, `vueling deals --help`, `vueling session --help`) for its usage.

## Structured output

`search`, `link`, and `calendar` return a JSON object with one `url` field; `flights` returns `url` and `flights`; `status` returns an array of flight statuses; `version` returns a `version` field. Both use the same shape in TOON format:

```json
{"url":"https://tickets.vueling.com/booking?..."}
```

`airports` returns an array of `{ "code", "city" }` objects; no matches is an empty array with `--json`/`--toon` and a `no Vueling airport matches` error (exit `2`) in text mode. Human-readable text goes to stdout and usage errors go to stderr. Invalid arguments exit with code `2`.

`flights` returns offers in the order displayed by Vueling unless `--cheapest` is given; `--direct`, `--after`, `--before`, and `--limit` narrow each direction independently.
