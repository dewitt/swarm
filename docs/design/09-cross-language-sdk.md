# Cross-Language SDK Architecture

The `swarm` project is designed as two distinct layers:

1. **The CLI (`cmd/swarm/`)**: A Terminal User Interface and application entry
   point.
1. **The Core SDK (`pkg/sdk/`)**: The embeddable intelligence and
   orchestration engine.

To maximize the impact of the `swarm` ecosystem, the Core SDK must be
accessible to developers working in _any_ programming language, not just Go.
This document outlines the architectural requirements and strategies for
cross-compiling the Go core into native language bindings (Python, TypeScript,
Rust, Java, etc.).

## 1. Core Principles

- **Single Source of Truth:** All core business logic, agent orchestration,
  tool definitions, and skill parsing reside exclusively in the Go `pkg/sdk/`
  implementation.
- **Native Developer Experience:** Consumers of the SDK in Python, TypeScript,
  or Rust should feel like they are using a library native to their ecosystem,
  not an awkward foreign bridge.
- **Consistent Interface:** The API contract must be rigidly defined and
  identical across all languages.
- **No Background Daemons (Where Possible):** Prefer compiling to shared
  libraries (C FFI) or WebAssembly (WASM) over requiring the user to manage a
  separate standalone background gRPC server process, though gRPC/IPC can
  serve as a fallback.

## 2. Interface Definition (Protobuf)

To ensure consistency across boundaries, the `Swarm` interface and all data
structures (e.g., `AgentManifest`, `Skill`, `ToolResult`) must be defined
using Protocol Buffers (`.proto` files).

These protobuf definitions act as the universal contract. We will generate the
Go interface from the proto, and use the proto definitions to generate the
data transfer objects for all target languages.

## 3. Integration Strategies

Based on the target environment, the Go core will be exposed using one of the
following mechanisms:

### A. C Foreign Function Interface (FFI) via cgo

For languages that support binding to C-style shared libraries (`.so`,
`.dylib`, `.dll`), we will compile the Go SDK using
`go build -buildmode=c-shared`.

- **Target Languages:** Python (via `ctypes` or `cffi`), Rust (via `bindgen`),
  C/C++, Ruby.
- **Data Serialization:** Requests and responses across the C boundary will be
  serialized as Protobuf byte arrays to avoid complex manual memory management
  of nested structs in C.

### B. WebAssembly (WASM)

For JavaScript environments, we will compile the Go SDK to WebAssembly
(`js/wasm`).

- **Target Environments:** Node.js, Deno, Bun, and the Browser.
- **Network Considerations:** Because WASM in the browser has restrictions on
  standard socket networking, we must provide a Go adapter that hijacks
  `net/http` calls (specifically the ADK/Gemini API calls) and routes them
  through the native JavaScript `fetch` API via `syscall/js`.

### C. Java Native Access (JNA) / JNI

For JVM-based languages, we will utilize the C-shared library approach in
conjunction with JNA.

- **Target Languages:** Java, Kotlin, Scala.
- **Approach:** JNA provides a much simpler development experience than
  writing custom JNI C code. We will map the Java classes to the exported C
  functions from our Go shared library, passing serialized protobufs.

## 4. Implementation Roadmap

To achieve this, the repository structure will expand to include:

1. **`/proto`**: The Protobuf definitions defining the SDK contract.
1. **`/adapters/cffi`**: Go code exporting the SDK functions via `//export`.
1. **`/adapters/wasm`**: Go code handling JS interop and network fetch
   hijacking.
1. **`/bindings/python`**: The generated Python SDK and native wrappers.
1. **`/bindings/typescript`**: The generated TS SDK and WASM loader.

_Note: This architecture draws inspiration from successful cross-language Go
projects (like Esbuild or Terraform's JS APIs) and internal proofs-of-concept
for unified LLM library distribution._
