# Rip output names — dev99

Each content-browser title exposes an Output title entry, defaulting to Title_01,
Title_02, etc. Files combine that name with the disc Title:
`Title_01_Disc_title.mkv`. Selected chapter groups add their existing suffix.
The Output path retains control of the destination directory and format extension;
title names control the per-file basename. Sanitize filename components and avoid
existing/queued output collisions using numeric suffixes.

Names are project state, preserved across Rip view rebuilds and cleared when a
different source/project loads. Recycled list cards must bind the current owner
before SetText and suppress programmatic edits, as with selection controls.

After a verified successful rip, clear the disc Title for that project. Failed or
cancelled jobs retain it. Capture a project/revision token in every queued job so
completion of older work cannot clear a newer project's title or recent edits.
Queued filenames/metadata are immutable snapshots and do not change after reset.
The reset also persists when returning to Rip; do not automatically refill the
cleared field from the old source. Full-disc/archive naming remains its existing
directory workflow.

Behavioral tests cover recycled edits, independent project state, successful
reset guards, filename composition/collision allocation and chapter naming.
Run the native build/vet/full test gate. Real-media multi-output acceptance and
cross-platform visual checks remain separate requirements.
