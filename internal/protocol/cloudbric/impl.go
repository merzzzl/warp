package cloudbric

import (
	"context"
	"sort"
	"strconv"

	"github.com/merzzzl/warp/internal/protocol/wireguard"
	"github.com/merzzzl/warp/internal/utils/log"
)

type Config struct {
	DeviceID   string `yaml:"device_id"`
	PrivateKey string `yaml:"private_key"`
	Domain     string `yaml:"domain"`
}

type Protocol struct {
	*wireguard.Protocol
	server *serverInfo
	stat   *connectStatus
}

func New(ctx context.Context, cfg *Config) (*Protocol, error) {
	list, err := listServers(ctx)
	if err != nil {
		return nil, err
	}

	sort.Slice(list.Data, func(i, j int) bool {
		ci, _ := strconv.Atoi(list.Data[i].ClientCount)
		cj, _ := strconv.Atoi(list.Data[j].ClientCount)

		return ci > cj
	})

	var server *serverInfo

	for _, v := range list.Data {
		if v.Alive == "1" && v.Open == "1" {
			server = v

			break
		}
	}

	stat, err := activateConn(ctx, server, cfg.DeviceID, cfg.PrivateKey)
	if err != nil {
		return nil, err
	}

	go func() {
		<-ctx.Done()

		if _, err := deactiveConn(); err != nil {
			log.Error().Err(err).Msg("CLB", "deactivate failed")
		}
	}()

	wg, err := wireguard.New(ctx, &wireguard.Config{
		PrivateKey:    cfg.PrivateKey,
		PeerPublicKey: server.PublicKey,
		Endpoint:      server.PublicIP + ":" + server.Port,
		DNS:           []string{server.DNS},
		Domain:        cfg.Domain,
		Address:       stat.Data,
	})
	if err != nil {
		return nil, err
	}

	return &Protocol{
		server:   server,
		stat:     stat,
		Protocol: wg,
	}, nil
}
