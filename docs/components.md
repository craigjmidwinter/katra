---
title: Components
layout: default
nav_order: 3
description: >-
  Every rich component katra renders — compare sliders, galleries, video,
  embeds and callouts — with the exact keys each accepts, plus the recipe for
  charts and diagrams.
---

# Components

A component is a fenced code block whose language names it. That is the whole
mechanism, and it is why an entry stays a readable, diffable markdown file: the
source shows the data, the page shows the widget.

````markdown
```compare
before: media/bunker_before.png
after:  media/bunker_after.png
caption: Bunker reshape
```
````

Six are built in. Every one takes YAML in the fence body.

{: .note }
**An unregistered language renders as an ordinary code block.** That is
deliberate and is the compatibility rule for the format: an entry written
against a newer katra still renders in an older one, just less prettily. A
fence is never an error.

## compare

A before/after slider with a draggable handle.

````markdown
```compare
before: media/before.png
after:  media/after.png
caption: Arc, before and after
```
````

| Key | Required | Notes |
| --- | --- | --- |
| `before` | yes | Path to the "before" image, relative to the katra directory. |
| `after` | yes | Path to the "after" image. |
| `caption` | no | Rendered as a `<figcaption>`. |

Omitting either image is an error, because a one-sided comparison is a mistake
rather than a degraded case.

The usual way to produce one is not to hand-write the block:

```bash
katra compare before.png after.png --caption "Bunker reshape"
```

That imports both files into `media/` and appends the fence. Pass
`--no-import` if the paths already point inside `media/`.

## gallery

A row of images, each with its own caption, each opening in a lightbox.

````markdown
```gallery
- src: media/tier-one.png
  cap: tier one
- src: media/tier-two.png
  cap: tier two
- src: media/tier-three.png
  cap: tier three
```
````

The body is a YAML **list**, not a mapping. Each item takes:

| Key | Required | Notes |
| --- | --- | --- |
| `src` | yes | Items without one are skipped rather than failing the block. |
| `cap` | no | Doubles as the image's `alt` text. |

An empty list is an error. The item count is emitted as `data-count`, which the
stylesheet uses to lay the row out, so a gallery of two and a gallery of six
are both deliberate.

## video

````markdown
```video
src: media/horde.mp4
poster: media/horde-poster.png
caption: The horde, dispatched
loop: true
```
````

| Key | Required | Notes |
| --- | --- | --- |
| `src` | yes | Any format the browser plays. `.mp4` (h264) is the safe choice. |
| `poster` | no | Still shown before playback. |
| `caption` | no | |
| `loop` | no | `true` also sets `muted`, because browsers refuse to autoplay or loop audio unprompted. |

Controls are always shown, and playback is `playsinline` so a phone does not
take the video fullscreen.

For a short screen recording, a `.gif` captured with `katra capture` is often
better than a video — it needs no controls and starts instantly.

## embed

An iframe over a self-contained HTML file. This is the escape hatch, and the
most useful component in the set.

````markdown
```embed
src: media/frame-times.html
height: 480
caption: Frame time vs district count, Quest 3 vs Editor
```
````

| Key | Required | Notes |
| --- | --- | --- |
| `src` | yes | Usually an `.html` file in `media/`. |
| `height` | no | Pixels. Defaults to `480`. |
| `caption` | no | |

The iframe is sandboxed with `allow-scripts allow-same-origin
allow-pointer-lock`, so inline JavaScript runs and a WebGL canvas can capture
the pointer.

### Charts and diagrams

There is no chart component. You author the figure as HTML and embed it, which
means any chart you can draw is available rather than a fixed set of types:

```bash
katra capture /tmp/frame-times.html --caption "p95 latency, before vs after"
```

`katra capture` recognises `.html` and writes an `embed` block automatically.

Four rules for the HTML, because of where it renders:

- **No external requests.** No CDN scripts, no web fonts, no remote images.
  Inline everything and embed images as `data:` URIs. A static `katra build`
  has to work offline; a figure that phones home breaks that promise.
- **Size it for the column.** Set an explicit `viewBox`, let the SVG scale to
  100% width, and pass `height:` if 480px is wrong.
- **Give the figure its own background.** `prefers-color-scheme` follows the
  *OS*, not the page, and the iframe cannot see its host. A transparent figure
  will eventually render dark ink on a light page. Set an explicit `background`
  in both branches of your media query so ink and surface always agree.
- **Label the axes and the units.** An unlabelled chart is decoration.

A bar chart is about twenty lines of inline SVG — one `<rect>` per bar, one
`<text>` per label, one `<line>` for the baseline. Do not reach for a library.

## note and warning

Callouts. The body is **markdown**, not YAML — it is rendered, so links,
emphasis and lists all work inside one.

````markdown
```note
The seeded layout still replays: RNG draw order is untouched.
```

```warning
Needs an in-headset capture to confirm. Not verified on device.
```
````

Neither takes any keys. `warning` is the conventional way to close an entry
that has a part still unconfirmed — something needing a device, a person, or a
rerun.

## Images

A plain markdown image gets a lightbox for free:

```markdown
![the bunker after the reshape](media/bunker.png)
```

Use `katra capture` to import one rather than copying it by hand — it puts the
file in `media/`, gives it a collision-free name, and appends the block to the
active draft:

```bash
katra capture ~/Desktop/shot.png --caption "after the fix"
katra capture render.gif --caption "the horde, dispatched"
```

`--no-append` imports the file without touching any entry, and `--as` forces
the kind when the extension is misleading.

## Adding a component

One `ComponentFunc` in `internal/core/render.go` — that is the entire extension
surface:

```go
var Registry = map[string]ComponentFunc{
	"embed":   renderEmbed,
	"compare": renderCompare,
	// ...
}
```

A `ComponentFunc` takes the raw fence body and returns an HTML fragment. See
[Contributing](https://github.com/craigjmidwinter/katra/blob/main/CONTRIBUTING.md#adding-a-rich-component)
for the two constraints on what one may do, and the two other files a new
component has to touch.
