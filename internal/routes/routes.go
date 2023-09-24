package routes

import (
	"github.com/merzzzl/warp/internal/log"
	"github.com/merzzzl/warp/internal/tun"
)

type Routes struct {
	tun    *tun.Tunnel
	routes []string
}

func NewRoutes(tun *tun.Tunnel) *Routes {
	return &Routes{
		tun: tun,
	}
}

func (s *Routes) Add(ip string) {
	log.Info().Str("ip", ip).Msg("ROT", "add route")

	if err := s.tun.AddTunRoute(ip + "/32"); err != nil {
		log.Error().Err(err).Str("ip", ip).Msg("ROT", "fail on create route")
		return
	}

	routes, err := s.tun.GetTunRoutes()
	if err != nil {
		log.Error().Err(err).Str("ip", ip).Msg("ROT", "fail on update routes")
		return
	}

	s.routes = routes
}

func (s *Routes) GetAll() []string {
	return s.routes
}
