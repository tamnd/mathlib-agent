# Changelog

Each release has a section. The newest release is first.

## Unreleased

### Added

- `mlib family match`, `mlib family list` and `mlib family show`. They read `blueprint/families.yaml` and give each declaration the first family whose pattern it matches. A pattern can test the end of the name, the kind, an attribute, a constant that the declaration uses, a text of the type, a deprecated alias, and an additive or order dual twin. A twin must have the type of its source after the swaps of words and symbols. `mlib family match -out` writes `records/families.json`.
- `mlib check` also checks `blueprint/families.yaml`.

## v0.1.0 - 2026-11-26

### Changed

- The packages move from `internal/` to the root of the module, so that other Go programs can import them. The README lists the packages.

## v0.0.2 - 2026-11-26

## v0.0.1 - 2026-11-26

### Added

- The command `mlib` with `help` and `version`. Each other command of spec 11.1 exits with code 3 and names the milestone that adds it.
- The workflows ci, codeql, release and scorecard.
- `mlib volumes check`, `mlib volumes orphans` and `mlib volumes home`. They read `blueprint/volumes.yaml` and give each module the volume of its longest matching prefix.
- `mlib check`. It checks the volume map and the milestones of the map. With `-mathlib` or `-modules` it also checks the volume map against the modules.
- `mlib index fetch` and `mlib index show`. The fetch command downloads the index asset of a Mathlib commit from the release `index` of tamnd/mathlib, and the curated lists of Mathlib at the same commit.
- `mlib depth`. It gives each declaration the depth E0, E1, E2 or E3 with the rules of spec 04.4, and the reason for the depth.
- `mlib coverage`. It writes `records/coverage.json` and `records/modules.json` from an index and the volume map.
- `mlib site build` and `mlib site check`. The build writes mathlib.tamnd.com: one page for each module with an anchor for each declaration, the areas, the coverage, the drift and a lookup of a declaration by name. The check finds missing pages, broken internal links and files over the limits of Cloudflare.
- `mlib sync`. It compares two indexes, puts each change in a drift class of spec 05.4 and writes `records/drift/<date>.json`.
- `mlib edition new`, `mlib edition list` and `mlib edition show`. The new command opens an edition at a stable Mathlib tag `v4.X.0` and writes it to `records/editions.json` with the commit of the tag, the Lean version, the date of the tag and the commit of the branch `edition/v4.X.0` of mathlib-lean. If an edition is open, it closes as skipped. A patch tag does not open an edition.
- `mlib check` also checks `records/editions.json` when the file exists.

### Fixed

- The lookup page gave `Nat.add_comm` as its example. This name is in core Lean, so the index does not have it. The example is now `Nat.Prime.two_le`, and the text tells the reader that names of core Lean and Batteries are not in the index.
