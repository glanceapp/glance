# Ops Centre: inspiration and what to borrow

Base: this repo is a fork of [Glance](https://github.com/glanceapp/glance). We keep Glance's Go backend, YAML-driven layout and widget system, and repurpose it as a personal ops command centre with a terminal look.

## Borrow from Arwes (https://github.com/arwes/arwes)

The sci-fi HUD feel, layered on top of Glance's panels (not a rewrite to React):

- Animated frames and corner-cut borders around panels
- Text "decipher" / scramble-in effect on headings and values
- Subtle glowing grid / dot backgrounds and moving light lines
- Optional UI sounds on interactions (off by default)
- Staggered panel entrance animations

Look at the `/demos` and `/docs` pages in the Arwes repo for reference. Recreate the effects in plain CSS/JS where practical.

## Borrow from m4tt72/terminal (https://github.com/m4tt72/terminal)

- A command prompt you can type into (e.g. `help`, `theme`, `open <bookmark>`, `search <q>`), with history and tab completion
- Its command registry pattern: each command is a small function registered by name
- The theme system: lots of terminal colour schemes in `themes.json`, switchable at runtime
- ASCII banner on load

## Look so far

`config/glance.yml` and `assets/terminal.css` are a working Glance config and stylesheet with a phosphor-green "OPS" page: monospace font, square bordered panels, CRT scanline overlay, Brisbane weather, UTC/PT clocks, server stats, service checks, HN/Lobsters, markets, releases.
