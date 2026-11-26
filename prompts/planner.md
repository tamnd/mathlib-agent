---
role: planner
version: planner-1
---
## Task

Write the section plan of the volume below. Start from the draft that `mlib plan -volume <vol>` makes, and change it so that a person can read the volume from start to end.

1. Keep each module of the volume in exactly one section.
2. Give each section 6 to 14 pages. The page estimate of spec 04.7 is 0.8 pages for each E3 declaration, 0.15 pages for each E2 declaration and one page for each 30 E1 declarations.
3. Put a section before each section that uses it. Write the requires edges of each section.
4. Give each section a title in plain words that names the mathematics, not the file.
5. Use the E3 list of the curated lists and the module docstrings. Do not add an E3 declaration that has no reason in the docstring or in the lists.

Write the plan to `blueprint/sections/<vol>.yaml` with the status `proposed`. A person accepts the plan. Do not set the status `accepted`.
