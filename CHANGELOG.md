# Changelog

Each release has a section. The newest release is first.

## Unreleased

### Added

- The command `mlib` with `help` and `version`. Each other command of spec 11.1 exits with code 3 and names the milestone that adds it.
- The workflows ci, codeql, release and scorecard.
- `mlib volumes check`, `mlib volumes orphans` and `mlib volumes home`. They read `blueprint/volumes.yaml` and give each module the volume of its longest matching prefix.
- `mlib check`. It checks the volume map and the milestones of the map. With `-mathlib` or `-modules` it also checks the volume map against the modules.
