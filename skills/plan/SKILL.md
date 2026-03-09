---
name: maggus-plan
description: "Generate a Implementation for one or more new features. Use when planning a feature, starting a new project, or when asked to create implementation plan. Triggers on: create a plan, write plan for, plan this feature"
user-invocable: true
---

# Implementation Plan Generator

Create detailed product requirements that are clear, actionable, and suitable for implementation.

---

## The Job

1. Receive a feature description from the user
2. Ask 3-5 essential clarifying questions (with lettered options)
3. Generate a structured implementation plan based on answers
4. Save to `.maggus/plan-[rolling_number].md`

**Important:** Do NOT start implementing. Just create the implementation plan.

---

## Step 1: Clarifying Questions

Ask only critical questions where the initial prompt is ambiguous. Focus on:

- **Problem/Goal:** What problem does this solve?
- **Core Functionality:** What are the key actions?
- **Scope/Boundaries:** What should it NOT do?
- **Success Criteria:** How do we know it's done?

### Format Questions Like This:

```
1. What is the primary goal of this feature?
   A. Improve user onboarding experience
   B. Increase user retention
   C. Reduce support burden
   D. Other: [please specify]

2. Who is the target user?
   A. New users only
   B. Existing users only
   C. All users
   D. Admin users only

3. What is the scope?
   A. Minimal viable version
   B. Full-featured implementation
   C. Just the backend/API
   D. Just the UI
```

This lets users respond with "1A, 2C, 3B" for quick iteration. Remember to indent the options.

---

## Step 2: Implementation plan structure

Generate the plan with these sections:

### 1. Introduction/Overview
Brief description of the feature and the problem it solves.

### 2. Goals
Specific, measurable objectives (bullet list).

### 3. User Stories
Each story needs:
- **Title:** Short descriptive name
- **Description:** "As a [user], I want [feature] so that [benefit]"
- **Acceptance Criteria:** Verifiable checklist of what "done" means

Each story should be small enough to implement in one focused session.

**Format:**
```markdown
### TASK-001: [Title]
**Description:** As a [user], I want [feature] so that [benefit].

**Acceptance Criteria:**
- [ ] Specific verifiable criterion
- [ ] Another criterion
- [ ] Typecheck/lint passes
- [ ] **[Backend stories only]** Unit tests are written and successful
- [ ] **[UI stories only]** Verify in browser using dev-browser skill
```

**Important:** 
- Acceptance criteria must be verifiable, not vague. "Works correctly" is bad. "Button shows confirmation dialog before deleting" is good.
- **For any story with UI changes:** Always include "Verify in browser using dev-browser skill" as acceptance criteria. This ensures visual verification of frontend work.

### 4. Functional Requirements
Numbered list of specific functionalities:
- "FR-1: The system must allow users to..."
- "FR-2: When a user clicks X, the system must..."

Be explicit and unambiguous.

### 5. Non-Goals (Out of Scope)
What this feature will NOT include. Critical for managing scope.

### 6. Design Considerations (Optional)
- UI/UX requirements
- Link to mockups if available
- Relevant existing components to reuse

### 7. Technical Considerations (Optional)
- Known constraints or dependencies
- Integration points with existing systems
- Performance requirements

### 8. Success Metrics
How will success be measured?
- "Reduce time to complete X by 50%"
- "Increase conversion rate by 10%"

### 9. Open Questions
Remaining questions or areas needing clarification.

---

## Writing for Junior Developers

The implementation plan reader may be a junior developer or AI agent. Therefore:

- Be explicit and unambiguous
- Avoid jargon or explain it
- Provide enough detail to understand purpose and core logic
- Number requirements for easy reference
- Use concrete examples where helpful

---

## Output

- **Format:** Markdown (`.md`)
- **Location:** `.maggus/`
- **Filename:** `plan_[rolling-number].md` (snake-case)

---

## Example Plan

```markdown
# Plan: Leberkassemme

## Introduction

Build a traditional Bavarian Leberkassemme — a warm slice of freshly baked Leberkäs served in a crispy Semmel (bread roll), optionally with sweet mustard. This is the quintessential Bavarian street food, found at every Metzger (butcher) and Imbiss across Bavaria.

## Goals

- Produce a Leberkäs with a crispy golden-brown crust and a smooth, juicy interior
- Serve on a fresh Semmel that holds up structurally without going soggy
- Achieve the classic flavor balance of savory meat, crusty bread, and sweet mustard
- Keep preparation approachable for a home kitchen

## User Stories

### TASK-001: Prepare the Leberkäs Brät
**Description:** As a cook, I want to prepare the Leberkäs meat mixture (Brät) so that it has the right smooth, emulsified texture.

**Acceptance Criteria:**
- [ ] Pork, beef, and back fat are finely ground (twice through the finest plate)
- [ ] Onion is finely diced and mixed in
- [ ] Brät is seasoned with salt, pepper, marjoram, and a pinch of nutmeg
- [ ] Mixture is smooth and paste-like with no visible chunks
- [ ] Ice water added during mixing to keep temperature below 12°C

### TASK-002: Bake the Leberkäs
**Description:** As a cook, I want to bake the Leberkäs loaf so that it develops a crispy crust while staying juicy inside.

**Acceptance Criteria:**
- [ ] Brät is pressed into a greased loaf pan (Kastenform), slightly domed on top
- [ ] Top is scored in a diamond pattern and brushed with water
- [ ] Oven preheated to 180°C (convection)
- [ ] Baked for approximately 60–75 minutes until core temperature reaches 69°C
- [ ] Crust is golden-brown and slightly cracked along the score lines

### TASK-003: Source or bake the Semmeln
**Description:** As a cook, I want fresh Semmeln so that the bread is crusty outside and soft inside.

**Acceptance Criteria:**
- [ ] Semmeln are fresh from the bakery or baked same-day
- [ ] Crust is crispy and audibly crunches when pressed
- [ ] Interior is soft and airy, not dense or stale
- [ ] Size is appropriate — roughly 10 cm diameter

### TASK-004: Assemble the Leberkassemme
**Description:** As a hungry Bavarian, I want a properly assembled Leberkassemme so that every bite has the right ratio of bread, meat, and mustard.

**Acceptance Criteria:**
- [ ] Semmel is sliced open horizontally (not all the way through)
- [ ] One thick slice of Leberkäs (approx. 1–1.5 cm) is cut fresh from the loaf while still warm
- [ ] Leberkäs slice fits the Semmel — trimmed or folded if needed, no tiny slice in a huge roll
- [ ] Sweet mustard (süßer Senf) applied generously on one side
- [ ] Served immediately while Leberkäs is still warm

## Functional Requirements

- FR-1: Leberkäs Brät must be emulsified to a smooth consistency — no coarse mince texture
- FR-2: Baking must produce a crust that is visibly golden-brown and slightly crisp
- FR-3: Core temperature of the finished Leberkäs must reach at least 69°C
- FR-4: Semmel must be fresh — maximum 4 hours old at time of serving
- FR-5: Assembly must happen immediately after slicing the Leberkäs to preserve warmth
- FR-6: Sweet mustard (Händlmaier's or equivalent) must be available as condiment

## Non-Goals

- No Weißwurst, no Brezen — this is about the Leberkassemme only
- No fancy toppings like lettuce, tomato, or cheese (this is not a burger)
- No Pizzaleberkäs or Käsleberkäs variants in this iteration
- No deep-frying or pan-frying — oven-baked only

## Technical Considerations

- Use a meat thermometer to verify core temperature — guessing is not acceptable
- Keep Brät cold during preparation to prevent fat separation
- A Kastenform (loaf pan) of approx. 30 cm works for a standard batch
- If no Metzger-quality grinder is available, ask your local Metzger to grind the meat twice on the finest setting

## Success Metrics

- Crust is crispy and golden — passes the visual and crunch test
- Interior is smooth, juicy, and uniformly pink — no gray spots or air pockets
- Semmel-to-Leberkäs ratio feels balanced in every bite
- At least one person says "Des is a guade Leberkassemme!"

## Open Questions

- Should we offer a Spiegelei (fried egg) on top as an optional upgrade?
- Is homemade sweet mustard worth the effort, or is Händlmaier's sufficient?
```

---

## Checklist

Before saving the PRD:

- [ ] Asked clarifying questions with lettered options
- [ ] Incorporated user's answers
- [ ] User stories are small and specific
- [ ] Functional requirements are numbered and unambiguous
- [ ] Non-goals section defines clear boundaries
- [ ] Saved to `tasks/prd-[feature-name].md`