# mathlib-agent

The toolchain of The Mathlib Handbook. The command is `mlib`.

The handbook explains the mathematics of Mathlib to a student, at the depth that each declaration needs. `mlib` reads the index of Mathlib that [mathlib-lean](https://github.com/tamnd/mathlib-lean) writes at each bump. It computes the depth of each declaration, the coverage of the handbook and the drift of the library from one commit to the next. It checks the map of [mathlib](https://github.com/tamnd/mathlib), ranks the work of the roles, and builds the site https://mathlib.tamnd.com.

`mlib` decides nothing about mathematics. Lean decides what builds, the checks of spec 14 decide what passes, and people decide what is reviewed.

## Install

You need Go 1.27 or later.

```sh
go install github.com/tamnd/mathlib-agent/cmd/mlib@latest
```

Each release on GitHub also has binaries for Linux, macOS and Windows, with checksums and a build provenance attestation.

## Use

Run `mlib help` to see the commands. A usage error exits with code 2. A failed check exits with code 1. The full list of commands is in spec 11.1 of tamnd/mathlib.

### The roles

`mlib queue -role <role>` prints the work queue of a role, with the rank and the reason of each item. The rank is the formula of spec 11.5: the weight of the volume in the current milestone, the planned sections that wait on the item, the level factor of a stale section, the failed runs and the age factor. `mlib stale -map <mathlib> -book <mathlib-handbook>` lists the sections that the drift makes stale. With `-apply`, it writes `status: stale`, `stale_since` and `stale_from` to the front matter, and the refresher queue items to `records/stale.json`. The refresher takes these items first when the volume is released. The roles are planner, informalizer, family, writer, exampler, exerciser, checker, refresher and porter.

`mlib agent run <role>` takes the top of the queue, or the item of `-item`, and writes a run record to `records/runs/<id>/run.json` with the prompt in `prompt.md`. The prompts are in `prompts/<role>.md`. Text from Mathlib goes in marked data blocks. The budget comes from `agent.yaml` of the map and the spend of the day in `agent/ledger.jsonl`. With no `agent.yaml`, the budget is zero and no call goes out. The environment variable `MLIB_RUNNER` names the command that runs the prompt. It reads the prompt on stdin. Use `-dry-run` to see the record and the prompt and to write nothing. mlib sets the status `ran` or `failed`. It never sets `passed`, because Lean, the checks of spec 14 and people decide that.

### Renames and reports

`mlib rename -from <commit> -to <commit>` reads the drift records from the first commit to the second and makes one table of the renames and the module moves. A chain such as a to b and b to c gives a to c. The command lists the changes in the section files of the handbook (`-book`), in the Lean files of the companion (`-lean`) and in the E1 records of the map. It changes the front matter, the code spans with one name, the `<?decl?>` directives, the Lean blocks and the import lines. An E1 record keeps its old names in `formerly`. Give `-apply` to write the changes. A rename that only a type hash match proposes needs `-proposals`, because a person must accept it first.

`mlib report -week 2027-W03` writes the weekly report: the last sync and its age, the drift of the week, the coverage, the runs and the spend by role, and the editions. When the last sync is more than 30 days old, the report gives an alert. `mlib report -edition v4.36.0` writes the report of an edition, with the drift since the last edition. Without `-write`, the command prints the report. With `-write`, it writes `reports/weekly/<week>.md` or `reports/editions/<tag>.md`.

### The Catalogue

`mlib catalogue -index index -map ../mathlib -volume V05,V13 -out catalogue` writes the rows of the Catalogue volumes of spec 06.4, one file `C05.jsonl.gz` for each main volume. The first line is a header with the counts. Each other line is one declaration whose home is in the volume, in module order, with the kind, the attributes, the depth, the home section and a sentence. The sentence is the E1 record when its type hash is current and no check failed. When there is no such record, the sentence is the text of the E0 family, then the first sentence of the docstring. The field `source` tells which. `mlhb catalogue` of tamnd/mathlib-handbook sets the files as books.

### The site

`mlib site build -index index -map ../mathlib -book ../mathlib-handbook -out public` builds the site. The pages `/teach/` give the reading paths of `blueprint/paths.yaml` as courses, with the volumes, the chapters and the count of exercises of each grade. They also list the exercises of each volume by grade: (A) routine, (B) needs an idea, (C) hard and (L) in Lean. Without `-book`, the courses have no chapters and no exercises. `mlib site check -out public` checks the internal links.

## Use as a library

The packages are at the root of the module, so other Go programs can import them. The API can change before v1.0.0.

| Package | Purpose |
|---|---|
| `agent` | rank the work of each role, apply the budget, write the run records and start the runner (spec 11.3 to 11.6) |
| `blueprint` | read and check `blueprint/volumes.yaml` and `blueprint/milestones.yaml` of the map |
| `index` | read the index of Mathlib that mathlib-lean writes |
| `depth` | compute the depth E0 to E3 of each declaration (spec 04.4) |
| `catalogue` | make the rows of the Catalogue volumes (spec 06.4) |
| `coverage` | compute the coverage numbers (spec 04.5) and the module records (spec 09.1) |
| `drift` | compare two indexes and write the drift items (spec 05.4) |
| `family` | read `blueprint/families.yaml` and give each declaration at most one E0 family (spec 09.4) |
| `edition` | read and write the edition log `records/editions.json` (spec 05.7) |
| `informal` | read and check the E1 records `informal/<vol>.jsonl`, find the stale ones and take a sample (spec 09.3, 14.3) |
| `prompts` | the prompt of each role, with its version (spec 11.4) |
| `plan` | make the draft section plan of a volume (spec 09.1, 07.7) |
| `rename` | make the rename table from the drift records and rewrite the names in the book, the companion and the E1 records (spec 05.4) |
| `report` | write the weekly report and the edition report (spec 05.6, 09.1) |
| `site` | build and check the site mathlib.tamnd.com |
| `teach` | read the reading paths of the map and the exercises of the handbook, for the pages `/teach/` (spec 06.6, 13.1) |
| `stale` | find the sections that the drift makes stale and write the stale marks (spec 05.5) |

```go
import "github.com/tamnd/mathlib-agent/index"

ix, err := index.Load("index")
```

## Exit codes

| Code | Meaning |
|---|---|
| 0 | success |
| 1 | a failed check or a failed run |
| 2 | a usage error |
| 3 | the command is not written yet |

## Development

```sh
make check
```

This runs gofmt, go vet, the tests with the race detector and golangci-lint.

## Licence

MIT. See `LICENSE`.
