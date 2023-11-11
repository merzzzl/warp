package kube

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/merzzzl/warp/internal/log"
	"github.com/merzzzl/warp/internal/tun"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

type KubeRoute struct {
	ctx       context.Context
	cfg       *rest.Config
	cli       *kubernetes.Clientset
	namespace string
	domains   []string
	mutex     sync.Mutex
	usedIPs   map[string]net.IP
	ipList    map[string]struct{}
	localNet  net.IP
}

type k8sLog struct {
	event func() *log.Event
}

func (l *k8sLog) Write(bts []byte) (n int, err error) {
	l.event().Msg("K8S", strings.TrimSuffix(string(bts), "\n"))

	return len(bts), nil
}

func NewKubeRoute(cfg *rest.Config, cli *kubernetes.Clientset, namespace string, lo0 net.IP) *KubeRoute {
	return &KubeRoute{
		ctx:       context.Background(),
		cfg:       cfg,
		cli:       cli,
		namespace: namespace,
		domains:   make([]string, 0),
		usedIPs:   make(map[string]net.IP),
		ipList:    make(map[string]struct{}),
		localNet:  lo0,
	}
}

func (k *KubeRoute) SetContext(ctx context.Context) {
	k.ctx = ctx
}

func (k *KubeRoute) LoadDomains(ctx context.Context) error {
	svcs, err := k.cli.CoreV1().Services(k.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, svc := range svcs.Items {
		k.domains = append(k.domains, svc.Name+".")
		k.domains = append(k.domains, fmt.Sprintf("%s.%s.svc.cluster.local.", svc.Name, svc.Namespace))
	}

	log.Info().Str("namespace", k.namespace).Msgf("K8S", "load %d domain names", len(svcs.Items))

	return nil
}

func (k *KubeRoute) GetDNS(serviceName string) (net.IP, error) {
	serviceName = strings.TrimSuffix(serviceName, fmt.Sprintf("%s.svc.cluster.local.", k.namespace))
	serviceName = strings.TrimSuffix(serviceName, ".")

	service, err := k.cli.CoreV1().Services(k.namespace).Get(context.TODO(), serviceName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	var labelSelector string
	for key, value := range service.Spec.Selector {
		if labelSelector != "" {
			labelSelector += ","
		}
		labelSelector += key + "=" + value
	}

	pods, err := k.cli.CoreV1().Pods(k.namespace).List(context.Background(), metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return nil, err
	}

	transport, upgrader, err := spdy.RoundTripperFor(k.cfg)
	if err != nil {
		return nil, err
	}

	ports := []string{}
	for _, port := range service.Spec.Ports {
		ports = append(ports, fmt.Sprintf("%d:%d", port.Port, port.TargetPort.IntVal))
	}

	req := k.cli.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(k.namespace).
		Name(pods.Items[0].Name).
		SubResource("portforward")

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", req.URL())

	stopChan := make(chan struct{}, 1)
	readyChan := make(chan struct{}, 1)

	hostIP, isNew := k.getFreeHost(serviceName)
	if !isNew {
		return hostIP, nil
	}

	portForwarder, err := portforward.NewOnAddresses(dialer, []string{hostIP.String()}, ports, stopChan, readyChan, &k8sLog{event: log.Info}, &k8sLog{event: log.Error})
	if err != nil {
		return nil, err
	}

	if err := tun.AddLoAlias(hostIP.String()); err != nil {
		return nil, err
	}

	go func() {
		if err = portForwarder.ForwardPorts(); err != nil {
			log.Error().Err(err).Msg("K8S", "port forwarding is failed")
		}
	}()

	<-readyChan

	go func() {
		log.Info().Str("service", serviceName).Str("host", hostIP.String()).Msg("K8S", "port forwarding is ready")

		<-k.ctx.Done()

		close(stopChan)
		log.Info().Str("service", serviceName).Str("host", hostIP.String()).Msg("K8S", "port forwarding is stoped")
	}()

	return hostIP, nil
}

func (k *KubeRoute) IsKubeDomain(host string) bool {
	for _, d := range k.domains {
		if d == host {
			return true
		}
	}

	return false
}

func (k *KubeRoute) getFreeHost(name string) (net.IP, bool) {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	if ip, ok := k.usedIPs[name]; ok {
		return ip, false
	}

	for {
		ip := net.IPv4(k.localNet[12], k.localNet[13], k.localNet[14], byte(rand.Intn(255)))
		if _, ok := k.ipList[ip.String()]; !ok {
			k.ipList[ip.String()] = struct{}{}
			k.usedIPs[name] = ip

			return ip, true
		}
	}
}

func (k *KubeRoute) GetIPs() []string {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	var ips []string

	for ip, _ := range k.ipList {
		ips = append(ips, ip)
	}

	return ips
}
