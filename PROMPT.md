This is a new project to build a CLI for managing, building, and deploying agents.

This project has several requirements:

- It should have a clean separation between the core business logic, which can be exposed as an embeddable SDK (likely wrapped and exposed via native-language wrappers) and terminal (and optionally other) UIs.
- It should be called 'agents', and should support a user-friendly installation flow as simple as "brew install agents".
- It should be built on ADK if possible to power the core business logic internally, but that's an implementation detail
- It should be able to work with agents written in all types of frameworks, from ADK to langchain/langgraph, etc.
- It should be highly modular, in sofar as adding new capabitities or evolving the capabilites is pluggable by design (e.g., adding support for something like managing agent swarms is just a plugin)
- It should be as small and light as possible, deferring as much of the heavy lifting to the LLMs that power it, and favoring lightweight, dynamic extensibility like Skills over hardcoding logic
- It should be rigorously testable 
- It should be built with "git ops" in mind from the beginning, integrating the development and deployment workflow into github processes
- It should probably be written entirely in Go, since that is a safe, easy to code language, that also cross-compiles well (cgo and wasm) if we want to expose the SDK portion in other languages
- It should offer an out-of-the box command line experience consistent with Gemini CLI or Claude Code or Codex

Your job is to iterating on this proposal and these requirements to begin writing high-level design documents that do a deep dive into all the various aspects and start creating a robust and detailed set of designs.

I will ask other agents to collaborate on this design, critique it, and offer improvements.

You should be thinking about multi-agent usecases throughout. One agent alone is never enough.
