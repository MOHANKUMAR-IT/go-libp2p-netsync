package netsync

import (
	"context"
	"fmt"
	"github.com/MOHANKUMAR-IT/go-libp2p-netsync/pb"
	logging "github.com/ipfs/go-log/v2"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-msgio/protoio"
	"math"
	"sort"
	"sync"
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
	host        host.Host
	ctx         context.Context
	cancel      context.CancelFunc
	kdht        *dht.IpfsDHT
	ServiceLock sync.Map
}

func NewNetSyncService(ctx context.Context, host host.Host, kdht *dht.IpfsDHT) *NetSyncService {
	sctx, cancel := context.WithCancel(ctx)
	return &NetSyncService{
		ctx:         sctx,
		cancel:      cancel,
		host:        host,
		kdht:        kdht,
		ServiceLock: sync.Map{},
	}
}

func (ns *NetSyncService) Start() {
	ns.host.SetStreamHandler(NetSyncProtocolID_v10, ns.HandleStream)

	go func() {
		ticker := time.NewTicker(refreshTime)
		defer ticker.Stop()

		for {
			select {
			case <-ns.ctx.Done():
				return
			case <-ticker.C:
				ns.ServiceLock.Range(func(key, value interface{}) bool {
					expiryTime, ok := value.(time.Time)
					if !ok || time.Now().After(expiryTime) {
						ns.ServiceLock.Delete(key)
					}
					return true
				})
			}
		}
	}()
}

func (ns *NetSyncService) Close() {
	ns.cancel()
	ns.host.RemoveStreamHandler(NetSyncProtocolID_v10)
}

func (ns *NetSyncService) HandleStream(stream network.Stream) {
	defer stream.Close()

	reader := protoio.NewDelimitedReader(stream, 1<<20) // Max message size: 1 MB
	writer := protoio.NewDelimitedWriter(stream)

	var req pb.ControlMessage
	if err := reader.ReadMsg(&req); err != nil {
		logger.Errorf("failed to read Protobuf message from stream: %v", err)
		return
	}

	respLockState := ns.HandleLockRequest(&req)

	resp := &pb.ControlMessage{
		Key:       req.Key,
		Deadline:  req.Deadline,
		LockState: respLockState,
	}

	if err := writer.WriteMsg(resp); err != nil {
		logger.Errorf("failed to write Protobuf message to stream: %v", err)
		return
	}

	logger.Infof("Successfully processed lock request for key: %s", req.Key)
}

func (ns *NetSyncService) HandleLockRequest(msg *pb.ControlMessage) pb.LockState {
	switch msg.LockState {
	case pb.LockState_LOCK_TRY_ACQUIRE:
		return ns.handleTryAcquireLock(msg)
	case pb.LockState_LOCK_TRY_RELEASE:
		return ns.handleTryReleaseLock(msg)
	default:
		return pb.LockState_LOCK_INVALID
	}
}

func (ns *NetSyncService) handleTryAcquireLock(msg *pb.ControlMessage) pb.LockState {
	if _, ok := ns.ServiceLock.Load(msg.Key); ok {
		return pb.LockState_LOCK_ACQUIRE_FAILED
	}
	deadline := time.Duration(msg.Deadline)
	if deadline > maxLockTime {
		return pb.LockState_LOCK_ACQUIRE_FAILED
	}
	expiryTime := time.Now().Add(deadline)
	ns.ServiceLock.Store(msg.Key, expiryTime)
	return pb.LockState_LOCK_ACQUIRED
}

func (ns *NetSyncService) handleTryReleaseLock(msg *pb.ControlMessage) pb.LockState {
	if deadline, ok := ns.ServiceLock.Load(msg.Key); !ok {
		return pb.LockState_LOCK_RELEASED
	} else {
		if time.Now().Before(deadline.(time.Time)) {
			return pb.LockState_LOCK_RELEASE_FAILED
		}
	}
	ns.ServiceLock.Delete(msg.Key)
	return pb.LockState_LOCK_RELEASED
}

type Locker interface {
	TryLock() bool
	Unlock()
}

type Mutex struct {
	cid      string
	ctx      context.Context
	service  *NetSyncService
	acquired bool
}

func (ns *Mutex) TryLock() bool {
	return ns.service.acquireNetworkLock(ns.ctx, ns.cid)
}

func (ns *Mutex) Unlock() {
	ns.service.releaseNetworkLock(ns.ctx, ns.cid)
}

func NewLock(ctx context.Context, service *NetSyncService, key string) *Mutex {
	return &Mutex{
		ctx:     ctx,
		cid:     key,
		service: service,
	}
}

func (ns *NetSyncService) acquireNetworkLock(ctx context.Context, key string) bool {
	rmsg, err := createLockRequestMsg(ctx, key, pb.LockState_LOCK_TRY_ACQUIRE)
	if err != nil {
		logger.Errorf("failed to create lock request message: %v", err)
		return false
	}

	closestPeers, err := ns.getSortedClosestPeers(ctx, key)
	if err != nil {
		logger.Errorf("failed to get closest peers: %v", err)
		return false
	}

	return ns.processLockRequest(ctx, closestPeers, rmsg, pb.LockState_LOCK_ACQUIRED)
}

func (ns *NetSyncService) releaseNetworkLock(ctx context.Context, key string) bool {
	rmsg, err := createLockRequestMsg(ctx, key, pb.LockState_LOCK_TRY_RELEASE)
	if err != nil {
		logger.Errorf("failed to create lock request message: %v", err)
		return false
	}

	closestPeers, err := ns.getSortedClosestPeers(ctx, key)
	if err != nil {
		logger.Errorf("failed to get closest peers: %v", err)
		return false
	}

	return ns.processLockRequest(ctx, closestPeers, rmsg, pb.LockState_LOCK_RELEASED)
}

func createLockRequestMsg(ctx context.Context, key string, lockState pb.LockState) (*pb.ControlMessage, error) {
	deadline, ok := ctx.Deadline()
	if !ok {
		return nil, ErrInvalidContext
	}
	duration := time.Until(deadline)
	if duration <= 0 {
		return nil, ErrInvalidContext
	}

	msg := &pb.ControlMessage{
		Key:       key,
		Deadline:  duration.Nanoseconds(),
		LockState: lockState,
	}

	return msg, nil
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

func (ns *NetSyncService) processLockRequest(ctx context.Context, closestPeers []peer.ID, rmsg *pb.ControlMessage, expectedState pb.LockState) bool {
	for _, pid := range closestPeers {
		response, err := ns.contactPeer(ctx, pid, rmsg)
		if err != nil {
			logger.Warnf("error contacting peer %s: %v", pid, err)
			continue
		}
		if response.LockState == expectedState {
			return true
		}
		return false
	}
	return false
}

func (ns *NetSyncService) contactPeer(ctx_g context.Context, pid peer.ID, msg *pb.ControlMessage) (*pb.ControlMessage, error) {
	ctx, cancel := context.WithTimeout(ctx_g, 5*time.Second)
	defer cancel()

	pad, err := ns.kdht.FindPeer(ctx, pid)
	if err != nil {
		return nil, err
	}
	if err = ns.host.Connect(ctx, pad); err != nil {
		return nil, err
	}

	peerStream, err := ns.host.NewStream(ctx, pid, NetSyncProtocolID_v10)
	if err != nil {
		return nil, err
	}
	defer peerStream.Close()

	peerStream.SetReadDeadline(time.Now().Add(5 * time.Second))

	writer := protoio.NewDelimitedWriter(peerStream)
	if err := writer.WriteMsg(msg); err != nil {
		return nil, err
	}

	reader := protoio.NewDelimitedReader(peerStream, 1<<20) // Max size: 1 MB
	var resp pb.ControlMessage
	if err := reader.ReadMsg(&resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

func calculateXORDistance(a, b string) uint64 {
	minLen := int(math.Min(float64(len(a)), float64(len(b))))
	var result uint64
	for i := 0; i < minLen; i++ {
		result = (result << 1) | (uint64(a[i]^b[i]) & 1)
	}
	return result
}
