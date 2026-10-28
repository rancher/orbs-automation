# [The Red Green Commit Workflow](https://gist.github.com/mallardduck/5943ad397b3a58ed800093d53df9e7fb)
## Honest Duct-Tape Driven Design (Pin → Stub → Fail → Fix)

> "If the compiler doesn't find you finished, it should at least find you compiling." — [Red Green](https://www.youtube.com/watch?v=pY7nx5Z6Kzo)

The idea: a failing test should fail because the behavior is wrong, not because the code cannot compile.

## Elevator pitch

Red-green only works if the red is honest.

A test that fails because a type, package, or function literally doesn't exist isn't testing logic.
It's just complaining about syntax.
That's not Red; that's just a broken build.

**Honest Red** stub-wires the API shape first so everything compiles and runs, returning deliberately wrong or zeroed values.
Every failing test then proves your test harness actually works, points at the exact behavior still missing, and leaves a clean, bisectable Git history along the way.

## ELI5: The Cardboard Robot Principle

Imagine giving a written quiz to a robot that hasn't been built yet.

If you drop the test paper on an empty workbench, you get nothing back—not even a score.
That doesn't tell you if the quiz questions are any good or what the robot will actually get wrong.

So first, you prop up a cardboard robot with a piece of duct tape that answers "0" or "false" to every single question.
Now when you grade the quiz, you get honest wrong answers:
> “Question 3: Expected 42, got 0.”

Now you know the quiz is working.
Step by step, you replace the cardboard parts with real gears until every answer turns green and the tape comes off.


## Full Details

The problem with plain red-green.
When the red step is "write tests against APIs that don't exist," the failure is a compile error.
That has three costs:
- **No behavioral information:** The failure carries no information about behavior. Any broken setup, missing file, or empty implementation produces the exact same build error.
- **Broken history:** The commit can't build, so `git bisect` and CI pipelines both choke on it.
- **Unverified harness:** You can't tell whether the tests exercise anything real until after you've written the implementation.

### The cycle

```
1. Pin ──► 2. Stub ──► 3. Fail ──► 4. Fix
```

1. **Pin**. Write tests against the APIs that already exist. If they fail on an assertion, fix that in a small green commit first. If they pass straight away, they're characterization tests: they lock in current behavior so you don't break the lodge while adding new rooms.
2. **Stub**. Add the minimal skeleton of the new API so everything compiles. It should be inert but plausible: return zero values, false, empty strings, or the input unchanged. Don't panic or throw `NotImplementedException` (or error matching language best practice), because a panic kills the whole test binary and hides the per-case results. Commit this on its own, so reviewers see the shape of the API before any logic exists.
3. **Fail**. Write the tests for the new behavior. They now execute cleanly and fail with _got X, want Y_. This is the honest red. A test that fails against a stub for the right reason is a test you can trust.
4. **Fix**. Replace the stubs with real implementations until everything is green and the duct tape comes off.

### What you get
- **Bisectable history**: Every commit builds, so history is clean and git bisect never hits a dead end.
- **Readable failures**: Every red is a result you can read, compare, and verify.
- **Cleaner reviews**: API design and logic are reviewed separately.
- **Tautology catching**: Tests that accidentally pass against a stub get caught before you write the real code.

### When to skip it.
For a tiny change with no new public surface area, classic Red-Green is fine. Honest Red pays off when you add a new API, a new type, or several behaviors at once.