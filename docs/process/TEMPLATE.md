<!--
HOW TO USE THIS TEMPLATE

Copy this file to docs/process/<project>.md and fill it in. Delete every
comment block like this one when you are done.

Three kinds of text:

  KEEP      Headings, subheadings, bold bullet labels and the table header are
            fixed. Keep them as written, in this order, so project docs can be
            compared side by side. Do not rename, reorder or add sections; put
            anything that does not fit under "Pain and unknowns".
  REPLACE   Text in <angle brackets> is a placeholder. Replace it, brackets
            included. A heading like "From <project>" repeats for each hand-off.
  DELETE    Comment blocks like this one are instructions. Never keep them.

A section that does not apply keeps its heading and says "None."

STYLE

Raw facts and rules, simply written, simply read.

  * Lists, not paragraphs. One fact per bullet.
  * Fragments are fine. Keep a bullet to one line where you can.
  * Facts say what is. Rules say what must be, then why, after a colon.
  * No history, no narrative, no hedging. If you are not sure, say what you
    know and mark the rest (unconfirmed).

LEVEL OF DETAIL

This documents a release process, not a project's change history.

  * Every release starts the same way: changes are proposed, reviewed, then
    merged to the line's branch. Say that and move on.
  * Do not describe the changes or where they come from: no dependency or
    version bumps, schedules, bots, cron times, or backport mechanics.
  * Do not give version numbers except to show how versions are named.
  * Keep what changes how a release is cut, gated, published or handed on.
    Ask of each bullet: does it? If not, drop it.
  * A hand-off is where one project's release causes a change in another.
    Ordinary dependencies are not hand-offs; they arrive as changes.

WHERE THE FACTS COME FROM

  * Read whatever helps: workflows, scripts, configs, docs, tags, branches,
    pull requests, and the people involved.
  * Do not cite, name or link a script, workflow file, command or tool, and do
    not describe how the tooling is built. The doc stays true if every script
    is rewritten.
  * One exception: a config or lock file may be named, as a record, where it
    is the source of truth for a fact. Use it in "Recorded in" and "Where it
    stands", and say what fact it holds, not how it is edited or generated.
  * If the repo's docs and its scripts disagree, write what the scripts show,
    say the docs differ, and mark it (unconfirmed).

      Not this:  release.yml parses the tag with two regexes and publishes.
      This:      Pushing a tag publishes the image to every registry.
-->

# <project>

<!-- One sentence: what the project is in the release process, and who is responsible. -->

**Status:** <draft | reviewed>. **Written:** <YYYY-MM-DD>, <by whom>, <from what: a person's account, a read of the repo, or both>.

## What a release is

### Artifact

<!-- What is produced and where it is published. -->

* **What:** <an image, a chart, a tag, a binary>
* **Published to:** <where>

### Version

* **Scheme:** <how versions are named>
* **Tells you:** <what a version says: a Rancher line, a date, a series>

### Stable and prerelease

* **Prerelease:** <how it is named; where it goes>
* **Stable:** <how it is named; where it goes>
* **Downstream may take:** <stable only, or both>

### Stages

<!-- Tiers a release moves through, in order, or "None." -->

* <stage>

## Lines

* **Shape:** <one stream for every line (universal), one stream for each line, or lines only in version names>
* **Active lines:** <which, and how that is decided>
* **New line:** <what happens to the project>
* **Ended line:** <what happens to the project>

## How a release happens

<!-- One row for each step, in order. Row 1 is fixed: keep it, and do not expand it into kinds of changes. Say what happens, not which tool does it. A step a person does by hand is a step. Add rows as needed. -->

| # | Step | Starts when | Done by | Needs | Produces |
|---|---|---|---|---|---|
| 1 | Changes land | A change is proposed | Contributors; the team reviews | The proposed change | The change merged to the line's branch |
| 2 | <step> | <event or condition> | <person, team or automation> | <inputs> | <outputs> |

* **Gates:** <what must hold before a step>
* **Choices:** <where someone decides, and on what>
* **Failure:** <what can go wrong, and the fix>

## What it takes in

<!-- One block for each upstream hand-off; repeat the block. "None." if there are none. -->

### From <project>

* **What:** <the thing, and which of its releases>
* **How it arrives:** <pushed, pulled, or put in by a person; who>
* **What changes here:** <the edit, and what is regenerated>
* **Recorded in:** <where this project says what it holds>
* **Lines:** <all, or which>

## What it hands on

<!-- One block for each downstream hand-off; repeat the block. "None." if there are none. -->

### To <project>

* **What:** <the thing, and which of this project's releases>
* **Starts when:** <the event; whether it needs approval>
* **Done by:** <who>
* **Lines:** <all, or which; which downstream stream each goes to>

## Differences between lines

<!-- One bullet for each, as a line or range and what differs. Write "from v2.16" and "until v2.15". "None." if every line works alike. -->

* <line or range>: <what differs>

## Who is responsible

* **Owns:** <team>
* **Acts:** <who releases, who promotes between stages, who approves>
* **Can step in:** <who, and for what>

## Rules

<!-- "None." if there are none. -->

* <rule>: <reason>

## Where it stands

<!-- What the process leaves behind (tags, branches, pull requests, published artifacts, recorded versions), and what each tells you. -->

* **Latest release of a line:** <how to tell>
* **What a downstream holds:** <how to tell>
* **Not visible from outside:** <what is only in someone's head or a private system>

## Pain and unknowns

<!-- Anything that has no other home goes here too. -->

* **Hard today:** <what>
* **Not known:** <what>

## Config notes

<!-- A parking lot, not a design. Facts and questions a config for this project will need to cover. One line each, no field names. -->

* <fact or question>

## Evidence

<!-- Existing project note in ../projects/, other docs in this repo, people consulted. No scripts, workflow files or tools. -->

* <link or name>
