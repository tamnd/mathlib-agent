# Changelog

Each release has a section. The newest release is first.

## Unreleased

## v0.6.0 - 2026-11-27

## v0.5.1 - 2026-11-27

### Added

- `mlib coverage -book <dir>` reads the front matter of the sections of the handbook and computes C-home, C-depth and C-E3 (spec 04.5). A module has a home when a section lists it in `modules`. A name in an `explains` list has the depth of the list. A section with `E1: auto` explains every declaration of its modules at E1. `mlib sync -book` and `mlib site -book` use the same numbers.

## v0.5.0 - 2026-11-27

## v0.4.1 - 2026-11-27

## v0.4.0 - 2026-11-27

### Added

- The pages `/teach/` of the site (spec 13.1). The page `/teach/` lists the reading paths of `blueprint/paths.yaml` of the map. Each path has a course page with its volumes, its chapters, the status of the sections and the count of exercises of each grade. The pages `/teach/exercises/` list the exercises of each volume by grade.
- `mlib site build -book <dir>` reads the sections of a checkout of tamnd/mathlib-handbook, for the exercises of the pages `/teach/`.
- The package `teach` reads the reading paths and the exercises.

## v0.3.0 - 2026-11-27

### Added

- `mlib stale -book <dir>` marks the sections that the drift makes stale (spec 05.5). A section is stale when a drift record after its `mathlib` commit has a breaking or a minor item about a declaration of its `explains` list, a name in its text or a module of its `modules` list. A rename or a move item does not make a section stale. With `-apply`, the command sets `status: stale`, `stale_since` and `stale_from` in the front matter, sets the status of the section in the plan, and writes the refresher queue items to `records/stale.json`.
- `mlib sync -book <dir>` writes the stale marks after the drift record.
- The prompt of the exerciser, version exerciser-2, has the rule of decision Q-20: new exercises, or an exercise adapted with a credit line from a source with an open licence.
- The prompt of the refresher, version refresher-2, says to set the status back to `stale_from` and to remove the two stale fields.
- The prompt of the refresher gives the drift items of the section, the commit and the date of each, and the status to set back. A change to an E3 statement or proof of a reviewed section sets the status to checked.

### Changed

- The package `index` reads `proofs.jsonl` of scripts/Index.lean when the index directory has it, and `UsedBy` counts the theorems whose proof uses a declaration. Rule 2 of spec 04.4 now sees the uses in proofs. On the root `Mathlib.Order.Basic`, the E2 declarations go from 5 to 15.
- `mlib index fetch` downloads the asset `proofs-<date>-<commit>.jsonl.gz` when the release has it. It says so when the index has no proofs file.

## v0.2.0 - 2026-11-27

### Added

- `mlib catalogue` and the package `catalogue` write the rows of the Catalogue volumes of spec 06.4, one file `C<NN>.jsonl.gz` for each main volume. Each row has the sentence of the declaration from the E1 record, the E0 family or the docstring, and the field `source` tells which. On Mathlib e54979a532, C05 has 24,928 rows and C13 has 20,493 rows.

## v0.1.1 - 2026-11-26

### Added

- `mlib rename -from <commit> -to <commit>` makes one table of the renames and the module moves of the drift records between two commits, and rewrites the names in the section files of the handbook, the Lean files of the companion and the E1 records. It lists the changes, and writes them only with `-apply`. A rename from a type hash match needs `-proposals`.
- `mlib report -week <week>` and `mlib report -edition <tag>` write the weekly report and the edition report from the records of the map. The weekly report gives an alert when the last sync is more than 30 days old.
- An E1 record can keep the old names of its declaration in `formerly`.
- `mlib queue -role <role>` prints the work queue of a role with the rank of spec 11.5 and the reason of each rank. The planner takes the volumes with no section plan. The informalizer takes the modules with E1 declarations that have no fresh record, in the order of the volumes. The family role takes the E1 declarations with no family, in groups by the end of the name. The writer takes the planned sections of an accepted plan. The exampler, the exerciser and the checker take the drafted sections, and the refresher takes the stale sections.
- `mlib agent run <role>` writes the run record and the prompt of the top item, applies the budget of `agent.yaml` and the ledger, and starts the command of `MLIB_RUNNER` with the prompt on stdin. A role with a small model goes to the large model after two failed runs. The checker refuses the model of the writer of the same section. The prompts of the nine roles are in `prompts/`.
- `blueprint/milestones.yaml` can give the date when a milestone closed and the volumes that it works on. The queue uses them for the weight of a volume.
- `mlib informal status`, `mlib informal stale` and `mlib informal sample`. They read the E1 records `informal/<vol>.jsonl` of spec 09.3. The status gives, for each volume, the E1 declarations and the fresh, stale and missing records, with the results of the round trip, the critic and the human sample. A record is stale when its name is gone or its type hash changed. The sample takes 1% of the records with a fixed seed, as spec 14.3 asks. `mlib check` also checks each record file.
- `mlib plan -volume V13` makes the draft section plan of a volume and writes it to `blueprint/sections/V13.yaml` with `-write`. The chapters come from the prefixes of the volume. A prefix with more than 40 modules splits by its next folder, and small groups join until a chapter has about 20 pages. A section takes modules until it has about 10 pages, with the estimates of spec 04.7. Each section has its modules, the E3 and E2 lists, the families, the requires edges and the counts by depth. The command does not write over a plan with the status `accepted`.
- `mlib family match`, `mlib family list` and `mlib family show`. They read `blueprint/families.yaml` and give each declaration the first family whose pattern it matches. A pattern can test the end of the name, the kind, an attribute, a constant that the declaration uses, a text of the type, a deprecated alias, and an additive or order dual twin. A twin must have the type of its source after the swaps of words and symbols. `mlib family match -out` writes `records/families.json`.
- `mlib check` also checks `blueprint/families.yaml`.

### Changed

- Each command of spec 11.1 is now written, so no command exits with code 3.

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
