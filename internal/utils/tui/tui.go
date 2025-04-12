package tui

import (
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"time"

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

	if v, err := g.SetView("logs", 0, 0, maxX-21, maxY-16); err != nil {
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

	if v, err := g.SetView("pipes", 0, maxY-15, maxX-21, maxY-1); err != nil {
		if !errors.Is(err, gocui.ErrUnknownView) {
			return err
		}

		v.Title = "Pipes"

		go func() {
			for range time.NewTicker(time.Millisecond * 250).C {
				g.Update(func(*gocui.Gui) error {
					v.Clear()

					list := network.List()

					sort.Slice(list, func(i, j int) bool {
						return list[i].OpenCount > list[j].OpenCount
					})

					for _, pipe := range list {
						fmt.Fprintf(v, "%s %s %s %s %s %s %s %s\n",
							log.Colorize(time.Unix(0, 0).UTC().Add(time.Since(pipe.OpenAt)).Format("15:04:05"), 7),
							fmt.Sprintf("%.3d", pipe.OpenCount),
							log.Colorize(strings.Repeat("»", 3), 6),
							log.Colorize(strings.ToUpper(pipe.Tag), 11),
							log.Colorize(strings.Repeat("»", 3), 6),
							log.Colorize(strings.ToUpper(pipe.Dest.Network()), 11),
							pipe.Dest.String(),
							log.Colorize(pipe.Protocol, 6),
						)
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

					fmt.Fprintf(v, "In:  %s/s\n", byteToSI(in))
					fmt.Fprintf(v, "     %s\n", log.Colorize(byteToSI(sin), 7))
					fmt.Fprintf(v, "Out: %s/s\n", byteToSI(out))
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
		loader := []rune("        ")
		transf := 0.0
		prevTotal := 0.0
		bar := func() string {
			rx, tx := traffic.GetTransferred()
			total := rx + tx
			transf = total - prevTotal
			prevTotal = total

			var newChar rune
			switch {
			case transf < 1:
				newChar = ' '
			case transf < 1024:
				if loader[7] == '⢀' {
					newChar = '⡀'
				} else {
					newChar = '⢀'
				}
			case transf < 1024*32:
				newChar = '⣀'
			case transf < 1024*64:
				if loader[7] == '⣠' {
					newChar = '⣄'
				} else {
					newChar = '⣠'
				}
			case transf < 1024*128:
				newChar = '⣤'
			case transf < 1024*256:
				if loader[7] == '⣴' {
					newChar = '⣦'
				} else {
					newChar = '⣴'
				}
			case transf < 1024*512:
				newChar = '⣶'
			case transf < 1024*1024:
				if loader[7] == '⣾' {
					newChar = '⣷'
				} else {
					newChar = '⣾'
				}
			default:
				newChar = '⣿'
			}

			// Сдвигаем символы влево и добавляем новый в конец
			loader = append(loader[1:], newChar)
			return string(loader)
		}

		go func() {
			for range time.NewTicker(time.Millisecond * 1000).C {
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
	ts := []string{
		"⊂(◉‿◉)つ──",
		"( ✜︵ ✜ )─",
		"ʕっ •ᴥ•ʔっ─",
		"(｡◕‿‿◕｡)─",
		"(っ ´ω`c)♡",
		"(ʘ‿ʘ)╯────",
	}
	art := ts[rand.Intn(6)]

	return fmt.Sprintf("─%s─", art)
}

func fun(g *gocui.Gui) {
	cl := fColor()

	v, err := g.SetView("fun", 0, -1, 11, 1)
	if !errors.Is(err, gocui.ErrUnknownView) {
		return
	}

	v.Frame = false

	_, _ = fmt.Fprint(v, fArt())

	for range time.NewTicker(time.Millisecond * 75).C {
		fgc := cl() + 1

		g.FgColor = gocui.Attribute(fgc)

		if _, err := g.View("fun"); err == nil {
			v.FgColor = gocui.Attribute(fgc)
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
