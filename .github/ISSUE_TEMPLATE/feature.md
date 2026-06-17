---
name: Feature request
about: Suggest a new feature for rein
title: ''
labels: enhancement
assignees: ''
---

## Use case

Describe the problem you're trying to solve. What are you doing
today, and what would you rather be doing?

For example: "I'm building an agent that needs to drive `vim`
interactively. Today I have to wrap `creack/pty` myself. I wish
rein had a `Session.SendKeys()` method that took a Vim key
sequence and sent it to the PTY."

## Proposed solution

Describe the API or behavior you'd like to see. A small code
example is the best way to make this concrete.

```go
session.SendKeys("iHello, world!<Esc>:wq<CR>")
```

## Alternatives considered

What other approaches have you considered? Why is your proposed
solution better?

## Additional context

Anything else that might help, e.g. links to similar features in
other tools, or the AI agent issue that prompted this request.
