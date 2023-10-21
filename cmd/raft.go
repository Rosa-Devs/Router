package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"testing"
	"time"

	raftlib "github.com/Rosa-Devs/Router/modules/raft"
	raft "github.com/hashicorp/raft"
	"github.com/libp2p/go-libp2p"
	consensus "github.com/libp2p/go-libp2p-consensus"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peerstore"
)

var raftTmpFolder = "testing_tmp"
var datastore = "testing_datastore"
var raftQuiet = true

// Define a struct for key-value pairs
// type KeyValue struct {
// 	Id   string `json:"Id"`
// 	Data string `json:"Data"`
// }

type KeyValue struct {
	Data map[string]string `json:"Data"`
}

type RaftState struct {
	KeyValueMap map[string]string
}

// func (o testOperation) ApplyTo(s consensus.State) (consensus.State, error) {
// 	raftSt := s.(*RaftState)

// 	// Check if the key already exists in the state
// 	if existingData, ok := raftSt.KeyValueMap[o.Id]; ok {
// 		// If the key exists, append the new data to the existing data
// 		raftSt.KeyValueMap[o.Id] = existingData + o.Data
// 	} else {
// 		// If the key does not exist, create a new entry with the provided data
// 		raftSt.KeyValueMap[o.Id] = o.Data
// 	}

// 	return raftSt, nil
// }

func (o KeyValue) ApplyTo(s consensus.State) (consensus.State, error) {
	raftSt := s.(*RaftState)

	// Update the KeyValueMap with the new data
	for key, value := range o.Data {
		raftSt.KeyValueMap[key] = value
	}

	return &raftSt, nil
}

// wait 10 seconds for a leader.
func waitForLeader(t *testing.T, r *raft.Raft) {
	obsCh := make(chan raft.Observation, 1)
	observer := raft.NewObserver(obsCh, false, nil)
	r.RegisterObserver(observer)
	defer r.DeregisterObserver(observer)

	// New Raft does not allow leader observation directy
	// What's worse, there will be no notification that a new
	// leader was elected because observations are set before
	// setting the Leader and only when the RaftState has changed.
	// Therefore, we need a ticker.

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ticker := time.NewTicker(time.Second / 2)
	defer ticker.Stop()
	for {
		select {
		case obs := <-obsCh:
			switch obs.Data.(type) {
			case raft.RaftState:
				if leaderAddr, _ := r.LeaderWithID(); leaderAddr != "" {
					return
				}
			}
		case <-ticker.C:
			if leaderAddr, _ := r.LeaderWithID(); leaderAddr != "" {
				return
			}
		case <-ctx.Done():
			t.Fatal("timed out waiting for Leader")
		}
	}
}

func shutdown(t *testing.T, r *raft.Raft) {
	err := r.Shutdown().Error()
	if err != nil {
		t.Fatal(err)
	}
}

func main() {
	// This example shows how to use go-libp2p-raft to create a cluster
	// which agrees on a State. In order to do it, it defines a state,
	// creates three Raft nodes and launches them. We call a function which
	// lets the cluster leader repeteadly update the state. At the
	// end of the execution we verify that all members have agreed on the
	// same state.
	//
	// Some error handling has been excluded for simplicity

	// Declare an object which represents the State.
	// Note that State objects should have public/exported fields,
	// as they are [de]serialized.

	// error handling ommitted
	newPeer := func(listenPort int) host.Host {
		h, _ := libp2p.New(
			libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", listenPort)),
		)
		return h
	}

	// Create peers and make sure they know about each others.
	peer1 := newPeer(9997)
	peer2 := newPeer(9998)
	peer3 := newPeer(9999)
	defer peer1.Close()
	defer peer2.Close()
	defer peer3.Close()
	peer1.Peerstore().AddAddrs(peer2.ID(), peer2.Addrs(), peerstore.PermanentAddrTTL)
	peer1.Peerstore().AddAddrs(peer3.ID(), peer3.Addrs(), peerstore.PermanentAddrTTL)
	peer2.Peerstore().AddAddrs(peer1.ID(), peer1.Addrs(), peerstore.PermanentAddrTTL)
	peer2.Peerstore().AddAddrs(peer3.ID(), peer3.Addrs(), peerstore.PermanentAddrTTL)
	peer3.Peerstore().AddAddrs(peer1.ID(), peer1.Addrs(), peerstore.PermanentAddrTTL)
	peer3.Peerstore().AddAddrs(peer2.ID(), peer2.Addrs(), peerstore.PermanentAddrTTL)

	// Create the consensus instances and initialize them with a state.
	// Note that state is just used for local initialization, and that,
	// only states submitted via CommitState() alters the state of the
	// cluster.
	consensus1 := raftlib.NewConsensus(&KeyValue{
		// Data: map[string]string{
		// 	"1": "test",
		// },
	})
	consensus2 := raftlib.NewConsensus(&KeyValue{})
	consensus3 := raftlib.NewConsensus(&KeyValue{})

	// Create LibP2P transports Raft
	transport1, err := raftlib.NewLibp2pTransport(peer1, time.Minute)
	if err != nil {
		fmt.Println(err)
		return
	}
	transport2, err := raftlib.NewLibp2pTransport(peer2, time.Minute)
	if err != nil {
		fmt.Println(err)
		return
	}
	transport3, err := raftlib.NewLibp2pTransport(peer3, time.Minute)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer transport1.Close()
	defer transport2.Close()
	defer transport3.Close()

	// Create Raft servers configuration for bootstrapping the cluster
	// Note that both IDs and Address are set to the Peer ID.
	servers := make([]raft.Server, 0)
	for _, h := range []host.Host{peer1, peer2, peer3} {
		servers = append(servers, raft.Server{
			Suffrage: raft.Voter,
			ID:       raft.ServerID(h.ID().Pretty()),
			Address:  raft.ServerAddress(h.ID().Pretty()),
		})
	}
	serversCfg := raft.Configuration{Servers: servers}

	// Create Raft Configs. The Local ID is the PeerOID
	config1 := raft.DefaultConfig()
	config1.LogOutput = io.Discard
	config1.Logger = nil
	config1.LocalID = raft.ServerID(peer1.ID().Pretty())

	config2 := raft.DefaultConfig()
	config2.LogOutput = io.Discard
	config2.Logger = nil
	config2.LocalID = raft.ServerID(peer2.ID().Pretty())

	config3 := raft.DefaultConfig()
	config3.LogOutput = io.Discard
	config3.Logger = nil
	config3.LocalID = raft.ServerID(peer3.ID().Pretty())

	// Create snapshotStores. Use FileSnapshotStore in production.
	snapshots1, err := raft.NewFileSnapshotStore(datastore, 3, nil)
	if err != nil {
		panic(err)
	}

	snapshots2 := raft.NewInmemSnapshotStore()
	snapshots3 := raft.NewInmemSnapshotStore()

	// Create the InmemStores for use as log store and stable store.
	logStore1 := raft.NewInmemStore()
	logStore2 := raft.NewInmemStore()
	logStore3 := raft.NewInmemStore()

	// Bootsrap the stores with the serverConfigs
	raft.BootstrapCluster(config1, logStore1, logStore1, snapshots1, transport1, serversCfg.Clone())
	raft.BootstrapCluster(config2, logStore2, logStore2, snapshots2, transport2, serversCfg.Clone())
	raft.BootstrapCluster(config3, logStore3, logStore3, snapshots3, transport3, serversCfg.Clone())

	// Create Raft objects. Our consensus provides an implementation of
	// Raft.FSM
	raft1, err := raft.NewRaft(config1, consensus1.FSM(), logStore1, logStore1, snapshots1, transport1)
	if err != nil {
		log.Fatal(err)
	}
	raft2, err := raft.NewRaft(config2, consensus2.FSM(), logStore2, logStore2, snapshots2, transport2)
	if err != nil {
		log.Fatal(err)
	}
	raft3, err := raft.NewRaft(config3, consensus3.FSM(), logStore3, logStore3, snapshots3, transport3)
	if err != nil {
		log.Fatal(err)
	}

	// Create the actors using the Raft nodes
	actor1 := raftlib.NewActor(raft1)
	actor2 := raftlib.NewActor(raft2)
	actor3 := raftlib.NewActor(raft3)

	// Set the actors so that we can CommitState() and GetCurrentState()
	consensus1.SetActor(actor1)
	consensus2.SetActor(actor2)
	consensus3.SetActor(actor3)

	//WAIT FOR LEADER
	//time.Sleep(5 * time.Second)

	http.HandleFunc("/get/", func(w http.ResponseWriter, r *http.Request) {
		// Extract key from URL path
		key := strings.TrimPrefix(r.URL.Path, "/get/")
		if key == "" {
			http.Error(w, "Key not provided", http.StatusBadRequest)
			return
		}

		// Retrieve the value from the Raft state
		state, err := consensus3.GetCurrentState()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		raftState, ok := state.(*KeyValue)
		if !ok {
			http.Error(w, "Invalid Raft state type", http.StatusInternalServerError)
			return
		}

		// Retrieve the value for the given key
		value, exists := raftState.Data[key]
		if !exists {
			http.Error(w, "Key not found", http.StatusNotFound)
			return
		}

		// Prepare the response as JSON
		response := map[string]string{
			"key":   key,
			"value": value,
		}

		// Marshal the response to JSON
		jsonData, err := json.Marshal(response)
		if err != nil {
			http.Error(w, "Error processing JSON", http.StatusInternalServerError)
			return
		}

		// Return JSON response
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonData)
	})

	http.HandleFunc("/set", func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("key")
		value := r.URL.Query().Get("value")
		if key == "" || value == "" {
			http.Error(w, "Key or value not provided", http.StatusBadRequest)
			return
		}

		// Create and apply the operation to set the key-value pairs
		newState := KeyValue{
			Data: map[string]string{
				key: value,
			},
		}

		log.Println("WRITING STATE:", newState)
		var err error
		var aggred_state consensus.State
		if actor1.IsLeader() {
			aggred_state, err = consensus1.CommitState(newState)
		} else if actor2.IsLeader() {
			aggred_state, err = consensus2.CommitState(newState)
		} else if actor3.IsLeader() {
			aggred_state, err = consensus3.CommitState(newState)
		}
		if err != nil {
			//leader_id, err := actor1.Leader()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			http.Error(w, "Err ftw? "+err.Error(), http.StatusInternalServerError)
			return
		}
		if aggred_state == nil {
			fmt.Println("agreedState is nil: commited on a non-leader?")
			http.Error(w, "agreedState is nil: commited on a non-leader?", http.StatusInternalServerError)
			return
		}
		agreedRaftState := aggred_state.(*KeyValue)
		log.Println("[STATE]", agreedRaftState.Data)

		// Return success as the API response
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"Key": %q, "Value": %q}`, key, value)
	})

	log.Println("Server waiting on http://localhost:8080")
	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("HTTP server error: ", err)
	}

	raft1.Shutdown().Error()
	raft2.Shutdown().Error()
	raft3.Shutdown().Error()
}
