package kube

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/merzzzl/warp/internal/log"
	"github.com/merzzzl/warp/internal/routes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

type Config struct {
	KubeConfigPath string
	KubeNamespace  string
}

type KubeRoute struct {
	ctx       context.Context
	cfg       *rest.Config
	cli       *kubernetes.Clientset
	namespace string
	domains   []string
	mutex     sync.Mutex
	usedIPs   map[string]net.IP
}

type k8sLog struct {
	event func() *log.Event
}

func (l *k8sLog) Write(bts []byte) (n int, err error) {
	l.event().Msg("K8S", strings.TrimSuffix(string(bts), "\n"))

	return len(bts), nil
}

func NewKubeRoute(ctx context.Context, cfg *Config) (*KubeRoute, error) {
	restcfg, err := clientcmd.BuildConfigFromFlags("", cfg.KubeConfigPath)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(restcfg)
	if err != nil {
		return nil, err
	}

	return &KubeRoute{
		ctx:       ctx,
		cfg:       restcfg,
		cli:       clientset,
		namespace: cfg.KubeNamespace,
		usedIPs:   make(map[string]net.IP),
	}, nil
}

func (k *KubeRoute) GetDNS(serviceName string) (net.IP, bool, error) {
	serviceName = strings.TrimSuffix(serviceName, fmt.Sprintf("%s.svc.cluster.local.", k.namespace))
	serviceName = strings.TrimSuffix(serviceName, ".")

	if !k.isKubeDomain(serviceName) {
		return nil, false, nil
	}

	hostIP, isNew, err := k.getFreeHost(serviceName)
	if err != nil {
		return nil, true, err
	}

	if !isNew {
		return hostIP, true, nil
	}

	service, err := k.cli.CoreV1().Services(k.namespace).Get(context.TODO(), serviceName, metav1.GetOptions{})
	if err != nil {
		return nil, true, err
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
		return nil, true, err
	}

	transport, upgrader, err := spdy.RoundTripperFor(k.cfg)
	if err != nil {
		return nil, true, err
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

	portForwarder, err := portforward.NewOnAddresses(dialer, []string{hostIP.String()}, ports, stopChan, readyChan, &k8sLog{event: log.Info}, &k8sLog{event: log.Error})
	if err != nil {
		return nil, true, err
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

	return hostIP, true, nil
}

func (k *KubeRoute) isKubeDomain(host string) bool {
	if k.domains == nil {
		k.mutex.Lock()
		defer k.mutex.Unlock()

		svcs, err := k.cli.CoreV1().Services(k.namespace).List(k.ctx, metav1.ListOptions{})
		if err != nil {
			log.Error().Err(err).Msg("K8S", "failed to get services")
		}

		k.domains = []string{}
		for _, svc := range svcs.Items {
			k.domains = append(k.domains, svc.Name)
		}
	}

	for _, d := range k.domains {
		if d == host {
			return true
		}
	}

	return false
}

func (k *KubeRoute) getFreeHost(name string) (net.IP, bool, error) {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	if ip, ok := k.usedIPs[name]; ok {
		return ip, false, nil
	}

	free, err := routes.GetFreeHost()
	if err != nil {
		return nil, true, err
	}

	k.usedIPs[name] = free

	return free, true, nil
}
