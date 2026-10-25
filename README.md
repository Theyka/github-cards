# github-cards

A small, self-hosted **Go** web service that renders GitHub **stats**, **repository pin** and **contribution streak** cards as SVGs for your README.

It is a Go port of two MIT-licensed projects:

- [github-stats-extended](https://github.com/stats-organization/github-stats-extended) (a fork of github-readme-stats) — stats card (`/api`) and pin card (`/api/pin`)
- [github-readme-streak-stats](https://github.com/DenverCoder1/github-readme-streak-stats) — streak card (`/streak`)

The SVG layouts, themes, icons, formulas and GraphQL queries are derived from those projects and were kept byte-faithful where possible (verified against live upstream output: the pin card and streak card are structurally identical, and the stats card matches everywhere except data that depends on token visibility).

No Vercel, no serverless, no build tooling: one static binary, one Dockerfile, ready for [Coolify](https://coolify.io).

## Design cards in your browser

Open the service's home page (for example, [githubstats.theyka.net](https://githubstats.theyka.net/)) to use the **card studio**. It works on phones, tablets, and desktops:

- Enter a GitHub username (and a repository for pin cards).
- Choose stats, repository pin, or contribution streak cards.
- Browse the actual renderer theme collections, override colors with pickers or hex/gradient inputs, and adjust card-specific layout and data options.
- Preview the real SVG with debounced auto-updates, or switch auto-update off and use **Update preview**. Light, dark, and checkerboard backgrounds help check transparent themes.
- Copy the image URL, a responsive, table-free HTML embed, or Markdown. URLs automatically use the host you're visiting, so self-hosted installations work without configuration.
- **Share design** copies a link that restores your settings. Designs live in the URL fragment, not a database, and never include a GitHub token.

The UI, CSS, and JavaScript are embedded in the Go binary. No npm install, frontend build, third-party CDN, or browser-side token is required. The server still needs its configured `GITHUB_TOKEN` to fetch live card data; API failures appear in the preview as error cards.

## Endpoints

| Path | Card | Example |
| --- | --- | --- |
| `GET /` | Responsive card builder (legacy `?user=` and `?username=` image redirects are preserved) | `/` |
| `GET /builder/themes` | Theme names and palettes used by the builder | `/builder/themes` |
| `GET /api` | GitHub stats card | `/api?username=Theyka&show_icons=true&theme=radical` |
| `GET /api/pin` (or `/api/pin/`) | Repository pin card | `/api/pin?username=Theyka&repo=Turnstile-Solver&theme=radical` |
| `GET /streak` (or `/streak/`) | Contribution streak card (SVG, or `type=json`) | `/streak?user=Theyka&theme=radical` |
| `GET /healthz` | Health check (`{"status":"ok"}`) | `/healthz` |

All cards are returned with `HTTP 200` and `Content-Type: image/svg+xml`; failures render an upstream-style **error card** (dark, with a short explanation) instead of an HTTP error, so README embeds never break. Error cards are cached briefly (≤15 minutes).

## Environment variables

| Variable | Default | Description |
| --- | --- | --- |
| `GITHUB_TOKEN` | — | **Required.** GitHub token used for GraphQL/REST calls (classic PAT, fine-grained token, or `gh auth token`). Without it, cards render a "token required" error card. |
| `PAT_1` … `PAT_9`, `TOKEN`, `TOKEN2` … `TOKEN9` | — | Alternative names, matching both upstreams' conventions. All may be comma-separated lists; tokens are picked randomly and rotated on rate limits. |
| `PORT` | `3000` | HTTP port. |
| `CACHE_SECONDS` | `21600` (stats/pin), `86400` (streak) | When set, overrides the in-memory cache TTL for **all** cards. `0` disables caching. |

Tokens should have **no scopes** (public data only). One token is enough; more tokens raise the API rate limit.

## Stats card — `/api`

| Parameter | Default | Description |
| --- | --- | --- |
| `username` | — | GitHub username (**required**). |
| `theme` | `default` | Theme name (same theme set as github-stats-extended). |
| `title_color`, `icon_color`, `text_color`, `bg_color`, `border_color`, `ring_color`, `prog_bar_bg_color` | — | Hex colors (`f00`, `ff0000`, `ff0000ff`, `0xff0000`) or CSS color names. `bg_color` supports `angle,start,end` gradients. Invalid values render an error card. |
| `hide` | — | Comma list of stats to hide: `stars`, `commits`, `prs`, `issues`, `contribs`. |
| `hide_title` | `false` | Hide the `{username}'s GitHub Stats` title. |
| `hide_rank` | `false` | Hide the rank circle (card height shrinks to fit). |
| `hide_border` | `false` | Hide the card border. |
| `show_icons` | `false` | Show the icons before labels. |
| `card_width` | `467` | Card width in px (grows to fit the longest label + value). |
| `line_height` | `25` | Line height of the stat rows. |
| `text_bold` | `true` | Use bold stat values. |
| `custom_title` | `{username}'s GitHub Stats` | Custom title text. |
| `border_radius` | `4.5` | Corner radius. Invalid values render an error card (upstream behavior). |
| `number_format` | `short` | `short` (`1.2k`) or `long` (`1200`). |
| `number_precision` | — | Number of decimals for `short` format (clamped 0–2). |
| `rank_icon` | `default` | `default` (letter), `percentile` (`B+ 50.8%`), or `github`. |
| `include_all_commits` | `false` | Count all commits ever (REST search API) instead of the last year's contributions. |
| `commits_year` | — | Restrict displayed commits to a four-digit year (also used with `include_all_commits`). |
| `exclude_repo` | — | Comma list of repos excluded from the star count. |
| `role` | — | Owner affiliations used for the star count: comma list of `OWNER`, `COLLABORATOR`, `ORGANIZATION_MEMBER`. |
| `contribs_include_own_repos` | `false` | Include contributions to your own repositories. |
| `disable_animations` | `false` | Strip entrance animations. |

> With `include_all_commits=true`, the commit count comes from the GitHub REST search API, which needs a token with access to `api.github.com/search/commits` (a plain PAT is fine).

## Pin card — `/api/pin`

| Parameter | Default | Description |
| --- | --- | --- |
| `username` | — | Repository owner (**required**). |
| `repo` | — | Repository name (**required**). `owner/repo` is also accepted in `repo`. |
| `theme` | `default_repocard` | Theme name. |
| `title_color`, `icon_color`, `text_color`, `bg_color`, `border_color` | — | Same color rules as the stats card. |
| `hide` | — | Comma list of items to hide: `stars`, `language`. |
| `hide_border` | `false` | Hide the card border. |
| `card_width` | `400` | Card width in px. |
| `line_height` | `22` | Line height (hidden metrics only). |
| `show_icons` | `true` | Show repository icon. |
| `show_owner` | `false` | Prefix the repo name with `owner/`. |
| `text_bold` | `false` | Use bold repo name. |
| `border_radius` | `4.5` | Corner radius. |
| `description_lines_count` | — | Force the description line count (clamped 1–3). |
| `number_format` | `short` | `short` or `long`. |
| `disable_animations` | `true` | Animations are off by default for pins (upstream behavior). |

## Streak card — `/streak`

| Parameter | Default | Description |
| --- | --- | --- |
| `user` | — | GitHub username (**required**). |
| `theme` | `default` | Theme name (same theme set as github-readme-streak-stats). |
| `background`, `border`, `stroke`, `ring`, `fire`, `currStreakNum`, `sideNums`, `currStreakLabel`, `sideLabels`, `dates`, `excludeDaysLabel` | — | Theme property overrides. Invalid values silently fall back (upstream behavior). |
| `hide_border` | `false` | Hide the card border (`true` exact match). |
| `border_radius` | `4.5` | Corner radius. |
| `card_width` | `495` | Card width (minimum `100 × number of visible columns`). |
| `card_height` | `195` | Card height (minimum `170`). |
| `hide_total_contributions` | `false` | Hide the Total Contributions column. |
| `hide_current_streak` | `false` | Hide the Current Streak column. |
| `hide_longest_streak` | `false` | Hide the Longest Streak column. |
| `short_numbers` | `false` | Format numbers short (`1.5k`). |
| `disable_animations` | `false` | Strip entrance animations (`true` exact match). |
| `starting_year` | — | Override the first year queried. |
| `exclude_days` | — | Days that extend (not break) the current streak, e.g. `Sat,Sun`. |
| `mode` | `daily` | `daily` or `weekly`. |
| `timezone` | `UTC` | IANA timezone, e.g. `Europe/Paris`. Invalid values are rejected. |
| `type` | `svg` | `json` returns the raw stats as JSON. |

## Deploy on Coolify

1. Push this repository to GitHub (see below).
2. In Coolify: **Project → New Resource → Public/Private Repository** (or GitHub App).
3. Pick the repository; set **Build Pack: `Dockerfile`**.
4. Set the **exposed port** to `3000` and health check path to `/healthz` (both already default in the Dockerfile: `EXPOSE 3000`, `HEALTHCHECK`; the image ships `curl` and `wget` so Coolify's own health check works too).
5. Add environment variables in the Coolify UI:
   - `GITHUB_TOKEN=<your token>` *(required — create a token with no scopes; fine-grained tokens need "Public Repositories (read-only)")*
   - optionally `CACHE_SECONDS=21600`
6. Assign a domain (e.g. `https://stats.example.com`) and deploy. The build is a multi-stage `golang:1.27-alpine` → `alpine:3.20` image; no runtime dependencies beyond `ca-certificates` and `tzdata`.
7. Verify `https://<your-domain>/healthz` returns `{"status":"ok"}`, then open a card URL.

`docker compose up --build` also works locally (see `docker-compose.yml`).

## Embed in your profile README

Replace `https://<your-domain>` with your Coolify domain:

```html
<p align="center">
  <a href="https://github.com/Theyka/Turnstile-Solver">
    <img src="https://<your-domain>/api/pin/?username=Theyka&repo=Turnstile-Solver&theme=radical&show_icons=true&disable_animations=true&show_owner=true" width="400" alt="Turnstile-Solver" />
  </a>
  <a href="https://github.com/Theyka/Torrent-Viewer">
    <img src="https://<your-domain>/api/pin/?username=Theyka&repo=Torrent-Viewer&theme=radical&show_icons=true&disable_animations=true&show_owner=true" width="400" alt="Torrent-Viewer" />
  </a>
</p>

<p align="center">
  <img src="https://<your-domain>/api?username=Theyka&show_icons=true&theme=radical&disable_animations=true" alt="Theyka's GitHub Stats" />
  <img src="https://<your-domain>/streak/?user=Theyka&theme=radical&disable_animations=true" alt="Theyka's streak" />
</p>
```

## Supported subset / known differences

Ported with high fidelity, but intentionally not implemented:

- **Locales/translations** — English only (the upstream project ships many languages).
- **WakaTime / Gist cards** — not implemented.
- **Top languages card** (`/api/top-langs`) — not implemented.
- **`browser_rendering`** (`foreignObject` PNG rendering) — not implemented.
- **Light/dark dual themes** (`theme=dark&theme=light` aliasing) — set one theme instead.
- **`type=png`** — returns SVG (GitHub's camo cache handles SVG fine).
- **Custom streak labels** (`card_labels`), RTL layout, `date_format`, other niche params.

Three deliberate improvements over upstream:

- A missing/invalid token renders a styled error card instead of crashing; the service never exits because of API errors.
- Tokens are not dropped from the pool on rate limits (upstream removes them permanently); they are retried and kept.
- Transient failures never pin a card: GitHub lookups are detached from the incoming request (a browser reload or a camo timeout can no longer leave `context canceled` cached as your card), canceled or timed-out lookups are never cached, failures reaching GitHub are cached for only 2 minutes, and uncached error cards are sent with `no-store`.

## Development

```sh
go build ./...          # build
go vet ./...            # static checks
go test ./...           # Go route, asset, and theme tests
node --test tools/tests/*.test.mjs # builder URL / HTML / validation tests (Node 20+)
go run .                # run locally on :3000 (needs GITHUB_TOKEN)

node tools/gen_data.mjs # regenerate embedded theme/color/width data from upstream sources
```

`internal/cards/data/*.json` is generated by `tools/gen_data.mjs` and committed. `reference/` and `test/` are local-only research/verification artifacts and are git-ignored.

## Credits & license

This project is licensed under the [GNU Affero General Public License v3.0](LICENSE) (AGPL-3.0-or-later) — the deployed service is a network service, so section 13 applies and the source is kept public here.

It is a Go rewrite of two MIT-licensed projects whose code, layouts, themes and logic it derives from; their copyright and permission notices are preserved in [NOTICE](NOTICE) and apply to the derived portions independently:

- [github-stats-extended](https://github.com/stats-organization/github-stats-extended) — Copyright © 2020 Anurag Hazra and contributors (icons from [Octicons](https://github.com/primer/octicons), Copyright © GitHub Inc.)
- [github-readme-streak-stats](https://github.com/DenverCoder1/github-readme-streak-stats) — Copyright © 2020 Jonah Lawrence
