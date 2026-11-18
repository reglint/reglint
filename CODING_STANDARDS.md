# Coding Standards

Apply when writing or modifying reglint code.

## Scan behavior

- Keep scans deterministic and reproducible.
- Skip binary/oversized files per spec; record skipped-file stats.
- Treat match text as sensitive; keep it out of console output. Match text is user source code contents, so printing it can leak source into logs.

## Code structure

- Consolidate code paths doing similar work with small variations into shared services with request structs.

## Quality bar

- Coverage gate is 90%; `make quality` enforces it.
- Run `make mutation` in the final stage only — it is slow, so it comes after the cheaper gates are green.
