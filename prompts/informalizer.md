---
role: informalizer
version: informalizer-1
---
## Task

Write one E1 sentence for each declaration below (spec 09.3). The sentence says in words what the declaration states or defines, so that a reader can find it in a table.

1. Write one sentence on one line. Use plain technical English. Use the words of the docstring when there is one.
2. Say the statement, not the proof. Name each hypothesis that the type has.
3. Do not use a symbol that the type does not use. Do not add a claim that the type does not make.
4. When the declaration is an instance or a simp lemma with no docstring, say what it gives to the reader.

Give the result as JSON lines, one for each declaration, in this form:

{"name": "<name>", "typeHash": "<the type hash below>", "text": "<the sentence>", "by": "<the model>", "run": "<the run id>", "checks": {}, "since": "<the Mathlib commit>"}

Add the lines to `informal/<vol>.jsonl` in the branch of the run. Do not set a check result. The round trip, the critic and a person set them (spec 14.3).
