//go:build linux

package player

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/LeakTechnologies/VideoTools/internal/utils"
)

const playerWindowTitle = "VideoToolsPlayer"

func newController() Controller {
	return &ffplayController{}
}

type ffplayController struct {
	mu     sync.Mutex
	cmd    *exec.Cmd
	stdin  *bufio.Writer
	ctx    context.Context
	cancel context.CancelFunc
	path   string
	paused bool
	seekT  *time.Timer
	seekAt float64
	volume int // 0-100
	winX   int
	winY   int
	winW   int
	winH   int
}

// pickLastID runs a command and returns the last whitespace-delimited token from stdout.
func pickLastID(cmd *exec.Cmd) string {
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	parts := strings.Fields(string(out))
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

var (
	keyFullscreen = []byte{'f'}
	keyPause      = []byte{'p'}
	keyQuit       = []byte{'q'}
	keyVolDown    = []byte{'9'}
	keyVolUp      = []byte{'0'}
)

func (c *ffplayController) Load(path string, offset float64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.path = path
	if c.volume == 0 {
		c.volume = 100
	}
	c.paused = true
	return c.startLocked(offset)
}

func (c *ffplayController) SetWindow(x, y, w, h int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.winX, c.winY, c.winW, c.winH = x, y, w, h
}

func (c *ffplayController) Play() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	// Only toggle if we believe we are paused.
	if c.paused {
		if err := c.sendLocked(keyPause); err != nil {
			return err
		}
	}
	c.paused = false
	return nil
}

func (c *ffplayController) Pause() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.paused {
		if err := c.sendLocked(keyPause); err != nil {
			return err
		}
	}
	c.paused = true
	return nil
}

func (c *ffplayController) Seek(offset float64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.path == "" {
		return fmt.Errorf("no source loaded")
	}
	if offset < 0 {
		offset = 0
	}
	c.seekAt = offset
	if c.seekT != nil {
		c.seekT.Stop()
	}
	c.seekT = time.AfterFunc(90*time.Millisecond, func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		// Timer may fire after stop; guard.
		if c.path == "" {
			return
		}
		_ = c.startLocked(c.seekAt)
	})
	return nil
}

func (c *ffplayController) FullScreen() error { return c.send(keyFullscreen) }
func (c *ffplayController) Stop() error       { return c.send(keyQuit) }
func (c *ffplayController) SetVolume(level float64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	target := int(level + 0.5)
	if target < 0 {
		target = 0
	}
	if target > 100 {
		target = 100
	}
	if target == c.volume {
		return nil
	}
	diff := target - c.volume
	c.volume = target

	if !c.runningLocked() {
		return nil
	}

	key := keyVolUp
	steps := diff
	if diff < 0 {
		key = keyVolDown
		steps = -diff
	}
	// Limit burst size to avoid overwhelming stdin.
	for i := 0; i < steps; i++ {
		if err := c.sendLocked(key); err != nil {
			return err
		}
		// Tiny delay to let ffplay process the keys.
		if steps > 8 {
			time.Sleep(8 * time.Millisecond)
		}
	}
	return nil
}

func (c *ffplayController) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stopLocked()
}

func (c *ffplayController) send(seq []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sendLocked(seq)
}

func (c *ffplayController) sendLocked(seq []byte) error {
	if !c.runningLocked() {
		return fmt.Errorf("ffplay not running")
	}
	if _, err := c.stdin.Write(seq); err != nil {
		return err
	}
	return c.stdin.Flush()
}

func (c *ffplayController) stopLocked() {
	if c.stdin != nil {
		c.stdin.Write(keyQuit)
		c.stdin.Flush()
	}
	if c.cancel != nil {
		c.cancel()
	}
	c.cmd = nil
	c.stdin = nil
	c.cancel = nil
	c.path = ""
	c.paused = false
	if c.seekT != nil {
		c.seekT.Stop()
		c.seekT = nil
	}
}

func (c *ffplayController) waitForExit(cmd *exec.Cmd, cancel context.CancelFunc, stderr *bytes.Buffer) {
	err := cmd.Wait()
	exit := ""
	if cmd.ProcessState != nil {
		exit = cmd.ProcessState.String()
	}
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			log.Printf("[ffplay] exit error: %v (%s) stderr=%s", err, exit, msg)
		} else {
			log.Printf("[ffplay] exit error: %v (%s)", err, exit)
		}
	} else {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			log.Printf("[ffplay] exit: %s stderr=%s", exit, msg)
		} else {
			log.Printf("[ffplay] exit: %s", exit)
		}
	}
	cancel()
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cmd = nil
	c.stdin = nil
	c.ctx = nil
	c.cancel = nil
	c.path = ""
	c.paused = false
	if c.seekT != nil {
		c.seekT.Stop()
		c.seekT = nil
	}
}

func (c *ffplayController) runningLocked() bool {
	if c.cmd == nil || c.stdin == nil {
		return false
	}
	if c.cmd.ProcessState != nil && c.cmd.ProcessState.Exited() {
		return false
	}
	return true
}

func (c *ffplayController) startLocked(offset float64) error {
	if _, err := exec.LookPath("ffplay"); err != nil {
		return fmt.Errorf("ffplay not found in PATH: %w", err)
	}

	if strings.TrimSpace(c.path) == "" {
		return fmt.Errorf("no input path set")
	}
	input := c.path

	c.stopLocked()
	c.path = input

	ctx, cancel := context.WithCancel(context.Background())
	args := []string{
		"-hide_banner",
		"-loglevel", "error",
		"-autoexit",
		"-window_title", playerWindowTitle,
		"-noborder",
	}
	if c.winW > 0 {
		args = append(args, "-x", fmt.Sprintf("%d", c.winW))
	}
	if c.winH > 0 {
		args = append(args, "-y", fmt.Sprintf("%d", c.winH))
	}
	if c.volume <= 0 {
		args = append(args, "-volume", "0")
	} else {
		args = append(args, "-volume", fmt.Sprintf("%d", c.volume))
	}
	if offset > 0 {
		args = append(args, "-ss", fmt.Sprintf("%.3f", offset))
	}
	args = append(args, input)

	cmd := exec.CommandContext(ctx, utils.GetFFplayPath(), args...)
	env := os.Environ()
	if c.winX != 0 || c.winY != 0 {
		// SDL honors SDL_VIDEO_WINDOW_POS for initial window placement.
		pos := fmt.Sprintf("%d,%d", c.winX, c.winY)
		env = append(env, fmt.Sprintf("SDL_VIDEO_WINDOW_POS=%s", pos))
	}
	if os.Getenv("SDL_VIDEODRIVER") == "" {
		// Auto-detect display server and set appropriate SDL video driver
		if os.Getenv("WAYLAND_DISPLAY") != "" {
			env = append(env, "SDL_VIDEODRIVER=wayland")
		} else {
			// Default to X11 for compatibility, but Wayland takes precedence if available
			env = append(env, "SDL_VIDEODRIVER=x11")
		}
	}
	if os.Getenv("XDG_RUNTIME_DIR") == "" {
		run := fmt.Sprintf("/run/user/%d", os.Getuid())
		if fi, err := os.Stat(run); err == nil && fi.IsDir() {
			env = append(env, fmt.Sprintf("XDG_RUNTIME_DIR=%s", run))
		}
	}
	cmd.Env = env
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return err
	}
	if err := cmd.Start(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return fmt.Errorf("ffplay start failed: %w (%s)", err, msg)
		}
		cancel()
		return err
	}
	log.Printf("[ffplay] start pid=%d args=%v pos=(%d,%d) size=%dx%d offset=%.3f vol=%d env(SDL_VIDEODRIVER=%s XDG_RUNTIME_DIR=%s DISPLAY=%s)", cmd.Process.Pid, args, c.winX, c.winY, c.winW, c.winH, offset, c.volume, os.Getenv("SDL_VIDEODRIVER"), os.Getenv("XDG_RUNTIME_DIR"), os.Getenv("DISPLAY"))

	c.cmd = cmd
	c.stdin = bufio.NewWriter(stdin)
	c.ctx = ctx
	c.cancel = cancel

	// Best-effort window placement via xdotool (X11 only) if available and not on Wayland.
	// Wayland compositors don't support window manipulation via xdotool.
	if c.winW > 0 && c.winH > 0 && os.Getenv("WAYLAND_DISPLAY") == "" {
		go func(title string, x, y, w, h int) {
			time.Sleep(120 * time.Millisecond)
			ffID := pickLastID(exec.Command("xdotool", "search", "--name", title))
			mainID := pickLastID(exec.Command("xdotool", "search", "--name", "VideoTools"))
			if ffID == "" {
				return
			}
			// Reparent into main window if found, then move/size.
			if mainID != "" {
				_ = exec.Command("xdotool", "windowreparent", ffID, mainID).Run()
			}
			_ = exec.Command("xdotool", "windowmove", ffID, fmt.Sprintf("%d", x), fmt.Sprintf("%d", y)).Run()
			_ = exec.Command("xdotool", "windowsize", ffID, fmt.Sprintf("%d", w), fmt.Sprintf("%d", h)).Run()
			_ = exec.Command("xdotool", "windowraise", ffID).Run()
		}(playerWindowTitle, c.winX, c.winY, c.winW, c.winH)
	}

	go c.waitForExit(cmd, cancel, &stderr)

	// Reapply paused state if needed (ffplay starts unpaused).
	if c.paused {
		time.Sleep(20 * time.Millisecond)
		_ = c.sendLocked(keyPause)
	}
	return nil
}
