# Underwater Video Tools

Tools for analyzing, correcting, and stitching underwater video with Go, FFmpeg, Make, and opencode.

## Tracked Files

- `underwater.go`: samples video frames and writes RGB CSV/PNG graph output.
- `Makefile`: graphing, RGB correction, AI correction, and stitching workflows.
- `editor.txt`: instructions for autonomous opencode correction runs.
- `raw/order.txt`: stitch order. Each raw filename maps to `corrected/<base>_corrected.<ext>`.

## Common Commands

Generate RGB data and graph without opening a viewer:

```bash
make rgbGraph raw/example.mp4
```

Generate RGB data and open the graph:

```bash
make rgbGraph raw/example.mp4 OPEN_GRAPH=1
```

Correct a video:

```bash
make rgbCorrect raw/example.mp4
```

Run autonomous AI correction:

```bash
make aiCorrect
```

Stitch corrected videos in `raw/order.txt` order with audio disabled:

```bash
make stitch
```

Keep audio while stitching:

```bash
make stitch STITCH_AUDIO=1
```

## Git Policy

The repository tracks tooling and configuration only. Raw videos, corrected videos, stitched outputs, graphs, previews, and temporary attempt files are intentionally ignored.
