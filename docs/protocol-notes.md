# MCP protocol notes

Design notes for how this one-user, stdio Clockify MCP implements optional parts of
the MCP protocol.

## `resources/list` is intentionally single-page

`resources/list` and `resources/templates/list` return their full set in one
response with no `nextCursor`. This is intentional: the static resource set is fixed
and small, and the dynamic per-run demo resources are hard-capped at 50 entries
(`maxDemoResourcesListed`). The total listing therefore has a small, bounded upper
size, so cursor pagination would add protocol surface with no benefit.

## Progress notifications

Long-running tools emit `notifications/progress` when the client supplies a
`progressToken`. The server enforces the MCP rules: progress values must strictly
increase per token, and emissions are capped at 10 per second per token. Progress
stops once the originating call is cancelled or times out.
