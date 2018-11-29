package syncing_test

import (
	"reflect"
	"testing"

	bc "github.com/harmony-one/harmony/blockchain"
	"github.com/harmony-one/harmony/crypto/pki"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/syncing"
	"github.com/harmony-one/harmony/syncing/downloader"
	pb "github.com/harmony-one/harmony/syncing/downloader/proto"
	"google.golang.org/grpc"
)

const (
	serverPort1 = "9996"
	serverPort2 = "9997"
	serverPort3 = "9998"
	serverIP    = "127.0.0.1"
	clientPort  = "9999"
)

var (
	PriIntOne        = 111
	PriIntTwo        = 222
	PriIntThree      = 222
	TestAddressOne   = pki.GetAddressFromInt(PriIntOne)
	TestAddressTwo   = pki.GetAddressFromInt(PriIntTwo)
	TestAddressThree = pki.GetAddressFromInt(PriIntThree)
	ShardID          = uint32(0)
	ServerPorts      = []string{serverPort1, serverPort2, serverPort3}
)

type FakeNode struct {
	bc            *bc.Blockchain
	server        *downloader.Server
	ip            string
	port          string
	grpcServer    *grpc.Server
	doneFirstTime bool
}

// GetBlockHashes used for state download.
func (node *FakeNode) GetBlockHashes() [][]byte {
	res := [][]byte{}
	for _, block := range node.bc.Blocks {
		res = append(res, block.Hash[:])
	}
	return res
}

// GetBlocks used for state download.
func (node *FakeNode) GetBlocks() [][]byte {
	res := [][]byte{}
	for _, block := range node.bc.Blocks {
		res = append(res, block.Serialize())
	}
	return res
}

// SetBlockchain is used for testing
func (node *FakeNode) Init(ip, port string) {
	addresses := [][20]byte{TestAddressOne, TestAddressTwo}
	node.bc = bc.CreateBlockchainWithMoreBlocks(addresses, ShardID)
	node.ip = ip
	node.port = port

	node.server = downloader.NewServer(node)
}

// Start ...
func (node *FakeNode) Start() error {
	var err error
	node.grpcServer, err = node.server.Start(node.ip, node.port)
	return err
}

func (node *FakeNode) addOneMoreBlock() {
	addresses := [][20]byte{TestAddressThree}
	node.bc.Blocks = append(node.bc.Blocks, bc.CreateMoreBlocks(addresses, ShardID)...)
}

func (node *FakeNode) CalculateResponse(request *pb.DownloaderRequest) (*pb.DownloaderResponse, error) {
	response := &pb.DownloaderResponse{}
	if request.Type == pb.DownloaderRequest_HEADER {
		for _, block := range node.bc.Blocks {
			response.Payload = append(response.Payload, block.Hash[:])
		}
		if !node.doneFirstTime {
			node.addOneMoreBlock()
		}
		node.doneFirstTime = true
	} else {
		for i := range request.Hashes {
			block := node.bc.FindBlock(request.Hashes[i])
			response.Payload = append(response.Payload, block.Serialize())
		}
	}
	return response, nil
}

func TestSyncing(t *testing.T) {
	fakeNodes := []*FakeNode{&FakeNode{}, &FakeNode{}, &FakeNode{}}
	for i := range fakeNodes {
		fakeNodes[i].Init(serverIP, ServerPorts[i])
		if err := fakeNodes[i].Start(); err != nil {
			t.Error(err)
		}
	}

	stateSync := &syncing.StateSync{}
	bc := &bc.Blockchain{}
	peers := make([]p2p.Peer, len(fakeNodes))
	for i := range peers {
		peers[i].Ip = fakeNodes[i].ip
		peers[i].Port = fakeNodes[i].port
	}

	stateSync.StartStateSync(peers, bc)

	for i := range bc.Blocks {
		if !reflect.DeepEqual(bc.Blocks[i], fakeNodes[0].bc.Blocks[i]) {
			t.Error("not equal")
		}
	}

	for _, fakeNode := range fakeNodes {
		fakeNode.grpcServer.Stop()
	}
}