feat: add model alias resolution for short names (TASK-302)

Add ResolveModel function to config package that maps short aliases
(sonnet, opus, haiku) to full Claude model IDs. Unknown strings and
empty strings pass through unchanged.
