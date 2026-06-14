FPS ?= 1
GRAPH ?= color_graph.png
CSV ?= color_data.csv
OPEN_GRAPH ?= 0
STITCH_DIR ?= corrected
STITCH_PATTERN ?= *.mp4
STITCH_ORDER ?= raw/order.txt
STITCH_SUFFIX ?= _corrected
STITCH_AUDIO ?= 0
OUTPUT ?= stitched.mp4
CONCAT_LIST ?= .concat_list.txt
ORIGINAL ?=
CORRECTED ?=
PREVIEW_FPS ?= 1
PREVIEW_SCALE ?= 480:-1
PREVIEW_TILE ?= 3x3
MODEL ?=
AUTO_APPROVE ?= 1
RED_GAIN ?= 1.30
GREEN_GAIN ?= 1.05
BLUE_GAIN ?= 0.90
CONTRAST ?= 1.08
SATURATION ?= 1.15
STABILIZE ?= 0
SHAKINESS ?= 5
ACCURACY ?= 15
SMOOTHING ?= 30
ZOOM ?= 5
EXTRA_GOALS := $(filter-out help rgbGraph rgbSpread compareRgb rgbCorrect previewSheet aiCorrect stitch clean cleanCorrected,$(MAKECMDGOALS))

.PHONY: help rgbGraph rgbSpread compareRgb rgbCorrect previewSheet aiCorrect stitch clean cleanCorrected
.PHONY: $(EXTRA_GOALS)

help:
	@echo "Usage:"
	@echo "  make rgbGraph <video-file>"
	@echo "  make rgbSpread CSV=<csv-file>"
	@echo "  make compareRgb ORIGINAL=<csv-file> CORRECTED=<csv-file>"
	@echo "  make rgbCorrect <video-file>"
	@echo "  make previewSheet <video-file> OUTPUT=<jpg-file>"
	@echo "  make aiCorrect"
	@echo "  make stitch"
	@echo ""
	@echo "Options:"
	@echo "  MODEL=                 Optional opencode model, for example openai/gpt-5.5"
	@echo "  AUTO_APPROVE=1         Auto-approve opencode permissions for aiCorrect"
	@echo "  FPS=1                  Frames sampled per second"
	@echo "  GRAPH=color_graph.png  Output PNG graph"
	@echo "  CSV=color_data.csv     Output CSV data"
	@echo "  OPEN_GRAPH=0           Set to 1 to open graph PNG with eog after rgbGraph"
	@echo "  STITCH_DIR=corrected   Directory containing videos to stitch"
	@echo "  STITCH_PATTERN=*.mp4   Filename pattern to stitch alphabetically"
	@echo "  STITCH_ORDER=raw/order.txt  Optional order file with raw video names"
	@echo "  STITCH_SUFFIX=_corrected    Suffix for corrected videos"
	@echo "  STITCH_AUDIO=0         Set to 1 to keep audio in stitched output"
	@echo "  OUTPUT=stitched.mp4    Output stitched video path"
	@echo "  ORIGINAL=              Original CSV path for compareRgb"
	@echo "  CORRECTED=             Corrected CSV path for compareRgb"
	@echo "  PREVIEW_FPS=1          Frames per second sampled by previewSheet"
	@echo "  PREVIEW_SCALE=480:-1   Frame scale used by previewSheet"
	@echo "  PREVIEW_TILE=3x3       Tile layout used by previewSheet"
	@echo "  RED_GAIN=1.30          Red channel multiplier"
	@echo "  GREEN_GAIN=1.05        Green channel multiplier"
	@echo "  BLUE_GAIN=0.90         Blue channel multiplier"
	@echo "  CONTRAST=1.08          FFmpeg eq contrast multiplier"
	@echo "  SATURATION=1.15        FFmpeg eq saturation multiplier"
	@echo "  STABILIZE=0            Set to 1 to stabilize before correction"
	@echo "  SHAKINESS=5            Stabilization shakiness, 1-10"
	@echo "  ACCURACY=15            Stabilization accuracy, 1-15"
	@echo "  SMOOTHING=30           Stabilization smoothing amount"
	@echo "  ZOOM=5                 Stabilization zoom to hide borders"
	@echo ""
	@echo "Examples:"
	@echo "  make rgbGraph giantSeaBass.mp4"
	@echo "  make rgbGraph giantSeaBass.mp4 FPS=2"
	@echo "  make rgbGraph giantSeaBass.mp4 OPEN_GRAPH=1"
	@echo "  make rgbGraph giantSeaBass1.mp4 GRAPH=fish_graph.png CSV=fish_data.csv"
	@echo "  make rgbSpread CSV=graphs/giantSeaBass.csv"
	@echo "  make compareRgb ORIGINAL=graphs/original.csv CORRECTED=graphs/corrected.csv"
	@echo "  make rgbCorrect giantSeaBass.mp4"
	@echo "  make rgbCorrect raw/seabass.MP4 OUTPUT=attempts/seabass_attempt2.MP4"
	@echo "  make rgbCorrect giantSeaBass.mp4 RED_GAIN=1.50 BLUE_GAIN=0.85 SATURATION=1.25"
	@echo "  make rgbCorrect giantSeaBass.mp4 STABILIZE=1"
	@echo "  make rgbCorrect giantSeaBass.mp4 STABILIZE=1 SMOOTHING=45 ZOOM=8"
	@echo "  make previewSheet attempts/seabass_attempt2.MP4 OUTPUT=previews/seabass_attempt2.jpg"
	@echo "  make aiCorrect"
	@echo "  make aiCorrect MODEL=openai/gpt-5.5"
	@echo "  make aiCorrect AUTO_APPROVE=0"
	@echo "  make stitch"
	@echo "  make stitch OUTPUT=final_dive.mp4"
	@echo "  make stitch STITCH_ORDER=raw/order.txt"
	@echo "  make stitch STITCH_AUDIO=1"
	@echo "  make clean"
	@echo "  make cleanCorrected"

rgbGraph:
	@if [ -z "$(word 2,$(MAKECMDGOALS))" ]; then \
		echo "usage: make rgbGraph <video-file>"; \
		 exit 1; \
	fi
	go run underwater.go -fps $(FPS) -graph "$(GRAPH)" -csv "$(CSV)" "$(word 2,$(MAKECMDGOALS))"
	@if [ "$(OPEN_GRAPH)" = "1" ]; then \
		eog "$(GRAPH)"; \
	fi

rgbSpread:
	@if [ -z "$(CSV)" ]; then \
		echo "usage: make rgbSpread CSV=<csv-file>"; \
		exit 1; \
	fi
	go run underwater.go -stats "$(CSV)"

compareRgb:
	@if [ -z "$(ORIGINAL)" ] || [ -z "$(CORRECTED)" ]; then \
		echo "usage: make compareRgb ORIGINAL=<csv-file> CORRECTED=<csv-file>"; \
		exit 1; \
	fi
	go run underwater.go -compare "$(ORIGINAL)" "$(CORRECTED)"

rgbCorrect:
	@if [ -z "$(word 2,$(MAKECMDGOALS))" ]; then \
		echo "usage: make rgbCorrect <video-file>"; \
		exit 1; \
	fi
	@set -e; \
	input="$(word 2,$(MAKECMDGOALS))"; \
		dir=$$(dirname "$$input"); \
		base=$$(basename "$$input"); \
		name="$${base%.*}"; \
		ext="$${base##*.}"; \
		if [ -n "$(OUTPUT)" ] && [ "$(OUTPUT)" != "stitched.mp4" ]; then \
			output="$(OUTPUT)"; \
		else \
			output="$$dir/$${name}_corrected.$$ext"; \
		fi; \
		trf="$$dir/$${name}_transforms.trf"; \
		filter="colorchannelmixer=rr=$(RED_GAIN):gg=$(GREEN_GAIN):bb=$(BLUE_GAIN),eq=contrast=$(CONTRAST):saturation=$(SATURATION)"; \
		mkdir -p "$$(dirname "$$output")"; \
		if [ "$(STABILIZE)" = "1" ]; then \
		echo "analyzing stabilization into $$trf"; \
		ffmpeg -y -i "$$input" -vf "vidstabdetect=shakiness=$(SHAKINESS):accuracy=$(ACCURACY):result=$$trf" -f null -; \
		filter="format=yuv420p,vidstabtransform=input=$$trf:smoothing=$(SMOOTHING):zoom=$(ZOOM),format=rgb24,$$filter"; \
	fi; \
	echo "writing $$output"; \
	ffmpeg -y -i "$$input" -vf "$$filter" -map 0:v:0 -map 0:a? -dn -map_metadata -1 -c:v libx264 -pix_fmt yuv420p -c:a copy -write_tmcd 0 -movflags +faststart "$$output"; \
	if [ "$(STABILIZE)" = "1" ]; then \
			rm -f "$$trf"; \
		fi

previewSheet:
	@if [ -z "$(word 2,$(MAKECMDGOALS))" ]; then \
		echo "usage: make previewSheet <video-file> OUTPUT=<jpg-file>"; \
		exit 1; \
	fi
	@if [ -z "$(OUTPUT)" ] || [ "$(OUTPUT)" = "stitched.mp4" ]; then \
		echo "usage: make previewSheet <video-file> OUTPUT=<jpg-file>"; \
		exit 1; \
	fi
	@mkdir -p "$$(dirname "$(OUTPUT)")"
	ffmpeg -y -i "$(word 2,$(MAKECMDGOALS))" -vf "fps=$(PREVIEW_FPS),scale=$(PREVIEW_SCALE),tile=$(PREVIEW_TILE)" -frames:v 1 -update 1 -q:v 2 "$(OUTPUT)"

aiCorrect:
	@if [ ! -f editor.txt ]; then \
		echo "editor.txt not found"; \
		exit 1; \
	fi
	@set -e; \
	args=""; \
	if [ -n "$(MODEL)" ]; then \
		args="$$args --model $(MODEL)"; \
	fi; \
	if [ "$(AUTO_APPROVE)" = "1" ]; then \
		args="$$args --dangerously-skip-permissions"; \
	fi; \
	opencode run "Follow the attached editor.txt instructions for underwater video correction in this project." --dir "$(CURDIR)" $$args --file=editor.txt

stitch:
	@set -e; \
	if [ ! -d "$(STITCH_DIR)" ]; then \
		echo "stitch directory not found: $(STITCH_DIR)"; \
		exit 1; \
	fi; \
	rm -f "$(CONCAT_LIST)"; \
	count=0; \
	if [ -f "$(STITCH_ORDER)" ]; then \
		echo "using stitch order from $(STITCH_ORDER)"; \
		while IFS= read -r raw_file || [ -n "$$raw_file" ]; do \
			raw_file=$$(printf '%s' "$$raw_file" | sed 's/^[[:space:]]*//; s/[[:space:]]*$$//'); \
			case "$$raw_file" in ''|'#'*) continue ;; esac; \
			base=$$(basename "$$raw_file"); \
			name="$${base%.*}"; \
			ext="$${base##*.}"; \
			file="$(STITCH_DIR)/$${name}$(STITCH_SUFFIX).$$ext"; \
			if [ ! -f "$$file" ]; then \
				echo "missing corrected video for order entry '$$raw_file': $$file"; \
				exit 1; \
			fi; \
			printf "file '%s'\n" "$$file" >> "$(CONCAT_LIST)"; \
			count=$$((count + 1)); \
		done < "$(STITCH_ORDER)"; \
	else \
		echo "no stitch order found at $(STITCH_ORDER); using alphabetical order"; \
		for file in $$(find "$(STITCH_DIR)" -maxdepth 1 -type f -name "$(STITCH_PATTERN)" | sort); do \
			printf "file '%s'\n" "$$file" >> "$(CONCAT_LIST)"; \
			count=$$((count + 1)); \
		done; \
	fi; \
	if [ "$$count" -eq 0 ]; then \
		echo "no videos found in $(STITCH_DIR) matching $(STITCH_PATTERN)"; \
		exit 1; \
	fi; \
	echo "stitching $$count videos into $(OUTPUT)"; \
	if [ "$(STITCH_AUDIO)" = "1" ]; then \
		ffmpeg -y -f concat -safe 0 -i "$(CONCAT_LIST)" -map 0:v:0 -map 0:a? -dn -c:v libx264 -pix_fmt yuv420p -c:a aac -movflags +faststart "$(OUTPUT)"; \
	else \
		ffmpeg -y -f concat -safe 0 -i "$(CONCAT_LIST)" -map 0:v:0 -an -dn -c:v libx264 -pix_fmt yuv420p -movflags +faststart "$(OUTPUT)"; \
	fi; \
	rm -f "$(CONCAT_LIST)"

clean:
	rm -f "$(GRAPH)" "$(CSV)" "$(CONCAT_LIST)" underwater *.trf raw/*.trf

cleanCorrected:
	rm -f *_corrected.mp4 *_corrected.mov *_corrected.mkv raw/*_corrected.mp4 raw/*_corrected.mov raw/*_corrected.mkv

$(EXTRA_GOALS):
	@:
