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

kibitzr waits until the turn is over, looks at what landed in git, and hands the
comments back:

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

Nothing to bind, no pane to open. It starts working on the next agent turn that
ends. Needs herdr 0.8.0 or newer on macOS or Linux, and git.

## How it decides

Every time an agent pane settles to `done` or `idle`, kibitzr diffs that pane's
repository against the commit it recorded at the previous turn end. Measuring
from there rather than from `HEAD` means comments the agent committed mid-turn
still count. Comment lines in files git doesn't track yet count too.

It prompts the agent when that count is higher than at the last nudge, which is
what stops it repeating itself. Nudged about nine comments and the agent kept
two? The count went down, so it says nothing. Wrote four more later? The count
passed the mark, so it speaks again. A commit resets the mark, because what is
committed is part of the baseline now.

While a nudged pane sits at its turn end, its sidebar row wears the eyes:

```
▾ kibitzr
  ● claude  👀 done
```

Why it fired, or didn't, is in `herdr plugin log list --plugin wazum.kibitzr`.

## Configure

One knob, the wording. `herdr plugin config-dir wazum.kibitzr` prints the
directory; edit `prompt.md` in there. kibitzr appends the list of files with
added comments underneath whatever you write, so keep the file to prose.

## What counts as a comment

A line whose first non-blank characters are `//`, `/*`, `<!--`, `#`, or an
asterisk continuing a block comment. Three things that look like markers are
excluded, because each would otherwise fire on ordinary code:

- a shebang, `#!/usr/bin/env bash`
- a preprocessor directive, `#include` or `#define` and the rest of that family
- a dereference, `*ptr = value`, which is why an asterisk has to be followed by
  a space, a slash, or nothing

Prose and data files are skipped by extension: `.md`, `.txt`, `.rst`, `.yml`,
`.yaml`, `.json`, `.toml`, `.lock`, `.csv`, `.svg`.

Trailing comments after code are not counted. Finding them without a real
tokenizer flags every URL and every `#` inside a string, and the agent reviews
the whole file anyway once it knows which files to look at.

The detector only decides whether to speak. Your `prompt.md` decides what to
keep, which is why the default text protects type annotations and docblocks.

## What it gets wrong

A comment you typed yourself between two turn ends is blamed on the agent. The
window is small, since the hook only runs when an agent finishes and only looks
at what changed since it last finished, but it is real.

A prompt can also land while you are typing. A turn end in the pane you are
looking at reports `idle`, and kibitzr prompts it. If you had half a sentence in
the composer, the nudge is appended to it and submitted together. Detecting a
non-empty composer would mean parsing each agent's UI, so this is accepted
rather than solved.

Cost is low enough to ignore: a status change that isn't a turn end costs about
4 ms and runs no git at all, and a turn end on a 500-file repository costs about
70 ms in a child process, where it can't touch herdr's rendering or terminal
parsing. Untracked files are read up to 300 files and 1 MB each, so a project
scaffolded before anyone wrote a `.gitignore` can't turn into an unbounded read.

## Licence

MIT
