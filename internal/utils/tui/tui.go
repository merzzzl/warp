package tui

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/jroimartin/gocui"

	"github.com/merzzzl/warp/internal/service"
	"github.com/merzzzl/warp/internal/utils/log"
)

type LogWriter struct {
	logs chan string
}

// Write interface for writing to a log.
func (l *LogWriter) Write(p []byte) (n int, err error) {
	l.logs <- string(p)

	return len(p), nil
}

// CreateTUI creates a TUI for the given service.
func CreateTUI(routes *service.Routes, traffic *service.Traffic) error {
	l := &LogWriter{logs: make(chan string, 100)}

	log.SetOutput(l)

	g, err := gocui.NewGui(gocui.OutputNormal)
	if err != nil {
		return err
	}

	defer g.Close()

	g.SetManagerFunc(func(g *gocui.Gui) error {
		return layout(g, routes, traffic, l.logs)
	})

	if err := g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, func(*gocui.Gui, *gocui.View) error {
		return gocui.ErrQuit
	}); err != nil {
		return err
	}

	if err := g.MainLoop(); err != nil && !errors.Is(err, gocui.ErrQuit) {
		return err
	}

	return nil
}

func layout(g *gocui.Gui, routes *service.Routes, traffic *service.Traffic, logs <-chan string) error {
	maxX, maxY := g.Size()

	if v, err := g.SetView("logs", 0, 0, maxX-21, maxY-1); err != nil {
		if !errors.Is(err, gocui.ErrUnknownView) {
			return err
		}

		v.Title = "Logs"

		go func() {
			for logMsg := range logs {
				g.Update(func(*gocui.Gui) error {
					fmt.Fprint(v, logMsg)

					lines := len(v.BufferLines()) - 1
					_, vy := v.Size()

					if lines > vy {
						ox, _ := v.Origin()

						if err := v.SetOrigin(ox, lines-vy); err != nil {
							return err
						}
					}

					return nil
				})
			}
		}()
	}

	if v, err := g.SetView("stats", maxX-20, 0, maxX-1, 3); err != nil {
		if !errors.Is(err, gocui.ErrUnknownView) {
			return err
		}

		v.Title = "Bandwidth"

		go func() {
			for range time.NewTicker(time.Millisecond * 100).C {
				g.Update(func(*gocui.Gui) error {
					v.Clear()

					in, out := traffic.GetRates()

					fmt.Fprintf(v, "In:  %.2f\n", in/1024)
					fmt.Fprintf(v, "Out: %.2f\n", out/1024)

					return nil
				})
			}
		}()
	}

	if v, err := g.SetView("ips", maxX-20, 4, maxX-1, maxY-1); err != nil {
		if !errors.Is(err, gocui.ErrUnknownView) {
			return err
		}

		v.Title = "IP List"

		go func() {
			for range time.NewTicker(time.Second * 1).C {
				g.Update(func(*gocui.Gui) error {
					v.Clear()

					ips := routes.GetAll()
					sort.Strings(ips)

					for _, ip := range ips {
						fmt.Fprintln(v, ip)
					}

					return nil
				})
			}
		}()
	}

	return nil
}
