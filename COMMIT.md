TASK-005: Wire everything together in the work loop

Add iteration banner (`========== Iteration <i> of <N> ==========`) before
each iteration and print remaining incomplete tasks (title only, max 5) after
the loop. The full ralph-style flow is now wired end-to-end: detect branch →
create run dir → loop (find task → build prompt → invoke claude → git commit)
→ print summary.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
