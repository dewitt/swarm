# CUJ: Autonomous Swarm-based Skill Refinement via Web Research and GitOps

## User Persona

Jordan is the maintainer of the `swarm` CLI project. They want to ensure the
internal Skills (like `builder_agent`, `gitops_agent`, or `adk-skill`) are
up-to-date with the latest framework APIs, best practices, and ecosystem
standards without manually rewriting them.

## Journey

### 1. Initiating the Swarm Request

Jordan opens the terminal in the root of the `swarm` repository and launches
the CLI.

```bash
agents
```

> **Jordan:** I want you to review all of our current dynamic skills in the
> `skills/` directory. Please use a swarm of expert agents to deeply research
> the latest documentation on the web for their respective domains (like the
> newest Google ADK updates, LangGraph patterns, or GitHub Actions best
> practices). Once researched, update the `SKILL.md` files to be even better.
> Do this safely on a new branch.

### 2. Orchestrating the Swarm and Environment Setup

The internal Router Agent understands the complexity and the need for safety.
It first delegates to a GitOps Expert to prepare the workspace, then
instantiates the Swarm.

> **Swarm CLI:** Acknowledged. This is a complex operation requiring external
> research and file modifications.
>
> **\[GitOps Expert\]:** Creating a new branch
> `feature/skill-refinements-$(date +%s)` to ensure we do not disrupt your
> current working tree.
>
> **Swarm CLI:** Instantiating a Refinement Swarm (Web Researcher, Skill
> Architect, Git Practitioner)...

### 3. Deep Research & Analysis

The UI multiplexes the swarm's activity as they work concurrently on different
skills.

> **\[Web Researcher\]:** Searching the web for "Google Agent Development Kit
> Go latest API changes" to validate the `adk-skill`... **\[Web
> Researcher\]:** Reading `llms.txt` from the official LangGraph repository to
> update the `builder` skill... **\[Skill Architect\]:** Reviewing the current
> `skills/gitops/SKILL.md`. It lacks instructions for rollback strategies. I
> will incorporate the Researcher's findings on GitHub Actions rollback
> patterns.

### 4. Human-In-The-Loop (HITL) Confirmation

Before making destructive or broad sweeping changes to the core system
instructions, the Swarm pauses to get Jordan's opinion on a structural change.

> **\[Skill Architect\]:** I propose splitting the `adk-skill` into separate
> language-specific Markdown files (e.g., `adk-go.md`, `adk-python.md`) rather
> than keeping them in one massive `SKILL.md` file to reduce context bloat for
> the LLM.
>
> **Swarm CLI:** Do you approve this structural refactor for the ADK skill?
> (Y/n)

Jordan presses `Y`.

### 5. Execution and Verification

The swarm proceeds to apply the changes, utilizing local tools to write the
files and test the formatting.

> **\[Skill Architect\]:** Rewriting `skills/adk-skill/SKILL.md` as a router,
> and creating `skills/adk-skill/references/adk-go.md`... **\[Git
> Practitioner\]:** Running `mdformat --wrap 78` on all modified `SKILL.md`
> files to ensure they meet repository standards. **\[Git Practitioner\]:**
> Staging changes. I've grouped the updates into atomic commits:
>
> 1. `refactor(skills): modularize adk-skill for better context management`
> 1. `docs(skills): update gitops agent with modern rollback strategies`

### 6. Final Review and Handoff

The CLI presents the final state to Jordan.

> **Swarm CLI:** The swarm has completed the refinement process.
>
> - We are currently on branch: `feature/skill-refinements-1718293847`
> - All skills have been updated with the latest web-researched best
>   practices.
> - Commits have been created locally.
>
> Would you like me to push this branch and open a Pull Request for your final
> review, or would you like to inspect the diffs locally first?

> **Jordan:** Push it and open a PR.

> **\[Git Practitioner\]:** Pushing to
> `origin feature/skill-refinements-1718293847` and creating a Pull Request
> via `gh pr create`.

Jordan can now review the high-quality, researched improvements
asynchronously.
