package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/peer"
	host "github.com/libp2p/go-libp2p-host"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/p2p/discovery"
)

// DiscoveryInterval is how often we re-publish our mDNS records.
const DiscoveryInterval = time.Minute

// DiscoveryServiceTag is used in our mDNS advertisements to discover other chat peers.
const DiscoveryServiceTag = "spiritchain-pos-network"

func main() {
	nickFlag := flag.String("nick", "", "nickname for this terminal. will be auto generated if left empty")
	chainFlag := flag.String("chain", "spiritchain-terminals", "name for the chain/topic you want to join.")
	typeFlag := flag.String("type", "", "type of terminal i.e retail or cash")

	flag.Parse()

	typePos := *typeFlag
	if typePos != "cash" && typePos != "retail" {
		panic("Invalid type flag given as input. only 'retail' and 'cash' are valid inputs")
	}

	//setup the logfile
	logfile := fmt.Sprintf("Logs/%s.log", *nickFlag)
	f_log, err := os.Create(logfile)
	if err != nil {
		panic("Error opening log file")
	}
	defer f_log.Close()
	log.SetOutput(f_log)
	//end of logfile setup

	ctx := context.Background()

	//create a new libp2p host that listens on a random TCP port
	host, err := libp2p.New(ctx, libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"))

	if err != nil {
		panic(err)
	}

	// create a new PubSub service using the GossipSub router
	ps, err := pubsub.NewGossipSub(ctx, host)
	if err != nil {
		panic(err)
	}

	// setup local mDNS discovery
	err = setupDiscovery(ctx, host)
	if err != nil {
		panic(err)
	}

	// use the nickname from the cli flag, or a default if blank
	nick := *nickFlag
	if len(nick) == 0 {
		nick = defaultNick(host.ID())
	}

	// join the chain from the cli flag, or the flag default
	chain := *chainFlag

	log.Printf("Attempting to subscribe to chain / join chat room")

	// join the chain
	cs, err := SubscribeToChain(ctx, ps, host.ID(), chain, nick, typePos)
	if err != nil {
		panic(err)
	}

	log.Printf("Attempting to start UI")

	// draw the UI
	ui := NewTerminalUI(cs)
	if err = ui.Run(); err != nil {
		printErr("error running Terminal UI: %s", err)
	}

}

// printErr is like fmt.Printf, but writes to stderr.
func printErr(m string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, m, args...)
}

// defaultNick generates a nickname based on the $USER environment variable and
// the last 8 chars of a peer ID.
func defaultNick(p peer.ID) string {
	return fmt.Sprintf("%s-%s", os.Getenv("USER"), shortID(p))
}

// shortID returns the last 8 chars of a base58-encoded peer id.
func shortID(p peer.ID) string {
	pretty := p.Pretty()
	return pretty[len(pretty)-8:]
}

// discoveryNotifee gets notified when we find a new peer via mDNS discovery
type discoveryNotifee struct {
	h host.Host
}

// HandlePeerFound connects to peers discovered via mDNS. Once they're connected,
// the PubSub system will automatically start interacting with them if they also
// support PubSub.
func (n *discoveryNotifee) HandlePeerFound(pi peer.AddrInfo) {
	// fmt.Printf("discovered new peer %s\n", pi.ID.Pretty())
	err := n.h.Connect(context.Background(), pi)
	if err != nil {
		fmt.Printf("error connecting to peer %s: %s\n", pi.ID.Pretty(), err)
	}
}

// setupDiscovery creates an mDNS discovery service and attaches it to the libp2p Host.
// This lets us automatically discover peers on the same LAN and connect to them.
func setupDiscovery(ctx context.Context, h host.Host) error {
	// setup mDNS discovery to find local peers
	disc, err := discovery.NewMdnsService(ctx, h, DiscoveryInterval, DiscoveryServiceTag)
	if err != nil {
		return err
	}

	n := discoveryNotifee{h: h}
	disc.RegisterNotifee(&n)
	return nil
}
