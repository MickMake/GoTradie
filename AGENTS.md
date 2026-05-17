# Workspace layout

This Codex environment may contain three sibling repositories:

- `/workspace/GoBunningsNinja`
- `/workspace/GoBunnings`
- `/workspace/GoInvoiceNinja`

Treat `/workspace/GoBunningsNinja` as the primary repository unless told otherwise.

When changes span repositories:
- show `git status --short` in each repo
- show `git diff --stat` in each repo
- do not commit until the changed repo list is confirmed
- do not push unless explicitly requested

Use Go 1.22.

Run tests with:

```bash
go test ./...
```
