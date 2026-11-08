# Design Constraints

A set of design constraints we must consider when designing and building.
Optionally a constraint may have a few points of contrast to consider.

## Constraint: no release calendar state

* The tooling does not model the internal release calendar: code freeze, RC windows, release day
* Product lifecycle dates (GA, EOM, EOL) are data, and a line's phase may be derived from them and the clock
* No release answer depends on either: readiness, line consistency and what a branch holds come from what exists now (tags, branches, recorded versions)
* Release answers are the same on any date; anything that reads a lifecycle date takes the date as an input, so a test sets it
* A lifecycle date can raise a warning, never an error: an error would make a release exception harder to do

### Contrast: does not limit future scope

* The foundation we build on this constraint does not limit us in the future
* If an authoritative source exists we can use it for **additional** checks
* Should not degrade current features - only enhance with new function later

## Constraint: a stable after a stable is always possible

* The tooling may restrict a stable to follow an RC of its version
* It must still allow a new stable after a stable, for a small patch or a CVE fix
* So an RC-first check is a default that a release can override, never a wall
* An override is explicit and recorded; QA sign-off is outside the tooling for now (the [VISION](../VISION.md) v3 pillar may record it in the tracking issue, never in config)
* The process is in [RELEASE_PROCESS](../process/RELEASE_PROCESS.md#stable-after-stable)

## Constraint: unreleased lines

* The tooling can know a line before it is released, through config
* Several unreleased lines may exist at once
* A line cannot resolve into Rancher until Rancher has a branch for it
* `main` points at one future line only; a line with no branch yet is the one `main` serves
* The process is in [RELEASE_PROCESS](../process/RELEASE_PROCESS.md#the-shape)

## Constraint: a failed publish starts nothing downstream

* A publish ends by publishing the draft release, only after every step succeeds
* A hand-off starts from a published release, so a failed publish starts no hand-off
* The tooling does not need to clean up after a failed publish: the release stays in draft, and a re-tag starts again
* The process is in [RELEASE_PROCESS](../process/RELEASE_PROCESS.md#publishing-orbs-projects)
