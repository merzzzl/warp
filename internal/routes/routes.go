package routes

import (
	"github.com/merzzzl/warp/internal/kube"
	"github.com/merzzzl/warp/internal/log"
	"github.com/merzzzl/warp/internal/tun"
)

type Routes struct {
	kube   *kube.KubeRoute
	tun    *tun.Tunnel
	routes []string
}

func NewRoutes(tun *tun.Tunnel, kube *kube.KubeRoute) *Routes {
	return &Routes{
		tun: tun,
		kube: kube,
	}
}

func (s *Routes) Add(ip string) {
	log.Info().Str("ip", ip).Msg("ROT", "add route")

	if err := s.tun.AddTunRoute(ip); err != nil {
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
	if s.kube == nil {
		return s.routes
	}

	routes := append(s.routes, s.kube.GetIPs()...)

	return routes
}
