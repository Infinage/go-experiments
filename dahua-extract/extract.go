package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// stallTimeout is how long the stream may sit without producing data before
// we stop waiting. Dahua XVRs keep the RTSP session open without sending data
// or EOF when the requested window extends past the end of the recording (or
// crosses a recording gap); ffmpeg would otherwise wait until the XVR finally
// drops the session, which can take minutes. stallTimeout is passed to ffmpeg
// as its RTSP socket I/O timeout (-timeout, µs) so a stalled read fails inside
// ffmpeg and it still finalizes the mp4; the watchdog below is a backstop.
const stallTimeout = 10 * time.Second

// streamStall is returned when the stream stopped producing data and ffmpeg
// was interrupted after stallTimeout. pos is the last stream position reached.
type streamStall struct{ pos time.Duration }

func (e *streamStall) Error() string {
	return fmt.Sprintf("no data for %v, stopped at %s", stallTimeout, durationStr(e.pos))
}

// Layouts accepted by -start/-end. The first that parses wins.
var timeLayouts = []string{
	"2006-01-02 15:04:05Z07:00",
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05",
	"2006-01-02 15:04",
	"2006-01-02T15:04",
	"2006-01-02",
}

// Dahua requires exactly this format for starttime/endtime.
const dahuaTime = "2006_01_02_15_04_05"

const barWidth = 30

// shotTimeout is the hard wall-clock limit for one screenshot: covers the
// RTSP connect (not bounded by -rw_timeout) plus data wait and frame grab.
const shotTimeout = 20 * time.Second

func usage() {
	fmt.Fprintf(os.Stderr, `usage: extract <subcommand> [flags]

subcommands:
  vid    extract recorded video as mp4 (stream copy, no re-encode)
  shots  extract individual screenshots from a recording window

examples:
  extract vid -ip 192.168.1.10 -channels 1,3 \
    -start "2026-11-11 05:00:00" -end "2026-11-11 05:10:00"

  extract shots -ip 192.168.1.10 -channels 1,3 \
    -start "2026-11-11 05:00:00" -end "2026-11-11 05:10:00" \
    -every 5s -out clips1

  extract shots -ip 192.168.1.10 -channels 1,3 \
    -start "2026-11-11 05:00:00" -end "2026-11-11 05:10:00" \
    -count 10 -out clips1
`)
}

func main() {
	// Fail fast on missing external binaries before doing any work.
	for _, bin := range []string{"ffmpeg", "stty"} {
		if _, err := exec.LookPath(bin); err != nil {
			fmt.Fprintf(os.Stderr, "required binary %q not found in PATH: %v\n", bin, err)
			os.Exit(1)
		}
	}

	args := os.Args[1:]
	if len(args) == 0 {
		usage()
		os.Exit(1)
	}
	switch args[0] {
	case "vid":
		runVid(args[1:])
	case "shots":
		runShots(args[1:])
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q\n\n", args[0])
		usage()
		os.Exit(1)
	}
}

// resolveConn fills in ip from the flag/env/prompt chain and returns
// username/password from env or interactive prompts. Shared by vid and shots.
func resolveConn(ip *string) (user, pass string) {
	user = os.Getenv("username")
	if user == "" {
		user = promptVisible("username: ")
	}

	pass = os.Getenv("password")
	if pass == "" {
		pass = promptHidden("password (input hidden): ")
	}

	if *ip == "" {
		*ip = os.Getenv("ip")
	}
	if *ip == "" {
		*ip = promptVisible("XVR ip: ")
	}
	return user, pass
}

// resolveWindow parses and validates -start/-end (prompting if empty) and
// returns them plus the parsed channel list. Shared by vid and shots.
func resolveWindow(startRaw, endRaw, channelsRaw *string) (start, end time.Time, chans []int) {
	if *startRaw == "" {
		*startRaw = promptVisible(fmt.Sprintf("start time (e.g. %q): ", "2026-11-11 05:00:00"))
	}
	if *endRaw == "" {
		*endRaw = promptVisible(fmt.Sprintf("end time (e.g. %q): ", "2026-11-11 05:10:00"))
	}

	var err error
	start, err = parseTime(*startRaw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid -start: %v\n", err)
		os.Exit(1)
	}
	end, err = parseTime(*endRaw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid -end: %v\n", err)
		os.Exit(1)
	}
	if !end.After(start) {
		fmt.Fprintln(os.Stderr, "-end must be after -start")
		os.Exit(1)
	}

	chans, err = parseChannels(*channelsRaw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid -channels: %v\n", err)
		os.Exit(1)
	}
	return start, end, chans
}

// barCursor repaints ONE bar line in place, with per-channel status lines
// accumulating below it as channels complete.
//
// Cursor protocol: the bar is drawn without a trailing newline. Status lines
// are printed below it; after the first status the cursor lives on the empty
// row below the last status, and each repaint moves up (statuses+1) rows to
// the bar row and back down. All positioning is relative, so terminal
// scrolling stays correct.
type barCursor struct {
	mu       sync.Mutex
	statuses int
	drawn    bool
}

func (c *barCursor) paint(line string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.drawn {
		c.drawn = true
		fmt.Print("\r" + line) // no trailing newline; status() ends the row
		return
	}
	if c.statuses == 0 {
		fmt.Print("\r\033[K" + line)
		return
	}
	fmt.Printf("\033[%dA\r\033[K%s\033[%dB", c.statuses+1, line, c.statuses+1)
}

// status prints a per-channel result line below the bar.
func (c *barCursor) status(line string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.drawn {
		c.drawn = true
		fmt.Print("\r\033[K")
	}
	fmt.Println()
	fmt.Println(line)
	c.statuses++
}

// --- vid subcommand ---------------------------------------------------------

// vidProgress drives the single aggregate bar: channels run in parallel over
// the same window, so the bar shows the average progress across channels.
type vidProgress struct {
	cur      barCursor
	mu       sync.Mutex
	progress []float64
	total    time.Duration
}

func newVidProgress(n int, total time.Duration) *vidProgress {
	return &vidProgress{progress: make([]float64, n), total: total}
}

func (v *vidProgress) setProgress(idx int, p float64) {
	v.mu.Lock()
	v.progress[idx] = p
	_, line := v.barLine()
	v.mu.Unlock()
	v.cur.paint(line)
}

// complete finalizes the channel at 100%, repaints the bar, then appends the
// channel's result line below it.
func (v *vidProgress) complete(idx int, status string) {
	v.mu.Lock()
	v.progress[idx] = 1
	_, line := v.barLine()
	v.mu.Unlock()
	v.cur.paint(line)
	v.cur.status(status)
}

func (v *vidProgress) barLine() (float64, string) {
	avg := 0.0
	for _, p := range v.progress {
		avg += p
	}
	avg /= float64(len(v.progress))
	pos := time.Duration(avg * float64(v.total))
	line := fmt.Sprintf("%dch %s %3.0f%%  %s / %s  remaining %s",
		len(v.progress), bar(avg), avg*100, durationStr(pos), durationStr(v.total), durationStr(v.total-pos))
	return avg, line
}

// --- shots subcommand -------------------------------------------------------

// shotsProgress drives the single aggregate bar for shots: progress is shots
// completed across all channels; failed shots are counted and shown live.
type shotsProgress struct {
	cur        barCursor
	mu         sync.Mutex
	done       []int
	failed     []int
	perChannel int
}

func newShotsProgress(n, perChannel int) *shotsProgress {
	return &shotsProgress{done: make([]int, n), failed: make([]int, n), perChannel: perChannel}
}

func (s *shotsProgress) shotDone(idx int) {
	s.mu.Lock()
	s.done[idx]++
	line := s.barLine()
	s.mu.Unlock()
	s.cur.paint(line)
}

func (s *shotsProgress) shotFailed(idx int) {
	s.mu.Lock()
	s.failed[idx]++
	line := s.barLine()
	s.mu.Unlock()
	s.cur.paint(line)
}

// complete finalizes the channel at 100%, repaints the bar, then appends the
// channel's result line below it.
func (s *shotsProgress) complete(idx int, status string) {
	s.mu.Lock()
	s.done[idx] = s.perChannel
	line := s.barLine()
	s.mu.Unlock()
	s.cur.paint(line)
	s.cur.status(status)
}

func (s *shotsProgress) barLine() string {
	done, failed := 0, 0
	for i := range s.done {
		done += s.done[i]
		failed += s.failed[i]
	}
	total := len(s.done) * s.perChannel
	p := 0.0
	if total > 0 {
		p = float64(done) / float64(total)
	}
	line := fmt.Sprintf("%dch %s %d/%d shots", len(s.done), bar(p), done, total)
	if failed > 0 {
		line += fmt.Sprintf("  %d failed", failed)
	}
	return line
}

func runVid(args []string) {
	fs := flag.NewFlagSet("vid", flag.ExitOnError)
	ip := fs.String("ip", "", "XVR IP address")
	channels := fs.String("channels", "", "comma-separated channel numbers, e.g. 1,3,4")
	startRaw := fs.String("start", "", "recording start time")
	endRaw := fs.String("end", "", "recording end time")
	fs.Parse(args)

	user, pass := resolveConn(ip)
	start, end, chans := resolveWindow(startRaw, endRaw, channels)

	baseName := promptVisible("output file name (without extension): ")
	baseName = strings.TrimSpace(baseName)
	if baseName == "" {
		fmt.Fprintln(os.Stderr, "output file name must not be empty")
		os.Exit(1)
	}
	for _, ch := range chans {
		out := fmt.Sprintf("%s_ch%d.mp4", baseName, ch)
		if _, err := os.Stat(out); err == nil {
			fmt.Fprintf(os.Stderr, "output file %s already exists; remove it or choose another name\n", out)
			os.Exit(1)
		} else if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "checking output file %s: %v\n", out, err)
			os.Exit(1)
		}
	}

	startStr := start.Format(dahuaTime)
	endStr := end.Format(dahuaTime)
	total := end.Sub(start)

	fmt.Printf("\nExtracting %d channel(s) from %s: %s -> %s (~%s at 1x speed per channel)\n\n",
		len(chans), *ip, startStr, endStr, durationStr(total))

	r := newVidProgress(len(chans), total)
	var wg sync.WaitGroup
	failed := make([]int, 0, len(chans))
	var failMu sync.Mutex

	for i, ch := range chans {
		wg.Add(1)
		go func(idx, ch int) {
			defer wg.Done()
			out := fmt.Sprintf("%s_ch%d.mp4", baseName, ch)
			err := extractChannel(user, pass, *ip, ch, startStr, endStr, out, total,
				func(progress float64) {
					r.setProgress(idx, progress)
				})
			var stall *streamStall
			if errors.As(err, &stall) && stall.pos > 0 {
				// Stream ended early because the recording data ends before the
				// window does (or a gap was crossed). The mp4 is finalized.
				r.complete(idx, fmt.Sprintf("ch%d done (recording data ends at %s; no data for %v after that) -> %s",
					ch, durationStr(stall.pos), stallTimeout, out))
			} else if errors.As(err, &stall) {
				r.complete(idx, fmt.Sprintf("ch%d FAILED: no data received (no recording in this window?)", ch))
				failMu.Lock()
				failed = append(failed, ch)
				failMu.Unlock()
			} else if err != nil {
				r.complete(idx, fmt.Sprintf("ch%d FAILED: %v", ch, err))
				failMu.Lock()
				failed = append(failed, ch)
				failMu.Unlock()
			} else {
				r.complete(idx, fmt.Sprintf("ch%d done -> %s", ch, out))
			}
		}(i, ch)
	}
	wg.Wait()
	fmt.Println()

	if len(failed) > 0 {
		fmt.Printf("Extraction failed for channel(s): %v\n", failed)
		os.Exit(1)
	}
	fmt.Println("All extractions succeeded.")
}

// extractChannel runs ffmpeg for one channel, reporting progress 0..1 via fn.
// ffmpeg writes progress lines to stdout (-progress pipe:1); stderr is captured
// so it does not corrupt the progress display, and shown only on failure.
func extractChannel(user, pass, ip string, ch int, startStr, endStr, outFile string, total time.Duration, fn func(float64)) error {
	rtspURL := fmt.Sprintf("rtsp://%s:%s@%s:554/cam/playback?channel=%d&starttime=%s&endtime=%s",
		user, pass, ip, ch, startStr, endStr)

	// -y overwrites the file if it exists
	// -c copy extracts the raw H.264/H.265 data without degrading quality
	// -t stops after exactly the requested duration: some Dahua XVRs keep
	//   streaming past endtime without EOF, which would otherwise hang ffmpeg
	//   and leave the mp4 un-finalized (missing moov atom)
	// -nostats hides ffmpeg's own progress spam; -progress gives us
	//   machine-readable out_time_us= lines to drive our own bar.
	// -t must be a plain number of seconds: Go's Duration.String() emits
	// "2m0s", which ffmpeg rejects as an invalid duration. start/end have
	// second resolution, so total is always a whole number of seconds.
	// -timeout: RTSP socket I/O timeout in µs. When the stream stalls (no data
	//   for stallTimeout), the read fails inside ffmpeg so it exits gracefully
	//   and finalizes the mp4, instead of blocking forever waiting for packets.
	cmd := exec.Command("ffmpeg", "-y", "-nostats", "-progress", "pipe:1",
		"-t", strconv.FormatInt(int64(total/time.Second), 10),
		"-timeout", strconv.FormatInt(int64(stallTimeout/time.Microsecond), 10),
		"-i", rtspURL, "-c", "copy", outFile)
	err := runFFmpegWithProgress(cmd, total, fn)
	var stall *streamStall
	if errors.As(err, &stall) && stall.pos == 0 {
		os.Remove(outFile) // drop the empty mp4 from a window with no recording
	}
	return err
}

// runFFmpegWithProgress starts cmd, parses ffmpeg's -progress pipe:1 output on
// stdout to drive fn(0..1), and returns an error carrying the tail of stderr
// if ffmpeg exits non-zero. A watchdog interrupts ffmpeg (gracefully, so the
// mp4 still gets finalized) if the stream position stops advancing for
// stallTimeout, and returns a *streamStall in that case.
func runFFmpegWithProgress(cmd *exec.Cmd, total time.Duration, fn func(float64)) error {
	var stderr strings.Builder
	cmd.Stderr = &stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting ffmpeg: %w", err)
	}

	// Watchdog: the stream can stall with no data at all (recording gap or
	// window past the end of the recording) while the XVR keeps the session
	// open. After stallTimeout without a position change, interrupt ffmpeg
	// gracefully; if it still hasn't exited shortly after, kill it.
	var mu sync.Mutex
	var lastPos time.Duration
	lastChange := time.Now()
	signalled := false
	killed := false
	done := make(chan struct{})
	go func() {
		t := time.NewTicker(time.Second)
		defer t.Stop()
		deadline := time.Time{}
		for {
			select {
			case <-done:
				return
			case <-t.C:
				mu.Lock()
				idle := time.Since(lastChange)
				switch {
				case idle >= stallTimeout && !signalled:
					signalled = true
					deadline = time.Now().Add(3 * time.Second)
					mu.Unlock()
					cmd.Process.Signal(os.Interrupt) // graceful: finalizes the mp4
					continue
				case signalled && !killed && !deadline.IsZero() && time.Now().After(deadline):
					killed = true
					mu.Unlock()
					cmd.Process.Kill()
					continue
				}
				mu.Unlock()
			}
		}
	}()

	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 0, 64*1024), 64*1024)
	var lastSec int64 = -1
	sawProgress := false // any out_time line received at all
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "out_time_us=") {
			if us, err := strconv.ParseInt(line[len("out_time_us="):], 10, 64); err == nil && us >= 0 {
				sawProgress = true
				pos := time.Duration(us) * time.Microsecond
				mu.Lock()
				if pos != lastPos {
					lastPos = pos
					lastChange = time.Now()
				}
				mu.Unlock()
				p := float64(pos) / float64(total)
				if p > 1 {
					p = 1
				}
				// Redraw once per second of stream position: ffmpeg emits a
				// progress line ~every 0.5s and stays at 100% for a while
				// before exiting, so gate on position, not on percent
				// (percent granularity would slow updates on long windows).
				if sec := int64(pos / time.Second); sec != lastSec {
					lastSec = sec
					fn(p)
				}
			}
		}
	}

	err = cmd.Wait()
	close(done)

	msg := strings.TrimSpace(stderr.String())
	mu.Lock()
	signalled, wasKilled, pos := signalled, killed, lastPos
	mu.Unlock()
	if wasKilled {
		return fmt.Errorf("ffmpeg did not exit after %v with no data; killed; output may be incomplete", stallTimeout)
	}
	if signalled || strings.Contains(msg, "Connection timed out") {
		// ffmpeg's RTSP read timeout (or the watchdog's SIGINT) ended the
		// stalled read; ffmpeg finalized the mp4 before exiting.
		return &streamStall{pos: pos}
	}
	if err != nil {
		const max = 300
		if len(msg) > max {
			msg = msg[len(msg)-max:]
		}
		// Keep the failure single-line: it is printed under the progress bar.
		msg = strings.ReplaceAll(msg, "\n", " | ")
		return fmt.Errorf("%v: %s", err, msg)
	}
	if !sawProgress {
		// ffmpeg exited cleanly but never got a single packet: the window has
		// no recording data for this channel (the XVR closes the session
		// immediately instead of stalling).
		return &streamStall{pos: 0}
	}
	fn(1)
	return nil
}

// --- shots subcommand -------------------------------------------------------

func runShots(args []string) {
	fs := flag.NewFlagSet("shots", flag.ExitOnError)
	ip := fs.String("ip", "", "XVR IP address")
	channels := fs.String("channels", "", "comma-separated channel numbers, e.g. 1,3,4")
	startRaw := fs.String("start", "", "recording start time")
	endRaw := fs.String("end", "", "recording end time")
	every := fs.String("every", "", "sample interval, e.g. 5s, 30s, 2m, 1m30s")
	count := fs.Int("count", 0, "number of evenly spaced screenshots (first at start, last at end)")
	out := fs.String("out", "", "output directory for jpg files")
	fs.Parse(args)

	if *every != "" && *count > 0 {
		fmt.Fprintln(os.Stderr, "-every and -count are mutually exclusive; give exactly one")
		os.Exit(1)
	}
	var everyDur time.Duration
	if *every != "" {
		d, err := time.ParseDuration(*every)
		if err != nil || d <= 0 {
			fmt.Fprintf(os.Stderr, "invalid -every %q: must be a positive duration like 5s, 30s, 2m, 1m30s\n", *every)
			os.Exit(1)
		}
		everyDur = d
	}

	start, end, chans := resolveWindow(startRaw, endRaw, channels)

	var times []time.Time
	var mode string
	if everyDur > 0 {
		for t := start; t.Before(end); t = t.Add(everyDur) {
			times = append(times, t)
		}
		mode = fmt.Sprintf("every %s", *every)
	} else if *count > 0 {
		n := *count
		if n == 1 {
			times = []time.Time{start}
		} else {
			step := end.Sub(start) / time.Duration(n-1)
			for i := 0; i < n; i++ {
				times = append(times, start.Add(step*time.Duration(i)))
			}
		}
		mode = fmt.Sprintf("%d evenly spaced", n)
	} else {
		fmt.Fprintln(os.Stderr, "give exactly one of -every or -count")
		os.Exit(1)
	}
	if len(times) == 0 {
		fmt.Fprintln(os.Stderr, "sampling interval is larger than the recording window; no screenshots to take")
		os.Exit(1)
	}

	user, pass := resolveConn(ip)

	outDir := strings.TrimSpace(*out)
	if outDir == "" {
		outDir = promptVisible("output directory (jpg files): ")
		outDir = strings.TrimSpace(outDir)
	}
	if outDir == "" {
		fmt.Fprintln(os.Stderr, "output directory must not be empty")
		os.Exit(1)
	}
	if _, err := os.Stat(outDir); err == nil {
		fmt.Fprintf(os.Stderr, "output directory %s already exists; remove it or choose another name\n", outDir)
		os.Exit(1)
	} else if !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "checking output directory: %v\n", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "creating output directory: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nTaking %d screenshot(s) per channel (%s) from %s: %s -> %s -> %s\n\n",
		len(times), mode, *ip, start.Format(dahuaTime), end.Format(dahuaTime), outDir)

	s := newShotsProgress(len(chans), len(times))
	var wg sync.WaitGroup
	failedChans := make([]int, 0, len(chans))
	var failMu sync.Mutex

	for i, ch := range chans {
		wg.Add(1)
		go func(idx, ch int) {
			defer wg.Done()
			failed := 0
			var lastFail time.Time
			for j, t := range times {
				outFile := filepath.Join(outDir, fmt.Sprintf("ch%d_%04d.jpg", ch, j+1))
				if err := grabShot(user, pass, *ip, ch, t, outFile); err != nil {
					failed++
					lastFail = t
					s.shotFailed(idx)
					continue
				}
				s.shotDone(idx)
			}
			if failed > 0 {
				s.complete(idx, fmt.Sprintf("ch%d done with %d/%d failed shot(s), last at %s -> %s",
					ch, failed, len(times), lastFail.Format(dahuaTime), outDir))
				failMu.Lock()
				failedChans = append(failedChans, ch)
				failMu.Unlock()
			} else {
				s.complete(idx, fmt.Sprintf("ch%d done -> %s", ch, outDir))
			}
		}(i, ch)
	}
	wg.Wait()
	fmt.Println()

	if len(failedChans) > 0 {
		fmt.Printf("Some shots failed for channel(s): %v\n", failedChans)
		os.Exit(1)
	}
	fmt.Println("All screenshots taken.")
}

// grabShot captures a single frame by opening a short playback connection that
// starts exactly at time t and asking ffmpeg for the first decoded frame.
// The frame lands at or slightly before t (the server starts streaming from
// the keyframe at-or-before starttime; precision is limited by the GOP length,
// typically 1-2s on Dahua XVRs).
func grabShot(user, pass, ip string, ch int, t time.Time, outFile string) error {
	rtspURL := fmt.Sprintf("rtsp://%s:%s@%s:554/cam/playback?channel=%d&starttime=%s",
		user, pass, ip, ch, t.Format(dahuaTime))

	// -frames:v 1 exits right after the first decoded frame.
	// -q:v 2 requests high JPEG quality.
	// The Go-side context timeout (shotTimeout) is the hard stop for the
	// whole shot — TCP connect, no-data waits, everything — because ffmpeg
	// protocol-level timeouts don't bound the connect phase and -rw_timeout
	// is not accepted as an input option on all ffmpeg builds.
	ctx, cancel := context.WithTimeout(context.Background(), shotTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ffmpeg", "-y", "-nostats", "-loglevel", "error",
		"-i", rtspURL,
		"-frames:v", "1", "-q:v", "2", outFile)

	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("timed out after %s (XVR not reachable or no data sent)", shotTimeout)
		}
		msg := strings.TrimSpace(stderr.String())
		const max = 300
		if len(msg) > max {
			msg = msg[len(msg)-max:]
		}
		return fmt.Errorf("%v: %s", err, msg)
	}
	return nil
}

// --- shared helpers ---------------------------------------------------------

func bar(progress float64) string {
	filled := int(progress * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}
	return "[" + strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled) + "]"
}

func durationStr(d time.Duration) string {
	d = d.Round(time.Second)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

func parseTime(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	for _, layout := range timeLayouts {
		if t, err := time.Parse(layout, raw); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("%q does not match any accepted format (e.g. %q)", raw, "2026-11-11 05:00:00")
}

func parseChannels(raw string) ([]int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("no channels given, use -channels 1,3,4")
	}
	var out []int
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		n, err := strconv.Atoi(part)
		if err != nil || n < 1 {
			return nil, fmt.Errorf("%q is not a valid channel number", part)
		}
		out = append(out, n)
	}
	return out, nil
}

// promptVisible reads a normal echoed line.
func promptVisible(label string) string {
	fmt.Print(label)
	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line != "" {
			return line
		}
		fmt.Print(label)
	}
	fmt.Fprintln(os.Stderr)
	os.Exit(1)
	return ""
}

// promptHidden reads a line with terminal echo disabled (input invisible).
// Uses stty so no extra Go dependency is needed. Works on Linux/macOS.
func promptHidden(label string) string {
	fmt.Print(label)
	sttyEchoOff := exec.Command("stty", "-echo")
	sttyEchoOff.Stdin = os.Stdin
	sttyEchoOn := exec.Command("stty", "echo")
	sttyEchoOn.Stdin = os.Stdin

	if err := sttyEchoOff.Run(); err != nil {
		// Not a terminal (e.g. piped input); fall back to visible read.
		return promptVisible(label)
	}
	fmt.Println()

	sc := bufio.NewScanner(os.Stdin)
	var line string
	if sc.Scan() {
		line = strings.TrimSpace(sc.Text())
	}
	sttyEchoOn.Run()
	if line == "" {
		fmt.Fprintln(os.Stderr)
		os.Exit(1)
	}
	return line
}
