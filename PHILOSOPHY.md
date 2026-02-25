# Project Philosophy (PHILOSOPHY.md)

This document outlines the core beliefs and guiding principles of the `agents`
project. When faced with an architectural decision, always favor the path that
aligns closest to these ideals.

## 1. Defer to the Frontier (Thin Software, Fat Models)

Foundational, frontier LLMs are improving at a rate faster than traditional
software built to orchestrate them possibly can.

Therefore, our primary goal is to **delegate and defer as much logic as
possible to the models (and the agents that drive them).**

- **Avoid Hardcoding:** Do not write thousands of lines of Go code to parse a
  specific API or manage a proprietary framework if an LLM can achieve the
  same result by reading a markdown file.
- **Thin Capabilities:** Core capabilities must be as thin and easy to update
  as possible. This is why we use dynamic **Skills** (plain-text instructions
  and simple tool manifests) instead of monolithic binary plugins.

## 2. One Agent Alone is Never Enough

Single-agent architectures are fragile. Real-world, complex tasks require
specialized context, debate, and iterative verification loops.

- The system must natively assume and support multi-agent collaboration
  (Swarms, Supervisor-Worker patterns, Debate teams).
- Every problem should be approached by asking: *"Can we split this task among
  specialized sub-agents?"*

## 3. Zero-HITL (Human-In-The-Loop) for Verification

Agents must respect the human developer's time and attention.
Human-In-The-Loop should *only* be required for permissions (e.g., "Can I push
to `main`?") or creative opinions (e.g., "Do you like this UI layout?").

- **Mechanical Verification is Autonomous:** An agent must never ask a human
  to run a binary just to verify if it compiled correctly or to describe what
  the UI looks like.
- Agents must utilize headless testing, unit tests, and snapshot testing to
  verify their own work autonomously.

## 4. GitOps is the Source of Truth

We reject proprietary, black-box deployment engines.

- Version control (Git) is the absolute source of truth for both code and
  infrastructure.
- "Deploying" an agent means scaffolding standard CI/CD pipelines (e.g.,
  GitHub Actions) and committing them to the repository, ensuring every change
  is versioned, auditable, and easily reversible.

## 5. World-Class CLI UX

A command-line tool for orchestrating AI should feel as magical and polished
as the AI it commands.

- We hold ourselves to the standard of tools like Gemini CLI, Claude Code, and
  Codex.
- The UI must be highly interactive, visually beautiful (rich text, colors,
  ephemeral spinners), and completely hide the mechanical complexity of the
  underlying LLM calls.
