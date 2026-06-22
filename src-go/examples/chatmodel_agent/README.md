# ChatModelAgent Example

This example is an isolated Eino ADK learning spike. It demonstrates a
`ChatModelAgent` with one local `greet` tool and a deterministic fake model, so
the example runs without network access or provider credentials.

The core `llm chat` and `llm toolcall` commands intentionally do not depend on
this package. Product runtime code remains compose-first until an agentic mode
has a concrete requirement.

Run it with:

```bash
go test ./examples/chatmodel_agent -v
go run ./examples/chatmodel_agent
```
