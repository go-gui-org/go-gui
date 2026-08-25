---
description:
  Anti-vibecoding learning framework. Generate detailed explanations of code
  written by AI with curated external resources for deeper learning. Use when
  the user wants to understand WHAT and WHY behind AI-generated code, not just
  accept it.
---

# AntiVibe - AI Code Learning Framework

## Purpose

AntiVibe generates **learning-focused explanations** of AI-written code. It is
not a generic summary. It creates educational content that helps developers
understand:

- **What** the code does (its function)
- **Why** it was written this way (design decisions)
- **When** to use these patterns (context)
- **What alternatives** exist (broader knowledge)

## When to Use

Use AntiVibe when:

1. **Manual invocation**: User types `/antivibe` or "deep dive"
2. **Post-task learning**: After a feature/phase completes, user wants to learn
   from it
3. **Proactive**: User says "explain what AI wrote", "learn from this code", or
   "understand what AI wrote"

## What AntiVibe Produces

Output is saved to the `deep-dive/` folder as markdown:

```
deep-dive/
├── auth-system-2026-01-15.md
├── api-layer-2026-01-15.md
└── database-models-2026-01-15.md
```

Each file contains:

- **Overview**: What this code does and why it exists
- **Code Walkthrough**: File-by-file explanation with line-by-line notes
- **Concepts Explained**: Design patterns, algorithms, CS concepts used
- **Learning Resources**: Curated docs, tutorials, videos
- **Related Code**: Links to other files in the codebase

## Workflow

### Step 1: Identify Code to Analyze

- Check the user request for an explicit file list
- Use `git diff` to find recently modified or created files
- Ask the user which files or components they want to understand

### Step 2: Analyze Code Structure

For each file:

- Identify the main purpose and the responsibilities
- Note key functions, classes, modules
- Identify the design patterns used (factory, singleton, observer, and more)
- Find any complex logic or algorithms

### Step 3: Explain Concepts

For each concept or pattern found:

- **What**: Plain-language explanation
- **Why**: Why this approach was chosen over the alternatives
- **When**: When to use this pattern (with context)
- **Alternatives**: Other approaches and trade-offs

### Step 4: Find External Resources

Search for and include:

- Official documentation for the libraries and frameworks used
- High-quality tutorials and blog posts
- Video resources (if available)
- Related concepts for further learning

### Step 5: Generate Output

Create the markdown file in the `deep-dive/` folder:

- Name format: `[component]-[timestamp].md`
- Follow the template in `templates/deep-dive.md`
- Include code snippets where they help
- Make it educational, not just descriptive

## Configuration

AntiVibe can be configured to auto-trigger via hooks:

- **SubagentStop**: After a Task completes a feature
- **Stop**: At session end

To enable auto-trigger, configure the hooks in your project. See
`hooks/hooks.json`.

## Principles

1. **Why over what** - Always explain design decisions
2. **Context matters** - Explain when/why to use patterns
3. **Curated resources** - Quality links, not random Google results
4. **Phase-aware** - Group by implementation phase
5. **Learning path** - Suggest next steps for deeper study
6. **Concept mapping** - Connect code to underlying CS concepts

## Dependencies

Optional scripts live in the `scripts/` folder:

- `capture-phase.sh` - Detect implementation phase boundaries
- `analyze-code.sh` - Parse code structure
- `find-resources.sh` - Search for external resources
- `generate-deep-dive.sh` - Create markdown output

These scripts are helpers. You can also do everything with direct code analysis.

## Examples

**Input**: "Explain the auth system Claude wrote" **Output**:
`deep-dive/auth-system-2026-01-15.md` containing:

- JWT structure explanation
- Password hashing rationale
- Session management concepts
- Learning resources for auth patterns

**Input**: "I want to understand this API layer" **Output**:
`deep-dive/api-layer-2026-01-15.md` containing:

- REST design decisions
- Middleware explanation
- Error handling patterns
- Further reading on API design
