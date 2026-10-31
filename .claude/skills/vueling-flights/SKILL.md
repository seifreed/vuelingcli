---
name: vueling-flights
description: Search live Vueling prices and flight itineraries in the user's browser session, or build flight-selection links with the vueling CLI.
---

# Vueling flights

Use Vueling's site in the user's browser for live prices and flight itineraries. Use `vueling flights` to read visible offers through the user's local Chrome session; use the other commands to resolve airports and build official calendar and flight-selection links.

## Boundaries

- Read-only: search and inspect offers only. Never book, pay, check in, or change a reservation.
- `vueling flights` attaches to Chrome's local DevTools endpoint (`http://127.0.0.1:9222` by default), opens the search in a tab using that Chrome profile, and reads the rendered offer cards. Remote debugging must be explicitly enabled for the profile containing the Vueling session; use `--cdp-url` only for a different local endpoint. This endpoint grants browser control, so keep it bound to loopback and do not expose it on a network.
- The live command reads visible page text only. Do not copy session tokens or cookies into the CLI, files, or output, and do not inspect private API requests.
- Build a calendar URL with `vueling calendar ... --open` or use Vueling's visible search form. Read the outbound and return calendar prices from the page, select the requested dates, and use the page's continue action only to display flight options. Report the visible date, route, time, carrier, flight number, and fare.
- Stop before selecting a flight/fare or continuing into passenger details, seats, extras, or payment. Never book, pay, check in, or change a reservation.
- Chrome 136+ ignores `--remote-debugging-port` for the default Chrome data directory; do not work around this by copying the user's profile. Use Chrome's explicit remote-debugging controls when available, or a dedicated non-default profile that the user signs into. Never enable remote debugging, relaunch Chrome, or change its startup flags without the user's explicit approval. See [Chrome's remote-debugging changes](https://developer.chrome.com/blog/remote-debugging-port).
- Stop if the site requires a CAPTCHA or login the user has not completed.
- Ask for origin, destination, dates, and passenger counts when missing. Do not infer travel dates.
- Prefer `--json` for programmatic workflows and keep diagnostics on stderr.
- Do not request or store account passwords or payment details.

## Commands

For complete command syntax, option meanings, date formats, and output schemas, read [the CLI reference](references/cli-reference.md). Quick examples: `vueling flights BCN AGP --date +7 --return +14 --json`, `vueling calendar Barcelona "Palma de Mallorca" --date tomorrow --open`, `vueling search BCN AGP --date +7 --adults 2 --json`, `vueling airports paris --toon`, and `vueling version --json`. Quote multi-word city names; ambiguous cities must be passed by IATA code. `flights` reads visible fares through local Chrome; `search`, `link`, and `calendar` only create URLs.
