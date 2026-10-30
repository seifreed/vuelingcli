---
name: vueling-flights
description: Open Vueling's public price calendar and build flight-selection links using the vueling CLI.
---

# Vueling flights

Use `vueling` to open Vueling's public price calendar or build a flight-selection link.

## Boundaries

- Read-only: search and inspect offers only. Never book, pay, check in, or change a reservation.
- The CLI does not retrieve live prices or availability. Ask the user to check these on Vueling's site.
- Ask for origin, destination, dates, and passenger counts when missing. Do not infer travel dates.
- Prefer `--json` for programmatic workflows and keep diagnostics on stderr.
- Do not request or store account passwords or payment details.

## Commands

`vueling calendar BCN MLA --date tomorrow` builds a public Vueling price-calendar link. `vueling link BCN MLA --date tomorrow` builds a flight-selection link. Add `--json` or `--toon` for structured output. `vueling airports paris` searches the local catalogue of common stations. The catalogue is partial. `vueling search` is unavailable because the official Flight Calendar Prices API requires Vueling access; do not claim the CLI returns live fares.
