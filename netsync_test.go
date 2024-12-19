package netsync

import (
	"context"
	"crypto/rand"
	"fmt"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type p2p struct {
	host           host.Host
	kdht           *dht.IpfsDHT
	netsyncService *NetSyncService
}

func spawnHost(ctx context.Context) (*p2p, error) {
	r := rand.Reader

	prvKey, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, r)
	if err != nil {
		panic(err)
	}

	sourceMultiAddr, _ := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/%s/tcp/%d", "0.0.0.0", 0))

	host, err := libp2p.New(
		libp2p.ListenAddrs(sourceMultiAddr),
		libp2p.Identity(prvKey),
	)
	if err != nil {
		return nil, err
	}

	kdht, err := dht.New(ctx, host, dht.Mode(dht.ModeServer))
	if err != nil {
		return nil, err
	}

	netsyncservice := NewNetSyncService(ctx, kdht)
	netsyncservice.Start()

	return &p2p{
		host:           host,
		kdht:           kdht,
		netsyncService: netsyncservice,
	}, nil

}

func spawnHosts(ctx context.Context, count int) ([]*p2p, error) {
	var hosts []*p2p
	for i := 0; i < count; i++ {
		host, err := spawnHost(ctx)
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

func (tn *TestNet) TryLockAquire(ctx context.Context) int64 {
	aquiredLks := atomic.Int64{}
	wg := sync.WaitGroup{}

	sctx, cancel := context.WithDeadline(ctx, time.Now().Add(time.Second*10))
	defer cancel()
	for _, p := range tn.Hosts {
		wg.Add(1)
		go func(p *p2p) {
			defer wg.Done()
			select {
			case <-sctx.Done():
			}

			lkCtx, cancel := context.WithTimeout(ctx, time.Minute)
			defer cancel()
			lk, _ := p.netsyncService.NewLock(lkCtx, "test")
			if lk.TryLock() {
				fmt.Println("Lock acquired for : ", p.host.ID().String())
				aquiredLks.Add(1)
			}
		}(p)
	}
	wg.Wait()
	return aquiredLks.Load()
}

func TestNetSyncService(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	bootstrapNode, err := spawnHost(ctx)
	if err != nil {
		t.Fatal(err)
	}

	hosts, err := spawnHosts(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}

	testNet := &TestNet{
		Hosts:         hosts,
		BootstrapNode: bootstrapNode,
	}

	testNet.Bootstrap()

	time.Sleep(time.Second * 5)

	t1 := time.Now()
	defer func() {
		fmt.Printf("Time taken : %v\n", time.Since(t1))
	}()
	if count := testNet.TryLockAquire(ctx); count != 1 {
		t.Fatal("Failed to aquire lock", count)
	}

}
