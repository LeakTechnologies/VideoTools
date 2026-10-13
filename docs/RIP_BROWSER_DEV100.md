# Rip content-browser correction — dev100

User feedback on dev99: a separate Output title label and stacked input make
cards unnecessarily tall, while preview thumbnails are absent on a loaded DVD.

Use two rows: title identity and editable filename on the top row, technical
summary below; selection stays at the right and a fixed preview slot at the left.
Details take the available width. Keep locked-title text readable; accent and
selection state still communicate the mode lock. Preserve stable recycled-card
selection/name ownership and programmatic update guards.

Thumbnail restoration must resolve the DVD source once, use a shared prepared
plaintext source for CSS media, and bound extraction work. Publish first frames
as they arrive; retirement prevents an older source/card updating newer content.
Fallback must identify the selected title's cells, not show the first movie for
every title. Capture extraction diagnostics instead of hiding the failure.

The rendered neutral fixture exposed a further confirmed bug: updating Image.Resource
without Image.Refresh left Fyne's cached pixels blank. Explicit refresh now
decodes arriving resources. A pixel-level regression protects this behavior.
The fixture was checked with VT's large VCR font and generated SMPTE thumbnails;
those test images are not encrypted-disc acceptance.

Source preparation is serialized, with two extraction workers and a first-still
pass across all titles. The existing CSS-copy API cannot interrupt its inner
loop; cancellation suppresses publication, then removes scratch data after copy
returns. Application shutdown during that copy remains an acceptance limitation.

Verification covers compact geometry at normal/large text and several widths,
preview-slot retention, recycled state and thumbnail argument/lifetime behavior.
Native build/vet/full tests remain required. Encrypted-disc thumbnail/content
acceptance and full platform/font coverage are not inferred from unit tests.

Validation for this slice: native build, vet and full tests passed; non-native
build passed. The actual FFmpeg command produced an 80×60 PNG from generated
DVD-shaped SMPTE media with non-square source pixels, confirming display-aspect
handling. Compact rendering was visually checked with the large VCR font.
