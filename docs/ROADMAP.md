# VideoTools Roadmap

This roadmap is intentionally lightweight. It captures the next few
high-priority goals without locking the project into a rigid plan.

## How We Use This

- The roadmap is a short list, not a full backlog.
- Items can move between buckets as priorities change.
- We update this at the start of each dev cycle.

## Current State

- dev20 focused on cleanup and the Authoring module.
- Authoring is now functional (DVD folders + ISO pipeline).

## Now (dev21 focus)

- Finalize Convert module cleanup and preset behavior.
- Validate preset defaults and edge cases (aspect, bitrate, CRF).
- Tighten UI copy and error messaging for Convert/Queue.
- Add smoke tests for authoring and DVD encode workflows.

## Next

- Color space preservation across Convert/Upscale.
- Merge module completion (reorder, mixed format handling).
- Filters module polish (controls + real-time preview stability).

## Later

- Trim module UX and timeline tooling.
- AI frame interpolation support (model management + UI).
- Packaging polish for v0.1.1 (AppImage + Windows EXE).

## Versioning Note

We keep continuous dev numbering. After v0.1.1 release, the next dev
tag becomes v0.1.1-dev26 (or whatever the next number is).
