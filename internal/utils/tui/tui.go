package tui

import (
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jroimartin/gocui"

	"github.com/merzzzl/warp/internal/service"
	"github.com/merzzzl/warp/internal/utils/log"
	"github.com/merzzzl/warp/internal/utils/network"
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
func CreateTUI(routes *service.Routes, traffic *service.Traffic, useFun bool) error {
	l := &LogWriter{logs: make(chan string, 100)}

	log.SetOutput(l)

	g, err := gocui.NewGui(gocui.Output256)
	if err != nil {
		return err
	}

	if useFun {
		go fun(g)
	} else {
		g.FgColor = gocui.Attribute(232)
	}

	g.BgColor = gocui.Attribute(235)

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

	if v, err := g.SetView("logs", 0, 0, maxX-21, maxY-21); err != nil {
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

	if v, err := g.SetView("pipes", 0, maxY-20, maxX-21, maxY-1); err != nil {
		if !errors.Is(err, gocui.ErrUnknownView) {
			return err
		}

		v.Title = "Pipes"

		go func() {
			for range time.NewTicker(time.Millisecond * 100).C {
				g.Update(func(*gocui.Gui) error {
					v.Clear()

					for _, pipe := range network.List() {
						slen := 22 - len(pipe.From())
						open := time.Unix(0, 0).UTC().Add(time.Since(pipe.OpenAt())).Format("15:04:05")

						fmt.Fprintf(v, "%s %s %s %s %s %s\n", log.Colorize(open, 7), log.Colorize(strings.ToUpper(pipe.Tag()), 11), pipe.From(), log.Colorize(strings.Repeat("»", slen), 6), log.Colorize(strings.ToUpper(pipe.Network()), 11), pipe.To())
					}

					return nil
				})
			}
		}()
	}

	if v, err := g.SetView("stats", maxX-20, 0, maxX-1, 5); err != nil {
		if !errors.Is(err, gocui.ErrUnknownView) {
			return err
		}

		v.Title = "Bandwidth"

		go func() {
			for range time.NewTicker(time.Millisecond * 100).C {
				g.Update(func(*gocui.Gui) error {
					v.Clear()

					in, out := traffic.GetRates()
					sin, sout := traffic.GetTransferred()

					fmt.Fprintf(v, "In:  %.2f Kb/s\n", in/1024)
					fmt.Fprintf(v, "     %s\n", log.Colorize(byteToSI(sin), 7))
					fmt.Fprintf(v, "Out: %.2f Kb/s\n", out/1024)
					fmt.Fprintf(v, "     %s\n", log.Colorize(byteToSI(sout), 7))

					return nil
				})
			}
		}()
	}

	if v, err := g.SetView("ips", maxX-20, 6, maxX-1, maxY-4); err != nil {
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

	if v, err := g.SetView("uptime", maxX-20, maxY-3, maxX-1, maxY-1); err != nil {
		if !errors.Is(err, gocui.ErrUnknownView) {
			return err
		}

		v.Title = "Uptime"

		start := time.Now()
		loader := []rune{'▓', '▒', '░', '░', '░', '░', '░', '░', '░', '▒'}
		bar := func() string {
			loader = append([]rune{loader[9]}, loader[:9]...)

			return string(loader[1:9])
		}

		go func() {
			for range time.NewTicker(time.Millisecond * 100).C {
				g.Update(func(*gocui.Gui) error {
					v.Clear()

					up := time.Unix(0, 0).UTC().Add(time.Since(start)).Format("15:04:05")

					fmt.Fprintf(v, "%8s %9s", bar(), up)

					return nil
				})
			}
		}()
	}

	return nil
}

func fColor() func() int {
	currentColor := 51
	color := func() int {
		if currentColor == 231 {
			currentColor = 51
		}

		currentColor++

		return currentColor
	}

	return color
}

func fArt() string {
	n := rand.Intn(100)

	if n < 99 {
		return ""
	}

	ts := []string{
		"⊂(◉‿◉)つ",
		"( ✜︵✜ )",
		"ʕっ•ᴥ•ʔっ",
		"(｡◕‿‿◕｡)",
		"(っ´ω`c)♡",
		"(ʘ‿ʘ)╯",
	}
	art := ts[rand.Intn(6)]

	return fmt.Sprintf("─%s%s", art, strings.Repeat("─", 10-utf8.RuneCountInString(art)))
}

func fun(g *gocui.Gui) {
	cl := fColor()

	v, err := g.SetView("fun", 0, -1, 11, 1)
	if !errors.Is(err, gocui.ErrUnknownView) {
		return
	}

	v.Frame = false

	fmt.Fprint(v, strings.Repeat("─", 12))

	for range time.NewTicker(time.Millisecond * 75).C {
		fgc := cl() + 1

		g.FgColor = gocui.Attribute(fgc)

		if _, err := g.View("fun"); err == nil {
			v.FgColor = gocui.Attribute(fgc)

			if s := fArt(); s != "" {
				v.Clear()

				fmt.Fprint(v, s)
			}
		}

		if v, err := g.View("ips"); err == nil {
			v.FgColor = gocui.Attribute(fgc)
		}

		if v, err := g.View("uptime"); err == nil {
			v.FgColor = gocui.Attribute(fgc)
		}
	}
}

func byteToSI(i float64) string {
	if i < 1024 {
		return fmt.Sprintf("%.2f B", i)
	}

	i /= 1024

	if i < 1024 {
		return fmt.Sprintf("%.2f KB", i)
	}

	i /= 1024

	if i < 1024 {
		return fmt.Sprintf("%.2f MB", i)
	}

	i /= 1024

	if i < 1024 {
		return fmt.Sprintf("%.2f GB", i)
	}

	i /= 1024

	return fmt.Sprintf("%.2f TB", i)
}
