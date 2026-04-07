# Tool Manual Enhancement System — Design Spec

## Overview

Three-component system to identify, improve, and maintain tool manuals in `prompts/tools_manuals/`.

## Components

### 1. Gap Detection

**Purpose:** Identify missing, outdated, and incomplete manuals by comparing dispatch cases against existing files.

**Implementation:**
- Go script: `scripts/tools/gap_detector.go`
- Parses `agent_dispatch_infra.go` for `case "toolname"` patterns
- Compares against `prompts/tools_manuals/*.md`
- Categorizes: **Missing**, **Outdated**, **Unscored**

**Output:**
- Console table summary
- JSON report: `reports/tool_manual_gaps.json`

### 2. Quality Scoring

**Purpose:** Rate existing manuals on a 0-100 scale across 5 dimensions.

**Dimensions & Weights:**
| Dimension | Weight | Description |
|-----------|--------|-------------|
| Operations Coverage | 20% | % of dispatch operations documented |
| JSON Examples | 25% | % of ops with working JSON examples |
| Parameter Explanations | 20% | % of parameters explained with types/defaults |
| Notes/Edge-Cases | 20% | Presence of troubleshooting hints |
| Config Examples | 15% | config.yaml snippets present |

**Implementation:**
- Scoring logic in same `scripts/tools/gap_detector.go`
- Score stored in frontmatter: `manual_score.total`
- Report includes: per-dimension scores + total

### 3. Manual Template

**Purpose:** Consistent structure for new/improved manuals.

```markdown
# {Tool Name} (`{action}`)

{Brief description of what this tool does.}

## Operations

| Operation | Description | Parameters |
|-----------|-------------|------------|
| `op1` | What op1 does | `param1`, `param2` |
| `op2` | What op2 does | — |

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `param1` | string | — | Description of param1 |
| `param2` | integer | `10` | Description of param2 |

## Examples

```json
{"action": "{action}", "operation": "op1", "param1": "value"}
```

## Configuration

```yaml
# Example config.yaml section
```

## Notes

- Important caveat or edge case
- Troubleshooting hint
```

## Quality Score Card (Frontmatter)

Each manual gets frontmatter:

```yaml
---
manual_score:
  operations: 80
  examples: 60
  parameters: 50
  notes: 40
  config: 30
  total: 52
---
```

## Phases

### Phase 1: Gap Detection Script
- Go binary/script in `scripts/tools/`
- Parses dispatch cases
- Generates gap report
- **Deliverable:** `scripts/tools/gap_detector.go`, `reports/tool_manual_gaps.json`

### Phase 2: Quality Scoring Integration
- Extend gap_detector with scoring logic
- Add score to existing manuals' frontmatter
- **Deliverable:** Updated `gap_detector.go`, scored manuals

### Phase 3: Gap Report Analysis
- Run detector, review report
- Create prioritized improvement list

### Phase 4: Manual Improvements (Top 20)
- Improve highest-priority manuals
- Use template + code analysis

### Phase 5: Sync CI/CD (Future)
- Git hook or CI check
- Validates manual exists for new dispatch cases
- **Status:** Deferred — per user priority decision

## Gap Categories

| Category | Definition | Action |
|----------|------------|--------|
| **Missing** | Tool in dispatch but no manual file | Create from template |
| **Outdated** | Manual exists but dispatch has new operations | Add missing operations |
| **Unscored** | Manual exists but no score frontmatter | Run scoring |
| **Low Score** | Score < 50% | Priority improvement |
| **Stale** | Manual older than 6 months, tool changed | Review and update |

## File Structure

```
scripts/tools/
  gap_detector.go      # Main script
  gap_detector_test.go # Tests

reports/
  tool_manual_gaps.json  # Gap analysis output

prompts/tools_manuals/
  *.md                   # All manuals (enhanced)
```

## Success Criteria

- Gap report generated with < 1s runtime
- All manuals scored 0-100
- Top 20 manuals improved to > 70% score
- Manual template consistently applied
