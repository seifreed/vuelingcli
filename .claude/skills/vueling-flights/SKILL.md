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

For complete command syntax, option meanings, date formats, and output schemas, read [the CLI reference](references/cli-reference.md). Quick examples: `vueling calendar BCN MLA --date tomorrow --open`, `vueling search BCN AGP --date +7 --adults 2 --json`, and `vueling airports paris --toon`. The CLI creates URLs and does not fetch or rank live fares.
