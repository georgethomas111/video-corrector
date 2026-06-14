package main

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type videoInfo struct {
	Width  int
	Height int
}

type sample struct {
	Time  float64
	Red   float64
	Green float64
	Blue  float64
}

type rgbStats struct {
	Samples int
	Red     float64
	Green   float64
	Blue    float64
	Spread  float64
}

func main() {
	fps := flag.Float64("fps", 1, "frames to sample per second")
	graphPath := flag.String("graph", "color_graph.png", "output PNG graph path")
	csvPath := flag.String("csv", "color_data.csv", "output CSV data path")
	statsPath := flag.String("stats", "", "read a CSV and print average RGB and spread")
	compareMode := flag.Bool("compare", false, "compare two CSV files and print average RGB and spread deltas")
	flag.Parse()

	if *statsPath != "" {
		if flag.NArg() != 0 {
			fmt.Fprintf(os.Stderr, "usage: go run underwater.go -stats color_data.csv\n")
			os.Exit(2)
		}
		stats, err := statsFromCSV(*statsPath)
		if err != nil {
			fatal(err)
		}
		printStats(filepath.Base(*statsPath), stats)
		return
	}

	if *compareMode {
		if flag.NArg() != 2 {
			fmt.Fprintf(os.Stderr, "usage: go run underwater.go -compare original.csv corrected.csv\n")
			os.Exit(2)
		}
		original, err := statsFromCSV(flag.Arg(0))
		if err != nil {
			fatal(fmt.Errorf("original CSV: %w", err))
		}
		corrected, err := statsFromCSV(flag.Arg(1))
		if err != nil {
			fatal(fmt.Errorf("corrected CSV: %w", err))
		}
		printComparison(filepath.Base(flag.Arg(0)), original, filepath.Base(flag.Arg(1)), corrected)
		return
	}

	if flag.NArg() != 1 {
		fmt.Fprintf(os.Stderr, "usage: go run underwater.go [-fps 1] [-graph color_graph.png] [-csv color_data.csv] input.mp4\n")
		fmt.Fprintf(os.Stderr, "       go run underwater.go -stats color_data.csv\n")
		fmt.Fprintf(os.Stderr, "       go run underwater.go -compare original.csv corrected.csv\n")
		os.Exit(2)
	}
	if *fps <= 0 {
		fmt.Fprintln(os.Stderr, "-fps must be greater than 0")
		os.Exit(2)
	}

	input := flag.Arg(0)
	info, err := probeVideo(input)
	if err != nil {
		fatal(err)
	}

	samples, err := analyzeVideo(input, info, *fps)
	if err != nil {
		fatal(err)
	}
	if len(samples) == 0 {
		fatal(errors.New("no frames were sampled from the video"))
	}

	if err := writeCSV(*csvPath, samples); err != nil {
		fatal(err)
	}
	if err := drawGraph(*graphPath, samples); err != nil {
		fatal(err)
	}

	fmt.Printf("analyzed %d samples from %s\n", len(samples), filepath.Base(input))
	fmt.Printf("wrote %s\n", *csvPath)
	fmt.Printf("wrote %s\n", *graphPath)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}

func probeVideo(input string) (videoInfo, error) {
	cmd := exec.Command(
		"ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height",
		"-of", "json",
		input,
	)
	out, err := cmd.Output()
	if err != nil {
		return videoInfo{}, fmt.Errorf("ffprobe failed: %w", err)
	}

	var result struct {
		Streams []struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return videoInfo{}, fmt.Errorf("parse ffprobe output: %w", err)
	}
	if len(result.Streams) == 0 || result.Streams[0].Width <= 0 || result.Streams[0].Height <= 0 {
		return videoInfo{}, errors.New("could not determine video dimensions")
	}

	return videoInfo{Width: result.Streams[0].Width, Height: result.Streams[0].Height}, nil
}

func analyzeVideo(input string, info videoInfo, fps float64) ([]sample, error) {
	cmd := exec.Command(
		"ffmpeg",
		"-v", "error",
		"-i", input,
		"-vf", fmt.Sprintf("fps=%s", formatFloat(fps)),
		"-f", "rawvideo",
		"-pix_fmt", "rgb24",
		"-",
	)

	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start ffmpeg: %w", err)
	}

	frameSize := info.Width * info.Height * 3
	frame := make([]byte, frameSize)
	reader := bufio.NewReaderSize(stdout, frameSize)
	samples := make([]sample, 0)
	frameIndex := 0

	for {
		_, err := io.ReadFull(reader, frame)
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read frame: %w", err)
		}

		samples = append(samples, averageFrame(frame, frameIndex, fps))
		frameIndex++
	}

	if err := cmd.Wait(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message != "" {
			return nil, fmt.Errorf("ffmpeg failed: %w: %s", err, message)
		}
		return nil, fmt.Errorf("ffmpeg failed: %w", err)
	}

	return samples, nil
}

func averageFrame(frame []byte, frameIndex int, fps float64) sample {
	var red, green, blue uint64
	for i := 0; i+2 < len(frame); i += 3 {
		red += uint64(frame[i])
		green += uint64(frame[i+1])
		blue += uint64(frame[i+2])
	}

	pixels := float64(len(frame) / 3)
	return sample{
		Time:  float64(frameIndex) / fps,
		Red:   float64(red) / pixels,
		Green: float64(green) / pixels,
		Blue:  float64(blue) / pixels,
	}
}

func writeCSV(path string, samples []sample) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	if err := writer.Write([]string{"time_seconds", "red", "green", "blue"}); err != nil {
		return err
	}
	for _, s := range samples {
		if err := writer.Write([]string{
			formatFloat(s.Time),
			formatFloat(s.Red),
			formatFloat(s.Green),
			formatFloat(s.Blue),
		}); err != nil {
			return err
		}
	}

	return writer.Error()
}

func statsFromCSV(path string) (rgbStats, error) {
	file, err := os.Open(path)
	if err != nil {
		return rgbStats{}, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return rgbStats{}, err
	}
	if len(records) < 2 {
		return rgbStats{}, errors.New("CSV has no sample rows")
	}

	var stats rgbStats
	for i, record := range records[1:] {
		if len(record) != 4 {
			return rgbStats{}, fmt.Errorf("row %d: expected 4 columns, got %d", i+2, len(record))
		}
		red, err := strconv.ParseFloat(record[1], 64)
		if err != nil {
			return rgbStats{}, fmt.Errorf("row %d red: %w", i+2, err)
		}
		green, err := strconv.ParseFloat(record[2], 64)
		if err != nil {
			return rgbStats{}, fmt.Errorf("row %d green: %w", i+2, err)
		}
		blue, err := strconv.ParseFloat(record[3], 64)
		if err != nil {
			return rgbStats{}, fmt.Errorf("row %d blue: %w", i+2, err)
		}

		stats.Samples++
		stats.Red += red
		stats.Green += green
		stats.Blue += blue
		stats.Spread += max3(red, green, blue) - min3(red, green, blue)
	}

	if stats.Samples == 0 {
		return rgbStats{}, errors.New("CSV has no sample rows")
	}
	samples := float64(stats.Samples)
	stats.Red /= samples
	stats.Green /= samples
	stats.Blue /= samples
	stats.Spread /= samples
	return stats, nil
}

func printStats(label string, stats rgbStats) {
	fmt.Printf("%s\n", label)
	fmt.Printf("samples: %d\n", stats.Samples)
	fmt.Printf("average_rgb: red=%s green=%s blue=%s\n", formatFloat(stats.Red), formatFloat(stats.Green), formatFloat(stats.Blue))
	fmt.Printf("average_spread: %s\n", formatFloat(stats.Spread))
}

func printComparison(originalLabel string, original rgbStats, correctedLabel string, corrected rgbStats) {
	fmt.Printf("original: %s\n", originalLabel)
	fmt.Printf("  samples: %d\n", original.Samples)
	fmt.Printf("  average_rgb: red=%s green=%s blue=%s\n", formatFloat(original.Red), formatFloat(original.Green), formatFloat(original.Blue))
	fmt.Printf("  average_spread: %s\n", formatFloat(original.Spread))
	fmt.Printf("corrected: %s\n", correctedLabel)
	fmt.Printf("  samples: %d\n", corrected.Samples)
	fmt.Printf("  average_rgb: red=%s green=%s blue=%s\n", formatFloat(corrected.Red), formatFloat(corrected.Green), formatFloat(corrected.Blue))
	fmt.Printf("  average_spread: %s\n", formatFloat(corrected.Spread))
	fmt.Printf("delta_rgb: red=%s green=%s blue=%s\n", formatSignedFloat(corrected.Red-original.Red), formatSignedFloat(corrected.Green-original.Green), formatSignedFloat(corrected.Blue-original.Blue))
	fmt.Printf("spread_delta: %s\n", formatSignedFloat(corrected.Spread-original.Spread))
	fmt.Printf("spread_improvement: %s\n", formatSignedFloat(original.Spread-corrected.Spread))
}

func drawGraph(path string, samples []sample) error {
	const (
		width       = 1200
		height      = 700
		leftMargin  = 80
		rightMargin = 35
		topMargin   = 40
		botMargin   = 75
	)

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{245, 247, 250, 255}}, image.Point{}, draw.Src)

	plot := image.Rect(leftMargin, topMargin, width-rightMargin, height-botMargin)
	drawFilledRect(img, plot, color.RGBA{255, 255, 255, 255})
	drawAxes(img, plot)

	maxTime := samples[len(samples)-1].Time
	if maxTime <= 0 {
		maxTime = 1
	}

	for i := 0; i <= 5; i++ {
		y := plot.Max.Y - int(math.Round(float64(i)*float64(plot.Dy())/5))
		drawLine(img, plot.Min.X, y, plot.Max.X, y, color.RGBA{225, 229, 235, 255})
		label := strconv.Itoa(i * 51)
		drawText(img, plot.Min.X-55, y-4, label, color.RGBA{80, 86, 96, 255})
	}

	for i := 0; i <= 6; i++ {
		x := plot.Min.X + int(math.Round(float64(i)*float64(plot.Dx())/6))
		drawLine(img, x, plot.Min.Y, x, plot.Max.Y, color.RGBA{235, 238, 243, 255})
		seconds := maxTime * float64(i) / 6
		drawText(img, x-12, plot.Max.Y+18, formatTick(seconds), color.RGBA{80, 86, 96, 255})
	}

	drawSeries(img, samples, maxTime, plot, func(s sample) float64 { return s.Red }, color.RGBA{220, 45, 45, 255})
	drawSeries(img, samples, maxTime, plot, func(s sample) float64 { return s.Green }, color.RGBA{35, 150, 70, 255})
	drawSeries(img, samples, maxTime, plot, func(s sample) float64 { return s.Blue }, color.RGBA{45, 95, 230, 255})

	drawText(img, 430, 14, "Average RGB Color Concentration Over Time", color.RGBA{35, 40, 48, 255})
	drawText(img, 505, height-28, "time in seconds", color.RGBA{60, 66, 76, 255})
	drawText(img, 14, 20, "intensity", color.RGBA{60, 66, 76, 255})
	drawLegend(img, width-250, 45)

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return png.Encode(file, img)
}

func drawAxes(img *image.RGBA, plot image.Rectangle) {
	drawLine(img, plot.Min.X, plot.Min.Y, plot.Min.X, plot.Max.Y, color.RGBA{75, 82, 92, 255})
	drawLine(img, plot.Min.X, plot.Max.Y, plot.Max.X, plot.Max.Y, color.RGBA{75, 82, 92, 255})
}

func drawSeries(img *image.RGBA, samples []sample, maxTime float64, plot image.Rectangle, value func(sample) float64, c color.RGBA) {
	if len(samples) < 2 {
		return
	}
	for i := 1; i < len(samples); i++ {
		x1, y1 := graphPoint(samples[i-1].Time, value(samples[i-1]), maxTime, plot)
		x2, y2 := graphPoint(samples[i].Time, value(samples[i]), maxTime, plot)
		drawThickLine(img, x1, y1, x2, y2, c)
	}
}

func graphPoint(t, v, maxTime float64, plot image.Rectangle) (int, int) {
	v = math.Max(0, math.Min(255, v))
	x := plot.Min.X + int(math.Round((t/maxTime)*float64(plot.Dx())))
	y := plot.Max.Y - int(math.Round((v/255)*float64(plot.Dy())))
	return x, y
}

func drawLegend(img *image.RGBA, x, y int) {
	drawFilledRect(img, image.Rect(x-10, y-12, x+190, y+72), color.RGBA{255, 255, 255, 235})
	drawText(img, x, y, "Legend", color.RGBA{45, 50, 58, 255})
	drawLegendItem(img, x, y+22, "Red", color.RGBA{220, 45, 45, 255})
	drawLegendItem(img, x, y+42, "Green", color.RGBA{35, 150, 70, 255})
	drawLegendItem(img, x, y+62, "Blue", color.RGBA{45, 95, 230, 255})
}

func drawLegendItem(img *image.RGBA, x, y int, label string, c color.RGBA) {
	drawThickLine(img, x, y+5, x+30, y+5, c)
	drawText(img, x+40, y, label, color.RGBA{45, 50, 58, 255})
}

func drawFilledRect(img *image.RGBA, rect image.Rectangle, c color.RGBA) {
	draw.Draw(img, rect, &image.Uniform{C: c}, image.Point{}, draw.Src)
}

func drawThickLine(img *image.RGBA, x0, y0, x1, y1 int, c color.RGBA) {
	drawLine(img, x0, y0, x1, y1, c)
	drawLine(img, x0, y0+1, x1, y1+1, c)
	drawLine(img, x0, y0-1, x1, y1-1, c)
}

func drawLine(img *image.RGBA, x0, y0, x1, y1 int, c color.RGBA) {
	dx := abs(x1 - x0)
	dy := -abs(y1 - y0)
	sx := -1
	if x0 < x1 {
		sx = 1
	}
	sy := -1
	if y0 < y1 {
		sy = 1
	}
	err := dx + dy

	for {
		if image.Pt(x0, y0).In(img.Bounds()) {
			img.SetRGBA(x0, y0, c)
		}
		if x0 == x1 && y0 == y1 {
			return
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}

func drawText(img *image.RGBA, x, y int, text string, c color.RGBA) {
	for _, r := range strings.ToUpper(text) {
		if r == ' ' {
			x += 6
			continue
		}
		glyph, ok := font5x7[r]
		if !ok {
			x += 6
			continue
		}
		for row, bits := range glyph {
			for col := 0; col < 5; col++ {
				if bits&(1<<(4-col)) != 0 {
					setPixel(img, x+col, y+row, c)
				}
			}
		}
		x += 6
	}
}

func setPixel(img *image.RGBA, x, y int, c color.RGBA) {
	if image.Pt(x, y).In(img.Bounds()) {
		img.SetRGBA(x, y, c)
	}
}

func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', 3, 64)
}

func formatSignedFloat(v float64) string {
	if v >= 0 {
		return "+" + formatFloat(v)
	}
	return formatFloat(v)
}

func formatTick(v float64) string {
	if v >= 100 {
		return strconv.FormatFloat(v, 'f', 0, 64)
	}
	return strconv.FormatFloat(v, 'f', 1, 64)
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func max3(a, b, c float64) float64 {
	return math.Max(a, math.Max(b, c))
}

func min3(a, b, c float64) float64 {
	return math.Min(a, math.Min(b, c))
}

var font5x7 = map[rune][7]byte{
	'0': {0x0E, 0x11, 0x13, 0x15, 0x19, 0x11, 0x0E},
	'1': {0x04, 0x0C, 0x04, 0x04, 0x04, 0x04, 0x0E},
	'2': {0x0E, 0x11, 0x01, 0x02, 0x04, 0x08, 0x1F},
	'3': {0x1E, 0x01, 0x01, 0x0E, 0x01, 0x01, 0x1E},
	'4': {0x02, 0x06, 0x0A, 0x12, 0x1F, 0x02, 0x02},
	'5': {0x1F, 0x10, 0x10, 0x1E, 0x01, 0x01, 0x1E},
	'6': {0x0E, 0x10, 0x10, 0x1E, 0x11, 0x11, 0x0E},
	'7': {0x1F, 0x01, 0x02, 0x04, 0x08, 0x08, 0x08},
	'8': {0x0E, 0x11, 0x11, 0x0E, 0x11, 0x11, 0x0E},
	'9': {0x0E, 0x11, 0x11, 0x0F, 0x01, 0x01, 0x0E},
	'A': {0x0E, 0x11, 0x11, 0x1F, 0x11, 0x11, 0x11},
	'B': {0x1E, 0x11, 0x11, 0x1E, 0x11, 0x11, 0x1E},
	'C': {0x0E, 0x11, 0x10, 0x10, 0x10, 0x11, 0x0E},
	'D': {0x1E, 0x11, 0x11, 0x11, 0x11, 0x11, 0x1E},
	'E': {0x1F, 0x10, 0x10, 0x1E, 0x10, 0x10, 0x1F},
	'F': {0x1F, 0x10, 0x10, 0x1E, 0x10, 0x10, 0x10},
	'G': {0x0E, 0x11, 0x10, 0x17, 0x11, 0x11, 0x0E},
	'H': {0x11, 0x11, 0x11, 0x1F, 0x11, 0x11, 0x11},
	'I': {0x0E, 0x04, 0x04, 0x04, 0x04, 0x04, 0x0E},
	'J': {0x01, 0x01, 0x01, 0x01, 0x11, 0x11, 0x0E},
	'K': {0x11, 0x12, 0x14, 0x18, 0x14, 0x12, 0x11},
	'L': {0x10, 0x10, 0x10, 0x10, 0x10, 0x10, 0x1F},
	'M': {0x11, 0x1B, 0x15, 0x15, 0x11, 0x11, 0x11},
	'N': {0x11, 0x19, 0x15, 0x13, 0x11, 0x11, 0x11},
	'O': {0x0E, 0x11, 0x11, 0x11, 0x11, 0x11, 0x0E},
	'P': {0x1E, 0x11, 0x11, 0x1E, 0x10, 0x10, 0x10},
	'Q': {0x0E, 0x11, 0x11, 0x11, 0x15, 0x12, 0x0D},
	'R': {0x1E, 0x11, 0x11, 0x1E, 0x14, 0x12, 0x11},
	'S': {0x0F, 0x10, 0x10, 0x0E, 0x01, 0x01, 0x1E},
	'T': {0x1F, 0x04, 0x04, 0x04, 0x04, 0x04, 0x04},
	'U': {0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x0E},
	'V': {0x11, 0x11, 0x11, 0x11, 0x11, 0x0A, 0x04},
	'W': {0x11, 0x11, 0x11, 0x15, 0x15, 0x15, 0x0A},
	'X': {0x11, 0x11, 0x0A, 0x04, 0x0A, 0x11, 0x11},
	'Y': {0x11, 0x11, 0x0A, 0x04, 0x04, 0x04, 0x04},
	'Z': {0x1F, 0x01, 0x02, 0x04, 0x08, 0x10, 0x1F},
	'.': {0x00, 0x00, 0x00, 0x00, 0x00, 0x0C, 0x0C},
	'-': {0x00, 0x00, 0x00, 0x1F, 0x00, 0x00, 0x00},
	':': {0x00, 0x0C, 0x0C, 0x00, 0x0C, 0x0C, 0x00},
}
