<h1 align="center">herdr-kibitzr</h1>
<p align="center"><em>Asks your coding agent to review the comments it just wrote,<br>so what explains something stays and the noise goes.</em></p>
<br>

<p align="center">
  <a href="https://github.com/wazum/herdr-kibitzr/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/wazum/herdr-kibitzr/ci.yml?branch=main&style=for-the-badge&logo=githubactions&logoColor=white&label=checks&labelColor=24273a" alt="checks"></a>
  <a href="https://herdr.dev"><img src="https://img.shields.io/badge/herdr-0.8.0%2B-c3b1e1?style=for-the-badge&logoColor=white&labelColor=24273a" alt="herdr 0.8.0 or newer"></a>
  <a href="go.mod"><img src="https://img.shields.io/badge/Go-1.26%2B-b5ead7?style=for-the-badge&logo=go&logoColor=white&labelColor=24273a" alt="Go 1.26 or newer"></a>
  <a href="go.mod"><img src="https://img.shields.io/badge/dependencies-0-ffb997?style=for-the-badge&labelColor=24273a" alt="no dependencies"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/licence-MIT-ffc6d9?style=for-the-badge&logo=opensourceinitiative&logoColor=white&labelColor=24273a" alt="MIT licence"></a>
</p>

A plugin for [herdr](https://herdr.dev), the terminal multiplexer that runs your
coding agents.

Every coding agent comments too much. You can put "no useless comments" in
`CLAUDE.md`, in `AGENTS.md`, in a skill, and it still writes `// increment the
counter` above `i++`. The instruction competes with everything else in the
prompt and loses.

kibitzr waits until the turn is over and hands the comments back:

```
› You added comments in your last changes, listed below by file. Review each
  one. Delete every comment that only restates what the code already says,
  including single-line ones. Shorten what remains. Keep type annotations and
  docblocks the tooling needs. Do not change code.

  src/Order.php
  src/OrderRepository.php

● Removed 7 of 9. Kept the two @var annotations.
```

An agent judges a comment in front of it better than it follows a rule about one
it hasn't written yet. You edit the wording once and then stay out of it.

## Install

```bash
herdr plugin install wazum/herdr-kibitzr
```

Nothing to bind, no pane to open. It starts on the next agent turn that ends.
Needs herdr 0.8.0 or newer on macOS or Linux, and git.

## How it decides

When an agent pane settles to `done` or `idle`, kibitzr reads what **that agent**
wrote since it last looked, and prompts it if any of it was a comment. Not what
changed in the repository. Two panes can share a repository, and one can be
editing a project rooted somewhere else, so "this tree has uncommitted comments"
says nothing about who wrote them.

Claude Code records the literal text of every edit, so for Claude this is exact.
Codex records only the shell it ran, so it falls back to files written while that
agent was working. That is weaker: a person typing in an editor during the same
stretch looks the same, and so does a second agent in the same repository.

A nudge hands the next turn to the agent unconditionally. That turn is the agent
acting on what it was told, and acting on it usually means rewriting a comment
rather than deleting it, which is freshly written text again. Judging that turn
would nudge about the cleanup.

Two panes are never touched: one you have focused, because a prompt would land
on whatever you were half way through typing, and one whose writes predate
kibitzr, so a session that has just opened is not blamed for the tree it found.

Why it fired, or didn't, is in `herdr plugin log list --plugin wazum.kibitzr`.

## What you see

A pane that was just asked about its comments wears them for ten seconds:

```
▾ kibitzr
  ● claude 👀
```

The eyes ride on the agent label, the one piece of text in herdr's default
sidebar rows a plugin can change, so this needs no configuration.

## Muting a pane

Every agent pane is watched from the moment you install it. To leave one alone,
bind the toggle and press it in that pane:

```toml
[[keys.command]]
key = "prefix+m"
type = "plugin_action"
command = "wazum.kibitzr.toggle"
description = "kibitzr: watch or mute this pane"
```

A muted pane wears `🔇` on its agent label until you unmute it. Muting is per
pane, so a second pane in the same project carries on being watched.

## Configure

One knob, the wording. `herdr plugin config-dir wazum.kibitzr` prints the
directory; edit `prompt.md` in there. kibitzr appends the list of files with
added comments underneath whatever you write, so keep the file to prose.

## What counts as a comment

A line whose first non-blank characters are `//`, `/*`, `<!--`, `#`, or an
asterisk continuing a block comment. Three lookalikes are excluded, because each
fires on ordinary code:

- a shebang, `#!/usr/bin/env bash`
- a preprocessor directive, `#include`, `#define` and that family
- a dereference, `*ptr = value`, which is why an asterisk needs a space, a
  slash, or nothing after it

Prose and data files are skipped by extension: `.md`, `.txt`, `.rst`, `.yml`,
`.yaml`, `.json`, `.toml`, `.lock`, `.csv`, `.svg`. Trailing comments after code
are not counted, because finding them without a tokenizer flags every URL and
every `#` inside a string.

The detector only decides whether to speak. Your `prompt.md` decides what to
keep, which is why the default text protects type annotations and docblocks.

## What it gets wrong

Only Claude gets exact attribution. For any other agent, a comment you typed
yourself while it was working is blamed on it, and so is one written by a second
agent in the same repository. Adding an agent is one adapter behind one
interface, so this improves as agents start recording their own edits.

Cost runs in a short-lived child process that cannot touch herdr's rendering,
terminal parsing or detection:

| event | measured |
| --- | --- |
| not a turn end: no herdr call, no git | 4 ms |
| a turn end, Claude, 400-entry session log | 19 ms |
| a turn end, Codex, 500-file repository | 48 ms |

Reads are bounded at 300 untracked files of 1 MB each, and every git and herdr
call runs under a timeout, so neither a scaffolded project nor a hung call can
hold a plugin slot.

## Licence

MIT
