//go:build linux

package linux

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"sync"

	"github.com/LeakTechnologies/VideoTools/internal/utils"
)

const playerWindowTitle = "videotools-player"

type Controller struct {
	mu     sync.Mutex
	cmd    *exec.Cmd
	stdin  *bufio.Writer
	ctx    context.Context
	cancel context.CancelFunc
	path   string
}

func New() *Controller {
	return &Controller{}
}

func (c *Controller) Load(path string, offset float64) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.stopLocked()

	ctx, cancel := context.WithCancel(context.Background())
	args := []string{
		"-hide_banner", "-loglevel", "error",
		"-autoexit",
		"-window_title", playerWindowTitle,
		"-noborder",
		"-x", "0",
		"-y", "0",
	}
	if offset > 0 {
		args = append(args, "-ss", fmt.Sprintf("%.4f", offset))
	}
	args = append(args, path)

	cmd := exec.CommandContext(ctx, utils.GetFFplayPath(), args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return err
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return err
	}

	c.cmd = cmd
	c.stdin = bufio.NewWriter(stdin)
	c.ctx = ctx
	c.cancel = cancel
	c.path = path

	go cmd.Wait()
	return nil
}

func (c *Controller) Play() error {
	return c.send('p')
}

func (c *Controller) Pause() error {
	return c.send('p')
}

func (c *Controller) Seek(offset float64) error {
	if c.path == "" {
		return fmt.Errorf("no source loaded")
	}
	return c.Load(c.path, offset)
}

func (c *Controller) FullScreen() error {
	return c.send('f')
}

func (c *Controller) Stop() error {
	return c.send('q')
}

func (c *Controller) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stopLocked()
}

func (c *Controller) send(ch byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stdin == nil {
		return fmt.Errorf("player stdin unavailable")
	}
	if _, err := c.stdin.Write([]byte{ch}); err != nil {
		return err
	}
	return c.stdin.Flush()
}

func (c *Controller) stopLocked() {
	if c.stdin != nil {
		c.stdin.Write([]byte{'q'})
		c.stdin.Flush()
	}
	if c.cancel != nil {
		c.cancel()
	}
	c.cmd = nil
	c.stdin = nil
	c.cancel = nil
	c.path = ""
}
