# Agent Guidelines

Read `CODING_STANDARDS.md` when writing or modifying code: scan behavior, code structure, and the quality bar live there.

## Workflow

1. **Spec-first.** Before any feature work, read `specs/README.md`, the index of design docs and feature specs. Specs state intent, not implementation: implement to their patterns and data shapes, update them only when asked. Done when you can name the spec your change implements.
2. **Ground truth.** Verify in the codebase before claiming something exists. Done when every existence claim traces to code you read.
3. **Red-first.** Write the failing test, then make it pass. Spec-writing stops at the spec; tests belong to implementation. Done when each new behavior went red before green.
4. **Quality gates.** Run `make quality` (`make help` self-documents every target). Done when `make quality` is green.
