---
name: vueling-flights
description: Search live Vueling prices, flight itineraries, and flight status with the vueling CLI (direct HTTPS to Vueling's public JSON APIs, no browser needed), or build flight-selection links.
---

# Vueling flights

Use the `vueling` CLI for live prices, flight itineraries, and flight status: `flights`, `calendar --prices`, and `status` call the JSON APIs behind Vueling's website over HTTPS without opening a browser or reading cookies. Use the other commands to resolve airports and build official calendar and flight-selection links. Fall back to `--browser` (rendered pages in the user's Chrome) only if the API path fails.

## Boundaries

- Read-only: search and inspect offers only. Never book, pay, check in, or change a reservation.
- By default the CLI sends HTTPS requests that mimic Chrome (TLS fingerprint and headers) to Vueling's undocumented APIs, using the anonymous session any visitor gets. Never add account credentials or cookies to these requests. If they fail (e.g. `HTTP 403` from the CDN), retry once with `--browser`. With `--browser` on macOS the CLI uses Chrome Apple Events, which needs **View > Developer > Allow JavaScript from Apple Events**; that permits AppleScript callers to run JavaScript in pages, so never enable it without the user's explicit approval. On other systems `--browser` attaches to a local Chrome DevTools endpoint (`http://127.0.0.1:9222` by default; `--cdp-url` selects another loopback endpoint). DevTools grants browser control, so never expose it on a network.
- Do not copy the user's session tokens or cookies into the CLI, files, or output.
- When reading through browser UI, use its accessibility tree or rendered page text and leave fare buttons untouched. The route, times, flight number, and displayed price are visible in the offer card.
- Use `vueling calendar ORIGIN DESTINATION --date DATE --return RETURN_DATE --prices --json` to read outbound and return fares from Vueling's fare calendar (about 11 months ahead; return dates must be within 330 days of departure). Add `--month` to list every visible day fare of each month with the cheapest day (`days`/`cheapest`, `returnDays`/`returnCheapest`). Treat these as calendar fares, not a final booking quote.
- Use `vueling flights ...` or Vueling's visible search form to read flight times and numbers. Continue only to display flight options, never select a fare.
- Stop before selecting a flight/fare or continuing into passenger details, seats, extras, or payment. Never book, pay, check in, or change a reservation.
- Chrome 136+ ignores `--remote-debugging-port` for the default Chrome data directory; do not work around this by copying the user's profile. Never enable remote debugging, relaunch Chrome, or change its startup flags without the user's explicit approval. See [Chrome's remote-debugging changes](https://developer.chrome.com/blog/remote-debugging-port).
- Stop if the site requires a CAPTCHA or login the user has not completed.
- Run live lookups one at a time and only as needed; never retry in a loop. `vueling status --watch` is the only polling allowed: watch at most one flight or route at a time and never lower its 5-minute minimum interval. With `--browser`, if tickets.vueling.com stays on "Cargando..." or `flights` reports no rendered results, stop and ask the user to sign in to Vueling in that Chrome profile (a signed-in session has resolved this) or try a manual search, then retry once. `Vueling shows no flights for that route on the requested date` is a real answer, not a stall: offer other dates (`calendar --prices --month`) instead of asking the user to sign in.
- Ask for origin, destination, dates, and passenger counts when missing. Do not infer travel dates.
- Prefer `--json` for programmatic workflows and keep diagnostics on stderr.
- Do not request or store account passwords or payment details.

## Commands

For complete command syntax, option meanings, date formats, and output schemas, read [the CLI reference](references/cli-reference.md). Quick examples: `vueling flights BCN AGP --date +7 --return +14 --json`, `vueling flights BCN AGP --date +7 --direct --after 08:00 --cheapest --limit 3 --json`, `vueling flights BCN AGP --date +7 --return +14 --nearby --json` (adds the cheapest calendar day of each month as `nearby`/`returnNearby`), `vueling calendar BCN AGP --date +7 --return +14 --prices --json`, `vueling calendar Barcelona "Palma de Mallorca" --date tomorrow --open`, `vueling search BCN AGP --date +7 --adults 2 --json`, `vueling status VY1158 --date today --json`, `vueling status BCN PMI --json`, `vueling status VY1158 --watch 10 --json`, `vueling airports paris --toon`, and `vueling version --json`. Quote multi-word city names; ambiguous cities must be passed by IATA code. `flights`, `calendar --prices`, and `status` read Vueling's JSON APIs (add `--browser` to read the rendered pages in Chrome instead); `status` also accepts past dates; `search`, `link`, and calendar without `--prices` only create URLs.
