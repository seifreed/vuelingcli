---
name: vueling-flights
description: Search Vueling flight availability and fares using the vueling CLI.
---

# Vueling flights

Use `vueling` to search publicly available flight offers from Vueling.

## Boundaries

- Read-only: search and inspect offers only. Never book, pay, check in, or change a reservation.
- Prices and availability can change; report the search date and currency with results.
- Ask for origin, destination, dates, and passenger counts when missing. Do not infer travel dates.
- Prefer `--json` for programmatic workflows and keep diagnostics on stderr.
- Do not request or store account passwords or payment details.

## Commands

`vueling calendar BCN MLA --date tomorrow` builds a public Vueling price-calendar link. Add `--json` or `--toon` for structured output. `vueling search` and `vueling airports` are still in progress; check `vueling help` before use and do not claim these commands return live data.
