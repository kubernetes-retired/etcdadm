package privateapi

import (
	"context"
	crypto_rand "crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"time"

	"github.com/golang/glog"
	grpccontext "golang.org/x/net/context"
)

type leadership struct {
	notification *LeaderNotificationRequest
	timestamp    time.Time
}

func (s *Server) LeaderNotification(ctx grpccontext.Context, request *LeaderNotificationRequest) (*LeaderNotificationResponse, error) {
	glog.Infof("Got LeaderNotification %s", request)

	if request.View == nil {
		return nil, fmt.Errorf("View is required")
	}
	if len(request.View.Healthy) == 0 {
		return nil, fmt.Errorf("View.Healthy is required")
	}

	s.addPeersFromView(request.View)

	s.mutex.Lock()
	defer s.mutex.Unlock()

	viewMap := make(map[PeerId]*PeerInfo)
	for _, p := range request.View.Healthy {
		peerId := PeerId(p.Id)
		viewMap[peerId] = p
	}

	reject := false
	for id, peer := range s.peers {
		leaderView := viewMap[id]
		if leaderView != nil {
			continue
		}
		_, healthy := peer.status(s.HealthyTimeout)
		if healthy {
			glog.Warningf("LeaderElection view did not include peer %s; will reject", id)
			reject = true
			break
		}
	}

	response := &LeaderNotificationResponse{}

	if reject {
		response.Accepted = false
		response.View = &View{}
		for _, peer := range s.peers {
			peerInfo, healthy := peer.status(s.HealthyTimeout)
			if healthy {
				response.View.Healthy = append(response.View.Healthy, peerInfo)
			}
		}
		return response, nil
	}

	response.Accepted = true
	s.leadership = &leadership{
		notification: request,
		timestamp:    time.Now(),
	}
	return response, nil
}

func (s *Server) snapshotHealthy() (map[PeerId]*peer, map[PeerId]*PeerInfo) {
	snapshot := make(map[PeerId]*peer)
	infos := make(map[PeerId]*PeerInfo)

	s.mutex.Lock()
	defer s.mutex.Unlock()
	for id, peer := range s.peers {
		info, healthy := peer.status(s.HealthyTimeout)
		if healthy {
			snapshot[id] = peer
			infos[id] = info
		}
	}
	return snapshot, infos
}

func (s *Server) addPeersFromView(view *View) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	for _, p := range view.Healthy {
		peerId := PeerId(p.Id)

		existing := s.peers[peerId]
		if existing == nil {
			glog.Infof("leader notification found new candidate peer: %s", peerId)
			existing = &peer{
				server: s,
				id:     peerId,
			}
			s.peers[peerId] = existing
			existing.updatePeerInfo(p)
			go existing.Run(s.context, s.PingInterval)
		}
	}
}

func (s *Server) IsLeader(leadershipToken string) bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// TODO: Immediately depose leader if we see a lower peer?

	if s.leadership == nil {
		glog.Infof("will reject leadership token %q: no leadership", leadershipToken)
		return false
	}
	if s.leadership.notification == nil {
		glog.Infof("will reject leadership token %q: no leadership notification", leadershipToken)
		return false
	}
	if s.leadership.notification.View == nil {
		glog.Infof("will reject leadership token %q: no leadership notification view", leadershipToken)
		return false
	}
	if s.leadership.notification.View.LeadershipToken != leadershipToken {
		glog.Infof("will reject leadership token %q: actual leadership is %q", leadershipToken, s.leadership.notification.View.LeadershipToken)
		return false
	} else {
		return true
	}
}

func (s *Server) BecomeLeader(ctx context.Context) ([]PeerId, string, error) {
	// TODO: Should we send a notification if we ourselves would reject it?
	snapshot, infos := s.snapshotHealthy()

	request := &LeaderNotificationRequest{}

	request.View = &View{}
	for _, info := range infos {
		request.View.Healthy = append(request.View.Healthy, info)
	}
	request.View.Leader = &s.myInfo
	request.View.LeadershipToken = randomToken()

	var acked []PeerId

	for peerId := range snapshot {
		conn, err := s.GetPeerClient(peerId)
		if err != nil {
			return nil, "", fmt.Errorf("error getting peer client for %s: %v", peerId, err)
		}

		peerClient := NewClusterServiceClient(conn)

		response, err := peerClient.LeaderNotification(ctx, request)
		if err != nil {
			return nil, "", fmt.Errorf("error sending leader notification to %s: %v", peerId, err)
		}

		if response.View != nil {
			s.addPeersFromView(response.View)
		}

		if !response.Accepted {
			return nil, "", fmt.Errorf("our leadership bid was not accepted by peer %q: %v", peerId, response)
		}
		acked = append(acked, peerId)
	}

	return acked, request.View.LeadershipToken, nil
}

func randomToken() string {
	b := make([]byte, 16, 16)
	_, err := io.ReadFull(crypto_rand.Reader, b)
	if err != nil {
		glog.Fatalf("error generating random token: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
