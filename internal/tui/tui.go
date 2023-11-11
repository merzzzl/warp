package tui

import (
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/jroimartin/gocui"
	"github.com/merzzzl/warp/internal/log"
	"github.com/merzzzl/warp/internal/routes"
	"github.com/merzzzl/warp/internal/tarification"
)

type TUI struct {
	logs   <-chan string
	meter  *tarification.DataMeter
	routes *routes.Routes
	config *Config
}

type Config struct {
	SSH    string
	Tunnel string
	IP     string
	Domain string
	K8S    string
}

type LogWriter struct{}

var logChannel = make(chan string, 100)

func (cw LogWriter) Write(p []byte) (n int, err error) {
	logChannel <- string(p)
	return len(p), nil
}

func NewTUI(meter *tarification.DataMeter, routes *routes.Routes, config *Config) *TUI {
	return &TUI{
		logs:   logChannel,
		meter:  meter,
		routes: routes,
		config: config,
	}
}

func (t *TUI) CreateTUI() error {
	g, err := gocui.NewGui(gocui.OutputNormal)
	if err != nil {
		return err
	}
	defer g.Close()

	g.SetManagerFunc(t.layout)

	if err := t.keybindings(g); err != nil {
		return err
	}

	log.SetOutput(LogWriter{})
	defer log.SetOutput(os.Stdout)

	if err := g.MainLoop(); err != nil && err != gocui.ErrQuit {
		return err
	}

	return nil
}

func (t *TUI) layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()

	if v, err := g.SetView("config", 0, maxY-7, maxX-21, maxY-1); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = "Config"

		g.Update(func(g *gocui.Gui) error {
			fmt.Fprintf(v, "SSH:     %s\n", t.config.SSH)
			fmt.Fprintf(v, "K8S:     %s\n", t.config.K8S)
			fmt.Fprintf(v, "Tunnel:  %s\n", t.config.Tunnel)
			fmt.Fprintf(v, "Gateway: %s\n", t.config.IP)
			fmt.Fprintf(v, "Domain:  %s\n", t.config.Domain)

			return nil
		})
	}

	if v, err := g.SetView("logs", 0, 0, maxX-21, maxY-8); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = "Logs"

		go func() {
			for logMsg := range t.logs {
				g.Update(func(g *gocui.Gui) error {
					fmt.Fprint(v, logMsg)

					lines := len(v.BufferLines()) - 1
					_, vy := v.Size()
					if lines > vy {
						ox, _ := v.Origin()
						v.SetOrigin(ox, lines-vy)
					}

					return nil
				})
			}
		}()
	}

	if v, err := g.SetView("stats", maxX-20, 0, maxX-1, 3); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = "Bandwidth"

		go func() {
			for range time.NewTicker(time.Second * 1).C {
				g.Update(func(g *gocui.Gui) error {
					v.Clear()
					in, out := t.meter.GetRates()

					fmt.Fprintf(v, "In:  %.2f\n", in/1024)
					fmt.Fprintf(v, "Out: %.2f\n", out/1024)

					return nil
				})
			}
		}()
	}

	if v, err := g.SetView("ips", maxX-20, 4, maxX-1, maxY-1); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = "IP List"

		go func() {
			for range time.NewTicker(time.Second * 1).C {
				g.Update(func(g *gocui.Gui) error {
					v.Clear()
					ips := t.routes.GetAll()
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

func (t *TUI) keybindings(g *gocui.Gui) error {
	if err := g.SetKeybinding("", 'q', gocui.ModNone, t.quit); err != nil {
		return err
	}
	return nil
}

func (t *TUI) quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}
