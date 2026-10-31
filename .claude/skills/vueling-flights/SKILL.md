---
name: vueling-flights
description: Search live Vueling prices and flight itineraries in the user's browser session, or build flight-selection links with the vueling CLI.
---

# Vueling flights

Use Vueling's site in the user's browser for live prices and flight itineraries. If the browser is already open and a computer-use/browser tool is available, read the visible offers from that page directly; this does not require changing Chrome settings. Use `vueling flights` when its local browser bridge is enabled, and use the other commands to resolve airports and build official calendar and flight-selection links.

## Boundaries

- Read-only: search and inspect offers only. Never book, pay, check in, or change a reservation.
- On macOS, `vueling flights` uses Chrome Apple Events to open a search tab, submit the visible search form, and read the rendered offer cards in the current profile. Chrome requires **View > Developer > Allow JavaScript from Apple Events** for this. This permits AppleScript callers to run JavaScript in pages; never enable it without the user's explicit approval. On other systems the command attaches to a local Chrome DevTools endpoint (`http://127.0.0.1:9222` by default); `--cdp-url` selects another loopback endpoint. DevTools grants browser control, so never expose it on a network.
- The live command reads visible page text only. Do not copy session tokens or cookies into the CLI, files, or output, and do not inspect private API requests.
- When reading through browser UI, use its accessibility tree or rendered page text and leave fare buttons untouched. The route, times, flight number, and displayed price are visible in the offer card.
- Use `vueling calendar ORIGIN DESTINATION --date DATE --return RETURN_DATE --prices --json` to read outbound and return fares from the rendered public calendar. The return month must appear in the calendar's visible month tabs, and return dates must be within 330 days of departure. Treat these as calendar fares, not a final booking quote.
- Use `vueling flights ...` or Vueling's visible search form to read flight times and numbers. Continue only to display flight options, never select a fare.
- Stop before selecting a flight/fare or continuing into passenger details, seats, extras, or payment. Never book, pay, check in, or change a reservation.
- Chrome 136+ ignores `--remote-debugging-port` for the default Chrome data directory; do not work around this by copying the user's profile. Never enable remote debugging, relaunch Chrome, or change its startup flags without the user's explicit approval. See [Chrome's remote-debugging changes](https://developer.chrome.com/blog/remote-debugging-port).
- Stop if the site requires a CAPTCHA or login the user has not completed.
- Ask for origin, destination, dates, and passenger counts when missing. Do not infer travel dates.
- Prefer `--json` for programmatic workflows and keep diagnostics on stderr.
- Do not request or store account passwords or payment details.

## Commands

For complete command syntax, option meanings, date formats, and output schemas, read [the CLI reference](references/cli-reference.md). Quick examples: `vueling flights BCN AGP --date +7 --return +14 --json`, `vueling calendar BCN AGP --date +7 --return +14 --prices --json`, `vueling calendar Barcelona "Palma de Mallorca" --date tomorrow --open`, `vueling search BCN AGP --date +7 --adults 2 --json`, `vueling airports paris --toon`, and `vueling version --json`. Quote multi-word city names; ambiguous cities must be passed by IATA code. `flights` reads visible fares through local Chrome; `calendar --prices` reads the public rendered calendar; `search`, `link`, and calendar without `--prices` only create URLs.
