# CUJ: Git-Native Swarm Collaboration on System Design

## User Persona

Sam is an architect starting a greenfield project: designing a multiplayer, old-school ascii roguelike similar to Nethack. 

Instead of brainstorming alone or relying on a single, monolithic LLM prompt that quickly loses context, Sam wants to orchestrate a long-running, asynchronous conversation across the most capable LLM models on the planet. Sam wants them to write, review, critique, improve, and polish a design document better than any single human or AI could alone.

## Journey

### 1. Initiating the Swarm Request

Sam opens their terminal in an empty repository. The repository contains a recently added `ROLES.md` file that defines two personas: `Architect` (optimized for Claude 3.5 Sonnet) and `Reviewer` (optimized for Gemini 2.5 Pro).

```bash
swarm -p "Write a design doc for a old-school ascii roguelike like nethack, but design it to support multiplayer. Engage the Architect and Reviewer roles to iterate on this until the networking latency mitigations are flawless."
```

### 2. The Concierge and Task Breakdown

The Swarm CLI acts as the initial "Mayor" or Concierge. It realizes this is a macro-task requiring coordination. Rather than spinning up a fragile local state machine, it offloads coordination to the universally accepted message bus: GitHub.

> **Swarm CLI:** Understood. This requires a multi-turn peer review. 
> I am creating a tracking issue on GitHub: `#1 Design Multiplayer Nethack Architecture`.
> I am assigning the `Architect` role to draft the initial proposal.

### 3. Asynchronous Drafting (The Architect)

A headless, background Swarm agent adopting the `Architect` persona wakes up. It clones the repo, creates a branch `design/nethack-multiplayer`, and spends several minutes drafting `architecture.md`.

Once complete, it uses the standard `gh pr create` CLI tool to open a Pull Request.

> **[Architect] via GitHub PR Body:**
> "I have drafted the initial architecture. I've chosen a client-prediction UDP model for movement to maintain the fast-paced roguelike feel over a network. Tagging the `@Reviewer` for critique on the state-sync mechanisms."

### 4. Adversarial Peer Review (The Reviewer)

Another Swarm agent process, watching repository events and adopting the hyper-critical `Reviewer` role, receives the GitHub notification. It checks out the PR.

Because it explicitly adopted the `Reviewer` persona defined in `ROLES.md`, its underlying model approaches the document skeptically. It uses the `gh pr review` tool to leave inline comments directly on the Markdown patch.

> **[Reviewer] via GitHub PR Comment (Line 45):**
> "Client prediction for an ascii grid is dangerous. Nethack relies on explicit, deterministic turn-orders (e.g., speed systems). If a player with high ping moves, how do you rollback visual grid updates without intense rubber-banding? Consider a locked-step turn buffer instead."

### 5. Iteration and Polish

The `Architect` wakes up upon receiving the review comment. 

> **[Architect] via GitHub PR Comment:**
> "Excellent point on deterministic speed systems. I will abandon pure client prediction and rewrite the networking section to utilize a dynamic lock-step buffer with 100ms ticks."

The `Architect` pushes a new commit to the branch. The `Reviewer` approves the PR.

### 6. The Handoff

Sam comes back from a coffee break. They open their browser to GitHub and see a polished, peer-reviewed, merged design document that accounts for complex edge cases they hadn't even considered. 

The task is marked complete in the CLI.

> **Swarm CLI:** The Architect and Reviewer have reached consensus. PR #2 has been merged. Would you like me to spawn a `Builder` agent to start scaffolding the UDP server based on the approved architecture?
