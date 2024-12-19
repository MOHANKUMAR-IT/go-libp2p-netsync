package main

import (
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"github.com/MOHANKUMAR-IT/go-libp2p-netsync"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	"github.com/multiformats/go-multiaddr"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"
)

type p2p struct {
	host           host.Host
	kdht           *dht.IpfsDHT
	topic          *pubsub.Topic
	netsyncService *netsync.NetSyncService
}

func spawnHost(ctx context.Context, bootnode bool) (*p2p, error) {
	r := rand.Reader

	prvKey, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, r)
	if err != nil {
		panic(err)
	}
	port := 0
	if bootnode {
		port = 6061
	}
	sourceMultiAddr, _ := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/%s/tcp/%d", "0.0.0.0", port))

	option := []libp2p.Option{
		libp2p.ListenAddrs(sourceMultiAddr),
		libp2p.Identity(prvKey),
	}

	if bootnode {
		newConMan, err := connmgr.NewConnManager(
			1000,
			2000,
		)
		if err != nil {
			log.Fatal(err)
		}
		option = append(option, libp2p.ConnectionManager(newConMan))
	}
	host, err := libp2p.New(option...)
	if err != nil {
		return nil, err
	}

	kdht, err := dht.New(ctx, host, dht.Mode(dht.ModeServer))
	if err != nil {
		return nil, err
	}

	netsyncservice := netsync.NewNetSyncService(ctx, kdht)
	netsyncservice.Start()

	pubsub, err := pubsub.NewGossipSub(ctx, host, pubsub.WithPeerExchange(true))
	if err != nil {
		log.Fatal(err)
	}
	topic, err := pubsub.Join("test")

	return &p2p{
		host:           host,
		kdht:           kdht,
		topic:          topic,
		netsyncService: netsyncservice,
	}, nil

}

func spawnHosts(ctx context.Context, count int) ([]*p2p, error) {
	var hosts []*p2p
	for i := 0; i < count; i++ {
		host, err := spawnHost(ctx, false)
		if err != nil {
			return nil, err
		}
		hosts = append(hosts, host)
	}
	return hosts, nil
}

type TestNet struct {
	Hosts         []*p2p
	BootstrapNode *p2p
}

func (tn *TestNet) Close() {
	for _, host := range tn.Hosts {
		host.host.Close()
	}
	tn.BootstrapNode.host.Close()
}

func (tn *TestNet) Bootstrap() {
	bootstrapAddr := peer.AddrInfo{Addrs: tn.BootstrapNode.host.Addrs(), ID: tn.BootstrapNode.host.ID()}

	for _, host := range tn.Hosts {
		connectCtx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		host.host.Connect(connectCtx, bootstrapAddr)
		cancel()
	}

	for _, host := range tn.Hosts {
		connectCtx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		host.kdht.Bootstrap(connectCtx)
		cancel()
	}

}

func Bootstrap(nodes []*p2p, bootstrapAddr peer.AddrInfo) {
	for _, host := range nodes {
		connectCtx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		host.host.Connect(connectCtx, bootstrapAddr)
		cancel()
	}

	for _, host := range nodes {
		connectCtx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		host.kdht.Bootstrap(connectCtx)
		cancel()
	}
}

func (tn *TestNet) TryLockAquire(ctx context.Context) {
	aquiredLks := atomic.Int64{}

	for _, p := range tn.Hosts {

		go func(p *p2p) {
			sub, err := p.topic.Subscribe()
			if err != nil {
				log.Fatal(err)
			}
			for {
				key, err := sub.Next(context.Background())
				if err != nil {
					log.Fatal(err)
				}
				go func(msg string) {
					lkCtx, cancel := context.WithTimeout(ctx, time.Minute)
					defer cancel()
					if msg == "" {
						return
					}
					lk, _ := p.netsyncService.NewLock(lkCtx, msg)
					if lk.TryLock() {
						client := http.DefaultClient
						get, err := client.Get("http://192.168.0.106:6060/hit?key=" + msg)
						if err != nil {
							log.Fatal(err)
						}
						defer get.Body.Close()
						fmt.Println("Lock acquired for : ", msg, " by ", p.host.ID().String())
						aquiredLks.Add(1)
					}
				}(string((key.Data)))
			}
		}(p)
	}

	fmt.Println("Created Nodes")

}

func main() {

	bootonly := flag.Bool("bootstraponly", false, "Run node in bootstrap mode")

	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	gracefulShutdown := make(chan os.Signal, 1)
	signal.Notify(gracefulShutdown, syscall.SIGINT, syscall.SIGTERM)

	if *bootonly {
		fmt.Printf("Running in bootstrap mode\n")
		s := Server{}
		s.Start(ctx)
		<-gracefulShutdown
	} else {

		bootnode := getBootstrapAddr()

		if bootnode == "" {
			log.Fatal("Bootnode address is required")
		}

		bootpad, err := peer.AddrInfoFromString(bootnode)
		if err != nil {
			log.Fatal(err)
		}

		hosts, err := spawnHosts(ctx, 100)
		if err != nil {
			log.Fatal(err)
		}

		Bootstrap(hosts, *bootpad)

		time.Sleep(time.Second * 5)

		t1 := time.Now()
		defer func() {
			fmt.Printf("Time taken : %v\n", time.Since(t1))
		}()
		testNet := TestNet{
			Hosts: hosts,
		}
		testNet.TryLockAquire(ctx)

		<-gracefulShutdown
	}
}

func getBootstrapAddr() string {
	resp, err := http.Get("http://127.0.0.1:6060/bootstrapaddr")
	if err != nil {
		log.Fatal("Failed to fetch bootstrap address:", err)
	}
	defer resp.Body.Close()

	bootnodebuf, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal("Failed to read response body:", err)
	}

	return string(bootnodebuf)
}
