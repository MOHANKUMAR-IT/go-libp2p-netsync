package netsync

import (
	"context"
	"fmt"
	"github.com/MOHANKUMAR-IT/go-libp2p-netsync/pb"
	logging "github.com/ipfs/go-log/v2"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

const (
	NetSyncProtocolID_v10 = "/netsync/1.0.0"
)

var (
	logger = logging.Logger("netsync")

	maxLockTime = time.Minute * 10
	refreshTime = time.Minute * 3
)

var (
	ErrInvalidContext = fmt.Errorf("deadline not set or expired")
)

type NetSyncService struct {
	pb.UnimplementedNetSyncServiceServer

	host        host.Host
	ctx         context.Context
	cancel      context.CancelFunc
	kdht        *dht.IpfsDHT
	serviceLock sync.Map

	grpcServer *Server
}

func NewNetSyncService(ctx context.Context, kdht *dht.IpfsDHT) *NetSyncService {
	sctx, cancel := context.WithCancel(ctx)

	return &NetSyncService{
		ctx:         sctx,
		cancel:      cancel,
		host:        kdht.Host(),
		kdht:        kdht,
		serviceLock: sync.Map{},
		grpcServer:  NewGrpcServer(sctx, kdht.Host()),
	}
}

func (ns *NetSyncService) Start() error {

	pb.RegisterNetSyncServiceServer(ns.grpcServer, ns)

	go func() {
		err := ns.grpcServer.Serve()
		if err != nil {
			logger.Errorf("failed to start grpc server: %v", err)
			ns.Close()
		}
	}()

	go func() {
		ticker := time.NewTicker(refreshTime)
		defer ticker.Stop()

		for {
			select {
			case <-ns.ctx.Done():
				return
			case <-ticker.C:
				ns.serviceLock.Range(func(key, value interface{}) bool {
					record := value.(syncRecord)
					if time.Now().After(record.expiryTime) {
						ns.serviceLock.Delete(key)
					}
					return true
				})
			}
		}
	}()
	return nil
}

func (ns *NetSyncService) Close() {
	ns.cancel()
	ns.host.RemoveStreamHandler(NetSyncProtocolID_v10)
}

func (ns *NetSyncService) ServiceRequest(ctx context.Context, req *pb.NetSyncMessage) (*pb.NetSyncMessage, error) {

	respLockState := ns.handleLockRequest(req)

	req.LockState = respLockState

	return req, nil
}

func (ns *NetSyncService) handleLockRequest(msg *pb.NetSyncMessage) pb.LockState {
	switch msg.LockState {
	case pb.LockState_LOCK_TRY_ACQUIRE, pb.LockState_LOCK_TRY_RS_ACQUIRE:
		return ns.handleTryAcquireLock(msg)
	case pb.LockState_LOCK_TRY_RELEASE:
		return ns.handleTryReleaseLock(msg)
	default:
		return pb.LockState_LOCK_INVALID
	}
}

type syncRecord struct {
	expiryTime time.Time
	peerID     string
}

func (ns *NetSyncService) handleTryAcquireLock(msg *pb.NetSyncMessage) pb.LockState {
	if holdedBy, ok := ns.serviceLock.Load(msg.Key); ok && holdedBy.(syncRecord).peerID != msg.Peerid {
		return pb.LockState_LOCK_ACQUIRE_FAILED
	}

	expiryTime := time.Now().Add(time.Minute)

	ns.serviceLock.Store(msg.Key, syncRecord{
		expiryTime: expiryTime,
		peerID:     msg.Peerid,
	})

	if msg.LockState == pb.LockState_LOCK_TRY_ACQUIRE {
		rctx, cancel := context.WithTimeout(ns.ctx, time.Second*10)
		defer cancel()
		if !ns.resolveConflict(rctx, msg) {
			ns.serviceLock.Delete(msg.Key)
			return pb.LockState_LOCK_ACQUIRE_FAILED
		}
	}
	return pb.LockState_LOCK_ACQUIRED
}

func (ns *NetSyncService) handleTryReleaseLock(msg *pb.NetSyncMessage) pb.LockState {
	if deadline, ok := ns.serviceLock.Load(msg.Key); !ok {
		return pb.LockState_LOCK_RELEASED
	} else {
		if time.Now().Before(deadline.(syncRecord).expiryTime) {
			return pb.LockState_LOCK_RELEASE_FAILED
		}
	}
	ns.serviceLock.Delete(msg.Key)
	return pb.LockState_LOCK_RELEASED
}

type Locker interface {
	TryLock() bool
	Unlock()
	IsAcquired() bool
}

type Mutex struct {
	cid      string
	ctx      context.Context
	cancel   context.CancelFunc
	service  *NetSyncService
	acquired atomic.Bool
}

func (mtx *Mutex) IsAcquired() bool {
	return mtx.acquired.Load()
}

func (mtx *Mutex) TryLock() bool {
	if mtx.service == nil {
		return false
	}
	return mtx.service.acquireNetworkLock(mtx)
}

func (mtx *Mutex) Unlock() {
	mtx.cancel()
	//mtx.ctx, mtx.cancel = context.WithTimeout(mtx.service.ctx, time.Second*30)
	//defer mtx.cancel()
	//mtx.service.releaseNetworkLock(mtx)
}

func (ns *NetSyncService) NewLock(ctx context.Context, key string) (*Mutex, error) {
	if key == "" {
		return nil, fmt.Errorf("key cannot be empty")
	}
	ctx, cancel := context.WithCancel(ctx)
	return &Mutex{
		ctx:     ctx,
		cancel:  cancel,
		cid:     key,
		service: ns,
	}, nil
}

func (ns *NetSyncService) acquireNetworkLock(mtx *Mutex) bool {

	rmsg := &pb.NetSyncMessage{
		Key:       mtx.cid,
		Peerid:    ns.host.ID().String(),
		LockState: pb.LockState_LOCK_TRY_ACQUIRE,
	}

	lock := func() bool {
		closestPeers, err := ns.getSortedClosestPeers(mtx.ctx, mtx.cid)
		if err != nil {
			return false
		}

		return ns.processLockRequest(mtx.ctx, closestPeers, rmsg, pb.LockState_LOCK_ACQUIRED)
	}

	firstLockStatus := lock()

	if firstLockStatus {
		go func() {
			for {
				select {
				case <-mtx.ctx.Done():
					return
				case <-time.After(time.Second * 50):
					if !lock() {
						mtx.acquired.Store(false)
						return
					}
				}
			}
		}()
	}

	return firstLockStatus

}

func (ns *NetSyncService) releaseNetworkLock(mtx *Mutex) bool {
	rmsg := &pb.NetSyncMessage{
		Key:       mtx.cid,
		Peerid:    ns.host.ID().String(),
		LockState: pb.LockState_LOCK_TRY_RELEASE,
	}

	closestPeers, err := ns.getSortedClosestPeers(mtx.ctx, mtx.cid)
	if err != nil {
		return false
	}

	return ns.processLockRequest(mtx.ctx, closestPeers, rmsg, pb.LockState_LOCK_RELEASED)
}

func (ns *NetSyncService) getSortedClosestPeers(ctx context.Context, key string) ([]peer.ID, error) {
	closestPeers, err := ns.kdht.GetClosestPeers(ctx, key)
	if err != nil {
		return nil, err
	}

	sort.Slice(closestPeers, func(i, j int) bool {
		return calculateXORDistance(closestPeers[i].String(), key) <
			calculateXORDistance(closestPeers[j].String(), key)
	})

	return closestPeers, nil
}

func (ns *NetSyncService) processLockRequest(ctx context.Context, closestPeers []peer.ID, rmsg *pb.NetSyncMessage, expectedState pb.LockState) bool {
	for _, pid := range closestPeers {
		sctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		response, err := ns.contactPeer(sctx, pid, rmsg)
		if err != nil {
			cancel()
			continue
		}
		cancel()

		if response.LockState == expectedState {
			return true
		}
		return false
	}
	return false
}

func (ns *NetSyncService) contactPeer(ctx context.Context, pid peer.ID, msg *pb.NetSyncMessage) (*pb.NetSyncMessage, error) {
	ctx_f, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	pad, err := ns.kdht.FindPeer(ctx_f, pid)
	if err != nil {
		return nil, fmt.Errorf("failed to find peer: %v", err)
	}
	if err = ns.host.Connect(ctx_f, pad); err != nil {
		return nil, fmt.Errorf("failed to connect to peer: %v", err)
	}

	grpcClient := NewClient(ns.host, NetSyncProtocolID_v10, WithServer(ns.grpcServer))

	conn, err := grpcClient.Dial(ctx, pid, grpc.WithTransportCredentials(insecure.NewCredentials()))

	if err != nil {
		return nil, fmt.Errorf("failed to create grpc client: %v", err)
	}

	resp, err := pb.NewNetSyncServiceClient(conn).ServiceRequest(ctx, msg)
	if err != nil {
		return nil, fmt.Errorf("failed to handle message: %v", err)
	}

	return resp, nil
}

func calculateXORDistance(a, b string) uint64 {
	minLen := int(math.Min(float64(len(a)), float64(len(b))))
	var result uint64
	for i := 0; i < minLen; i++ {
		result = (result << 1) | (uint64(a[i]^b[i]) & 1)
	}
	return result
}
