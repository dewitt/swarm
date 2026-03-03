# CUJ: Multi-Model Code Review and Consensus

## User Persona

Taylor is a Tech Lead who has just finished a massive refactor of the core caching system in their Go application. Due to the complexity and risk of race conditions, Taylor wants a comprehensive code review. 

Instead of asking one AI model and getting a single biased opinion, Taylor wants to leverage the diverse architectural training weights of **Codex**, **Claude 3.5 Sonnet**, and **Gemini 2.5 Pro** simultaneously to debate the implementation, spot edge cases, and ideally reach a consensus before merging.

## Journey

### 1. Initiating the Swarm Critique

Taylor opens their terminal in the feature branch and uses the CLI to dispatch a multi-model swarm. Taylor implicitly sets up a Git-Native workflow by forcing the agents to collaborate on the existing GitHub Pull Request.

```bash
swarm -p "Review my open PR #42 for the caching refactor. I want a deep dive from Claude, Gemini, and Codex. Evaluate for race conditions and cache stampedes. Each of you should write your independent review as a PR comment. Then, read each other's reviews and debate any disagreements in a unified thread until you reach a consensus on what must be changed."
```

### 2. The Concierge Dispatch

The Swarm CLI (running locally as the Mayor/Concierge) parses the intent. It notices the requirement for three distinct models.

> **Swarm CLI:** Understood. I am dispatching three independent agent instances:
> - **Agent 1 (Model: Claude 3.5 Sonnet)**: Assigned to Review
> - **Agent 2 (Model: Gemini 2.5 Pro)**: Assigned to Review
> - **Agent 3 (Model: Codex)**: Assigned to Review
>
> They will post their feedback to PR #42 asynchronously.

### 3. Independent Analysis

The three agents pull the diff for PR #42 concurrently using the standard `gh pr diff` tool. Because their underlying model architectures differ, they catch different nuances.

Each agent posts their initial findings as a standalone comment on the Pull Request.

> **[Claude] via GitHub PR Comment:**
> "The implementation looks solid, but I spotted a potential cache stampede vector on line 124. If the Redis key expires under high load, multiple goroutines will bypass the `sync.RWMutex` and simultaneously hit the database."

> **[Gemini] via GitHub PR Comment:**
> "Code is clean. I do not see a stampede risk because the `singleflight` package is imported, but it appears to be initialized incorrectly on line 45, meaning it's creating a new Group per request instead of globally."

> **[Codex] via GitHub PR Comment:**
> "The `Unmarshal` step in the `Get` method does not handle corrupted JSON payloads gracefully, which will cause a panic if the Redis cache is poisoned."

### 4. The Peer Review Debate

The Swarm Concierge monitors the PR. Once all three initial reviews are posted, it instructs the sub-agents to read the thread (`gh pr view --comments`) and synthesize a consensus.

> **[Claude] via GitHub PR Comment (Reply):**
> "@Gemini, excellent catch on the `singleflight` initialization. That directly mitigates my concern about the cache stampede if fixed. I agree that line 45 is the root issue."
>
> **[Gemini] via GitHub PR Comment (Reply):**
> "Agreed. Additionally, @Codex is correct about the JSON panic. We need a fallback mechanism if `json.Unmarshal` fails."

### 5. Final Consensus

After a few minutes of threaded debate, the agents align on the true risks of the PR. A designated "Lead Reviewer" (or whichever agent decides to step up) posts a formal, synthesized summary of the required changes.

> **[Gemini] via GitHub PR Comment:**
> "**Consensus Reached.** @Taylor, the Swarm recommends the following blocking changes before merge:
> 1. Hoist the `singleflight.Group` initialization out of the route handler and into the global `Cache` struct (Fixes cache stampedes).
> 2. Add an explicit error check and cache-eviction fallback after `json.Unmarshal` on line 130 to prevent panics from corrupted data.
> 
> The logic is otherwise sound with no detectable race conditions. Let us know when you've pushed the fixes!"

### 6. Developer Resolution

Taylor receives an incredibly high-signal, peer-reviewed summary of their code. They didn't have to read three disjointed, conflicting reports. The agents debated the nuances and presented a unified, actionable front. Taylor makes the two fixes, pushes the code, and merges the PR with absolute confidence.
