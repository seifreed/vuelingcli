---
name: vueling-flights
description: Search live Vueling prices and flight itineraries in the user's browser session, or build flight-selection links with the vueling CLI.
---

# Vueling flights

Use Vueling's site in the user's browser for live prices and flight itineraries. Use `vueling` to resolve airports and build official calendar and flight-selection links.

## Boundaries

- Read-only: search and inspect offers only. Never book, pay, check in, or change a reservation.
- For a live search, use the already available browser session and read prices and itineraries from Vueling's rendered page. Do not copy session tokens or cookies into the CLI, files, or output.
- Build a calendar URL with `vueling calendar ... --open` or use Vueling's visible search form. Read the outbound and return calendar prices from the page, select the requested dates, and use the page's continue action only to display flight options. Report the visible date, route, time, carrier, flight number, and fare.
- Stop before selecting a flight/fare or continuing into passenger details, seats, extras, or payment. Never book, pay, check in, or change a reservation.
- The CLI itself currently creates links and does not retrieve live prices or availability. Use the browser for live results; do not call undocumented private APIs from the CLI.
- Stop if the site requires a CAPTCHA or login the user has not completed.
- Ask for origin, destination, dates, and passenger counts when missing. Do not infer travel dates.
- Prefer `--json` for programmatic workflows and keep diagnostics on stderr.
- Do not request or store account passwords or payment details.

## Commands

For complete command syntax, option meanings, date formats, and output schemas, read [the CLI reference](references/cli-reference.md). Quick examples: `vueling calendar Barcelona "Palma de Mallorca" --date tomorrow --open`, `vueling search BCN AGP --date +7 --adults 2 --json`, `vueling airports paris --toon`, and `vueling version --json`. Quote multi-word city names; ambiguous cities must be passed by IATA code. The CLI creates URLs and does not fetch or rank live fares.
