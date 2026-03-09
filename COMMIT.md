TASK-006: Add startup banner and safety pause

Display a startup banner showing iteration count, branch, run ID, run
directory, and permissions mode before the work loop begins. Print a
warning about --dangerously-skip-permissions and wait 3 seconds so the
user can abort with Ctrl+C.
