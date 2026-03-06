# Swarm User Experience (UX) Review Guide

This document contains specialized instructions for an AI agent (or human
contributor) to perform a comprehensive, user-centric evaluation of the Swarm
CLI. The objective of this process is to step outside the perspective of a
developer and rigorously critique the product from the viewpoint of a
dedicated, discerning customer.

## When to Use This Guide

You should execute this workflow whenever a user requests a "UX review," a
"usability audit," a "user advocate pass," or a "friction analysis."

## Core Objectives

1. **Friction Hunting**: Identify any moment in the workflow where the user
   has to wait unnecessarily, read confusing output, or perform repetitive
   manual steps.
1. **Discoverability**: Critique how easily a new user can figure out advanced
   features (like Plan Mode, Web Panel, or Skills) without reading the entire
   `README.md`.
1. **Aesthetic & Layout Polish**: Analyze the Terminal UI and Web Panel for
   visual hierarchy issues, awkward text wrapping, inconsistent color coding,
   or poor use of terminal real estate.
1. **Error Empathy**: Evaluate how the system behaves when things go wrong
   (e.g., network failures, missing API keys, invalid commands). Does it crash
   abruptly, print a stack trace, or gracefully guide the user to a solution?

______________________________________________________________________

## The UX Review Workflow

When instructed to perform a UX review, you must adopt the persona of a
senior, slightly impatient engineer who expects tools to "just work." Do not
excuse bad UX just because the underlying code is clever.

### Phase 1: Onboarding & First Impressions

Analyze the immediate out-of-the-box experience:

- **The Blank Slate**: What does the user see immediately after launching
  `./bin/swarm` for the first time? Is it overwhelming? Is it too empty?
- **Context Establishment**: Does the CLI clearly explain what it's capable
  of? Are the `Quick Tips` actually useful, or just noise?
- **Configuration Friction**: How annoying is it to set up the LLM provider or
  change the active model?

### Phase 2: The Core Loop (Input -> Thinking -> Output)

Simulate or review the primary interactive loop:

- **Responsiveness**: When the user presses Enter, is there immediate visual
  feedback? Are the loading states (spinners, "Agents are working...")
  distinct and reassuring?
- **Information Density**: When an agent streams a massive markdown response,
  does the viewport handle it gracefully? Are code blocks easily
  distinguishable?
- **The Agent Panel**: Does the dynamic task panel actually help the user
  understand what the swarm is doing, or is it distracting? Does it flicker or
  jump around annoyingly?

### Phase 3: Advanced Workflows & Edge Cases

Examine the "power user" features:

- **Slash Commands**: Review the UX of `/context`, `/plan`, `/skills`, etc.
  Are the success/error messages clear?
- **Web Panel**: Is the transition from terminal to browser
  (`http://localhost:5050`) seamless? Does the web UI feel like a natural
  extension of the CLI?
- **Interruptions**: What happens if the user presses `Ctrl+C` or `Esc` while
  an agent is generating a massive diff? Is the interruption respected
  immediately?

### Phase 4: Reporting and Execution

Synthesize your findings into a structured "User Advocate Report."

1. **Check for Duplicates**: Before finalizing your report, read the
   `docs/UX_ISSUES.md` file in the project root. Check if any of your findings
   have already been logged.
1. **Update the Backlog**: Append any *new* unique findings or significant new
   context for existing issues to `docs/UX_ISSUES.md`.
1. **Group the findings** into categories:
   - 🚨 **High Friction** (Must fix: active user pain points, confusing
     errors).
   - 🎨 **Aesthetic Polish** (Visual inconsistencies, layout jumps).
   - 💡 **Feature Proposals** (Ideas that would delight the user).
1. Frame your critiques empathetically. (e.g., instead of *"the model fails if
   no API key is present,"* write *"If a user forgets their API key, the app
   just prints a raw error and gives up. We should intercept this and prompt
   them to enter it."*)
1. Present the report to the user and ask: *"Which of these UX improvements
   would you like to implement first?"*
1. Proceed iteratively with the user's approval.
