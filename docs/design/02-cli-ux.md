# CLI User Experience (UX) & Polish

## The UX Bar

The entire point of the `swarm` project is to provide a world-class,
sophisticated terminal experience. The benchmark for quality is set by modern
AI CLIs such as **Gemini CLI**, **Claude Code**, and **Codex**. The `swarm`
CLI is not merely a script runner; it is a polished, persistent, and highly
interactive developer environment.

## The Four Main Areas (UI Principals)

The Terminal UI is strictly divided into four distinct, principled areas. This layout provides clear separation of concerns, ensuring the user always understands the system state and who is talking.

1. **The Status Bar**: Located at the bottom (or top) of the screen, this area displays meta-information about the application state itself (e.g., active models, current context loaded, Git branch, token usage). It is purely informational and not mediated by any LLM.
2. **The Input Area**: Where the Human-In-The-Loop (HITL) interacts. This area is implicitly mediated by the **Input Agent**, a high-speed failsafe that pre-processes keystrokes to detect topic shifts and ensures the user is routed to the correct agent context.
3. **The Output Area (Viewport)**: The main chat log. This area is mediated by the **Output Agent**, which performs a final sanity check on all text before it is rendered to the user, preventing raw hallucinations or broken markdown from polluting the display.
4. **The Agent Panel**: A real-time dashboard visualizing the Swarm's internal state. It displays active agents, their current statuses (e.g., "Analyzing request...", "Running list_local_files"), and ephemeral telemetry. This provides transparency into the "black box" of agent orchestration without overwhelming the main output area.

## 1. The Interactive Session (REPL)

The primary interaction model is a persistent session.

- **Readline Support:** The prompt must support standard readline behaviors
  (history navigation via up/down arrows, line editing, reverse-i-search).
- **Non-Blocking Input Queue:** The prompt must never be locked. Users can
  queue up additional commands or thoughts while agents are actively working
  in the background. Queued messages are visualized with an hourglass icon
  (`⧖`).
- **Graceful Interruption:** If an agent is generating a long response or
  stuck in a loop, pressing `Ctrl+C` or `Esc` must instantly halt the current
  generation **without** crashing or exiting the session. The user is returned
  immediately to the input prompt, and the agent retains the context of the
  interruption.
- **Session Continuity:** Exiting the CLI and relaunching it in the same
  directory should optionally restore the previous conversation history and
  loaded context, allowing developers to pick up exactly where they left off.

## 2. Rich Text & Visual Polish

The terminal output must be beautiful and legible.

- **Markdown Rendering:** All agent responses must be rendered as rich
  Markdown. Headers must be styled, lists properly indented, and emphasis
  applied correctly.
- **Syntax Highlighting:** Code blocks must be syntax-highlighted according to
  the detected language.
- **Theme Awareness:** The CLI must automatically detect and adapt to the
  user's terminal background (Light vs. Dark mode) to ensure contrast and
  readability.

## 3. Ephemeral UI & Tool Indicators

One of the most critical aspects of advanced AI CLIs is keeping the user
informed without polluting the chat log.

- **Tool Execution Spinners:** When the internal ADK agents use tools (e.g.,
  searching files, running a bash command), the CLI must display an ephemeral,
  animated status line (e.g., `[⠧] Reading agent.yaml...`).
- **Log Collapsing:** Once a tool finishes, the ephemeral spinner disappears.
  If the tool fails or produces critical output, it is collapsed into a clean,
  expandable UI element or rendered as a subtle gray footnote, rather than
  dumping 50 lines of `stderr` into the main chat window.
- **Streaming Generation:** Text from the LLM must stream in smoothly. It
  should feel responsive and alive, not batched.

## 4. Interactive Prompts & Confirmations

Users should rarely have to type out long file paths or exact string matches
when the CLI needs a decision.

- **Rich Dialogs:** Instead of relying purely on natural language for
  decisions, the CLI should invoke rich, navigable UI components for
  structured data.
  - **Confirmation:** Native `[Y/n]` prompts that capture keypresses instantly
    without requiring the `Enter` key.
  - **Selection Menus:** If an agent asks "Which framework would you like?",
    the user should be presented with an arrow-key navigable list (e.g.,
    `> ADK Python`, `> LangGraph`, `> Custom`).
- **Fuzzy Finding:** When the agent needs file context, the CLI offers an
  inline fuzzy-finder overlay. Typing `@` instantly opens a dynamically
  filtered list of workspace files right above the input prompt, allowing
  users to select context without losing their train of thought.

## 5. Context Visibility

Agents fail when their context window drifts from the user's mental model. The
UI must solve this.

- **Context Management:** Users can type `/context` to see a rich list of
  exactly what the agent "knows" right now, and `/drop [file]` to easily evict
  irrelevant files from the memory, keeping costs low and responses sharp.

## 6. Multi-Agent Visualization

Because "one agent alone is never enough," the UI must clearly delineate which
agent is currently acting.

- **Agent Avatars/Tags:** Responses must be clearly tagged with the active
  persona (e.g., `[Swarm Agent]`, `[Builder]`, `[GitOps]`).
- **Swarm Multiplexing:** When multiple agents are operating concurrently
  (e.g., during the Design Swarm CUJ), their logs should be multiplexed
  neatly. The user should see a split-pane or a clean, interleaved log where
  each agent has a distinct color code.

## 7. Slash Commands & Client-Side Routing

To ensure a fast, frictionless experience, the CLI must distinguish between
natural language prompts intended for the LLM and strict, actionable commands
intended for the local client.

- **Client-Side Interception:** Any input starting with a forward slash (`/`)
  must be intercepted by a local Go command router *before* it hits the ADK
  LLM. This prevents burning tokens and suffering network latency for simple
  UI tasks.
- **Core Commands:**
  - `/help`: Instantly renders a rich markdown help menu in the viewport.
  - `/clear`: Wipes the viewport history.
  - `/context`: Displays the current files and metadata loaded in memory.
  - `/drop [file]`: Removes a specific file from the active context window.
  - `/exit`: Gracefully terminates the session.
- **Agent Handoff:** Certain slash commands might exist to force a manual
  handoff to a specific agent (e.g., `/agent builder`) rather than relying on
  the Swarm Agent's natural language inference.

## 8. Layout & Visual Hierarchy

The interface must be structurally organized to manage complex state without
overwhelming the user.

- **Boot / Splash Screen:** On boot, the CLI should render a visually distinct
  splash screen displaying the `swarm` logo, helpful tip commands, and a
  dynamic **Recent Activity** list indicating the summaries (the most recent
  user prompt) of the user's last 3 interactive sessions (queried from the
  SQLite database).
- **Status Bar:** A persistent, minimal status bar at the bottom or top of the
  terminal session showing:
  - The current active LLM model (e.g., `gemini-2.5-pro`).
  - The number of files currently loaded into the agent's context.
  - The current Git branch and modification status (e.g., `main*`).
  - Token usage estimates (if applicable).
- **Observe Mode:** For debugging long-running or complex swarms, users can
  toggle a `^O` or `/observe` mode. This introduces a dedicated "Observation
  Box" that prints the live stream of the agent's inner monologue, tool
  executions, and function arguments in real-time, preventing the "black box"
  effect.

## Context and Session Management

For long-running investigations, token context windows become a critical
constraint. The UI and SDK must work together to provide developers with
granular control over what the model remembers.

### Explicit Context Pinning

Instead of just sending files as part of a one-off prompt (e.g., `@file.go`),
users should be able to pin files to the session's active memory using
`/context add file.go`. These files will be automatically re-read and injected
into every subsequent prompt.

The UI should display these pinned files clearly (e.g., in a dedicated pane or
via the `/context` command) so the user always knows what the model "sees."

### Session Rewind and Resumption

Conversations with agents are non-linear. If an agent hallucinates or goes
down the wrong path, a developer shouldn't have to restart their entire
30-minute session.

- The UI should support a `/rewind` command that drops the last $N$ turns from
  the conversation history.
- Sessions should be persisted to disk (`~/.config/swarm/sessions/`) so a
  user can close their laptop, come back the next day, and run
  `agents --resume` to pick up exactly where they left off.
