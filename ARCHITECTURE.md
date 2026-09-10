# Architecture

bbmonitor has **two data layers**:

## 1. Programs
Public bug bounty / VDP program metadata and **in-scope rules**.

Stable raw feeds currently wired:

| Role | Feed |
|------|------|
| Platform scopes (H1, Bugcrowd, Intigriti, YesWeHack, Federacy) | arkadiyt/bounty-targets-data `*_data.json` |
| Program + domain lists | projectdiscovery/public-bugbounty-programs |
| VDP / BB policy index | disclose/diodb `program-list.json` |
| Extra program lists | rix4uni/scope |
| Program → GitHub org map | nikitastupin/orgs-data |

## 2. Inventory (assets)
Domains, wildcards, government zones, host indexes — what hunters enumerate.

| Role | Feed |
|------|------|
| US `.gov` domain zone (federal + full CSV) | cisagov/dotgov-data |
| Flat domain / wildcard dumps | arkadiyt `domains.txt` / `wildcards.txt` (via same arkadiyt adapter targets) |

### Planned / optional (large or secondary)
- **trickest/inventory** — `targets.json` + per-org `hostnames.txt` (huge; best as optional heavy adapter)
- **chaos.projectdiscovery.io** — `index.json` + zip datasets (subdomain packs)
- **GSA/data** dotgov-websites — related gov host lists

### Explicitly out of scope (no stable raw API)
HTML-only sites: bbradar.io, firebounty.com, bug-bounties.as93.net, sploitus, etc.  
disclose/bug-bounty-platforms is a README catalog of *platforms*, not program scopes.

## Diff + notify
Every program add/update and every target add/remove/update is recorded and can notify Telegram/webhook.
