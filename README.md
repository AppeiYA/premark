# PreMark

A rule-based smart buyer for [PreStocks](https://prestocks.com) — tokenized pre-IPO stocks on Solana.

Set a rule like *"tell me when ANTHROPIC trades 2% or more below its mark price, with a $50 buy"*, and PreMark watches the market, checks a live executable quote on Jupiter, and surfaces a signal with a ready-to-sign swap link when every safety check passes.

Built for the Stocklana hackathon — **Best Use of PreStocks** bounty.

**Live demo:** `https://premark.onrender.com/`
**Demo video:** `<link here>`

---

## Why this exists

PreStocks tokens trade at a real, visible premium or discount to their mark price (SPV fair value). Dashboards show this, but nobody acts on it. PreMark turns "the token is cheap right now" into an actionable, quoted signal — without ever touching your keys or your funds.

## What it does

- Pulls all 8 PreStocks tokens from the PreStocks API and computes each one's premium/discount to mark price.
- Lets you create a buy-below rule: token, discount threshold, budget, max acceptable price impact, and a cooldown.
- On every scan cycle (default 60s), evaluates all enabled rules in two stages:
  1. **Cheap check** — is the API-reported premium already below your threshold?
  2. **Live quote check** — pull a real executable quote from Jupiter, and re-check the threshold against the *actual* price you'd pay, not just the indicative one.
- Runs a handful of safety checks before ever surfacing a signal: stale data, price divergence between the API and the executable quote (possible corporate action), price impact too high, and executable premium worse than it looked.
- Notifies you (log / Telegram) and gives you a swap link. **You sign the transaction yourself — PreMark is non-custodial and never holds keys or funds.**

## What it deliberately does not do

- No custody, no wallet connection, no transaction signing or sending on your behalf.
- No sell-side rules, no leverage, no other pre-IPO token issuers — PreStocks tokens only, per the bounty rules.
- No user accounts or authentication beyond a plain `X-Owner-ID` header — this is a hackathon build, not a production auth system. Don't rely on it to keep rules private on a shared deployment.

## Architecture

Hexagonal (ports and adapters), SOLID throughout:

```
adapter/in  ──►  ports (inbound)  ◄── usecase ──► ports (outbound)  ◄── adapter/out
                                         │
                                         ▼
                                      domain
```

- **domain** — pure business rules (Rule, Premium, Decision, Signal, Quote). No I/O, no framework, no struct tags.
- **ports** — interfaces only, implemented by use cases (inbound) and adapters (outbound).
- **usecase** — application logic: RuleService, MarketQuery, Ingestor, Evaluator, Scanner.
- **adapter/in** — HTTP API (`httpapi`) and the scan scheduler. `json` tags live here, nowhere else.
- **adapter/out** — PreStocks API client, Jupiter quote client, Solana RPC (mint decimals + Token-2022 `ScaledUiAmount` multiplier), SQLite storage, notifiers.

An `arch-check.sh` script enforces the layering and the no-tags-outside-adapters rule as part of `make check`.

Full build spec: [`implementation.md`](./implementation.md).

## Tech stack

- Go (standard library HTTP router, no framework)
- SQLite (`modernc.org/sqlite`, pure Go, no CGO — the one allowed dependency)
- [Jupiter](https://jup.ag) for live swap quotes
- Solana RPC for mint decimals and Token-2022 `ScaledUiAmount` multipliers (two PreStocks tokens — OpenAI and SpaceX — use this extension; without reading it, quoted prices are off by that multiplier)
- Embedded single-page UI (`//go:embed`, no build step, no JS framework)

## Running locally

```bash
git clone <repo>
cd premark
go build -o premark ./cmd/premark
./premark
```

Open `http://localhost:8080`.

### Configuration (environment variables)

| Variable | Default | Purpose |
|---|---|---|
| `PORT` | `8080` | HTTP listen port |
| `STORAGE` | `sqlite` | `sqlite` or `memory` |
| `DB_PATH` | `premark.db` | SQLite file path |
| `PRESTOCKS_API_URL` | `https://prestocks.com/api/prestocks` | Market data source |
| `JUPITER_BASE_URL` | `https://lite-api.jup.ag/swap/v1` | Quote API |
| `JUPITER_API_KEY` | empty | Optional |
| `SOLANA_RPC_URL` | `https://api.mainnet-beta.solana.com` | Mint decimals/multiplier lookups |
| `SCAN_INTERVAL` | `60s` | How often rules are evaluated |
| `ADMIN_TOKEN` | empty | If set, required (`X-Admin-Token` header) to call `POST /v1/scan` |
| `TELEGRAM_BOT_TOKEN` | empty | Enables Telegram notifications when set |

### Smoke test

`cmd/smoke` hits the real PreStocks, Solana RPC and Jupiter APIs and prints raw + computed values, so you can sanity-check price impact units and Token-2022 multipliers against live data:

```bash
go run ./cmd/smoke
```

## API

| Method & path | Auth | Purpose |
|---|---|---|
| `GET /healthz` | — | Liveness check |
| `GET /v1/tokens` | — | All PreStocks with current premium/discount, cheapest first |
| `GET /v1/tokens/{symbol}` | — | One token |
| `GET /v1/tokens/{symbol}/history?hours=24` | — | Recent price/premium history |
| `POST /v1/rules` | `X-Owner-ID` | Create a buy-below rule |
| `GET /v1/rules` | `X-Owner-ID` | List your rules |
| `PATCH /v1/rules/{id}` | `X-Owner-ID` | Enable/disable a rule |
| `DELETE /v1/rules/{id}` | `X-Owner-ID` | Delete a rule |
| `GET /v1/signals?limit=20` | `X-Owner-ID` | Your triggered signals |
| `POST /v1/scan` | `X-Admin-Token` (if configured) | Trigger a scan on demand. The embedded UI's "Scan now" button calls this same-origin — the server holds `ADMIN_TOKEN` and attaches it itself, so the token is never sent to or exposed in the browser. |

## Testing

Three layers, matching the hexagonal boundaries:

- **Domain** — unit tests on `Rule`, `Premium`, `Precheck`/`Decide`, boundary conditions on every threshold.
- **Use case** — `RuleService`, `MarketQuery`, `Ingestor`, `Evaluator`, `Scanner`, exercised against in-memory fakes so failure-isolation behavior (one bad rule never blocks the others) is verified directly.
- **HTTP integration** — full router wired to real use cases and in-memory fakes for external APIs, covering routing, status codes, and error shapes.

```bash
go test ./...
```

## Known limitations

- **Price impact is near-zero at retail budget sizes.** Verified against live Jupiter quotes: a $10–$1,000 quote on ANTHROPIC returns ~0 price impact; a $50,000 quote returns ~66 bps. At the budget sizes this project targets, the price-impact safety check will rarely trigger — that's expected pool depth, not a bug.
- **No persistent disk on the free-tier deployment.** Data survives idle time but resets on redeploy. Fine for a demo; not meant for production use as-is.
- **`X-Owner-ID` is not authentication.** It's a convenience identifier for a hackathon demo, not a security boundary.
- **`ADMIN_TOKEN` never leaves the server.** The UI's "Scan now" button hits a same-origin endpoint; the server attaches `X-Admin-Token` to the internal scan call itself. The browser never sees the token, so it can't be read from page source or the network tab.

## Compliance notes

- Uses **PreStocks tokens exclusively**, per the bounty's eligibility rule — no other pre-IPO token issuer is integrated.
- PreMark is **non-custodial**: it never requests, stores, or uses private keys, and never signs or submits a transaction on the user's behalf. All it produces is a quote and a swap link the user signs themselves.
- PreStocks tokens are not available to US persons; this tool is a market/quote layer and does not attempt to circumvent that restriction.
- Not financial advice. Signals reflect a rule the user configured against public market and quote data, nothing more.
