---
name: vueling-flights
description: Search Vueling flights in the browser, open its public price calendar, or build flight-selection links with the vueling CLI.
---

# Vueling flights

Use Vueling's site in the browser for live flight results. Use `vueling` to build the public calendar and flight-selection links.

## Boundaries

- Read-only: search and inspect offers only. Never book, pay, check in, or change a reservation.
- The CLI does not retrieve live prices or availability. For a live search, open the official Vueling site in the available browser and report only the results shown there.
- Use the browser UI or a `vueling link` URL to search. Do not copy session tokens or call undocumented private APIs from the CLI. Vueling's [Flight Calendar documentation](https://www.vueling.com/developer/flightcalendar/flightcalendar-deeplink/) explains how to map its prices response to deeplinks, but does not document the request endpoint or contract needed by this CLI; direct requests tried from the CLI returned HTTP 403.
- Stop if the site requires a CAPTCHA or login the user has not completed. Never select a fare, book, pay, or change a reservation.
- Ask for origin, destination, dates, and passenger counts when missing. Do not infer travel dates.
- Prefer `--json` for programmatic workflows and keep diagnostics on stderr.
- Do not request or store account passwords or payment details.

## Commands

`vueling calendar BCN MLA --date tomorrow` builds a public Vueling price-calendar link. `vueling search BCN MLA --date tomorrow` and `vueling link BCN MLA --date tomorrow` both build the official flight-selection link; open it in the browser to see current results. These commands accept passenger counts, `--discount`, `--culture`, and `--currency`. For a specific offer they also accept `--flight VY2115 --fare-basis DOWVYCLB`, and the matching return options `--return tomorrow --return-flight VY5200 --return-fare-basis DOWVYCLB`. `--token` passes Vueling's optional `MET_Token`. Add `--json` or `--toon` for structured output. The CLI does not fetch or rank live fares. `vueling airports paris` searches the local catalogue and also accepts `--json` or `--toon`; the catalogue is partial.
