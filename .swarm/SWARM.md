# Swarm CLI Mandates

## Markdown Formatting
- All markdown files in this repository (except for `SKILL.md` files) must be formatted using `mdformat --wrap 78` before being committed.
- **NEVER** run `mdformat` on `SKILL.md` files (found in subdirectories of `skills/`). These files contain YAML frontmatter that is critical for the Swarm SDK to parse agent metadata, and `mdformat` will corrupt this structure.
- Quotation marks must be used exclusively for direct, verbatim quotes from sources or specific UI elements. Avoid using quotation marks for emphasis, technical terminology, neologisms, or novel phrases. Technical terms do not require "scare quotes" or call-outs.
