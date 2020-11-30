package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	peer "github.com/libp2p/go-libp2p-peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

const BlockChainSizeLimit = 1024

//this object is the subscription to a topic
type ChainSubscription struct {
	Blocks    chan *Block
	Chain     []Block
	ctx       context.Context
	ps        *pubsub.PubSub
	topic     *pubsub.Topic
	sub       *pubsub.Subscription
	self      peer.ID
	typePos   string
	topicName string
	nickName  string
	balance   map[int]float32
}

/*this struct is for sending request messages
Type -->
		 1 - Request Index
		 2 - Request BlockChain
		 3 - Return Index Message
		 4 - Return BlockChain Message
*/
type SpecialMessage struct {
	Type       int
	Timestamp  string
	Sender     string
	SenderNick string
	Receiver   string
	Index      int
	Blockchain []Block
}

//this struct represents a single block of the block chain
type Block struct {
	// Type       int
	Index      int
	PrevHash   string
	Timestamp  string
	CardId     int
	Amount     float32
	Hash       string
	Sender     string
	SenderNick string
}

func calculateBlockHash(block Block) string {
	record := strconv.Itoa(block.Index) + block.Timestamp + strconv.Itoa(block.CardId) + fmt.Sprintf("%f", block.Amount) + block.PrevHash
	h := sha256.New()
	h.Write([]byte(record))
	hashed := h.Sum(nil)
	return hex.EncodeToString(hashed)
}

/*GetGenesisBlock returns the first block to place in the chain as a placeholder for initiating the chain*/
func GetGenesisBlock() Block {
	var block Block
	block.Index = 0
	block.PrevHash = ""
	block.Timestamp = time.Now().String()
	block.CardId = -1
	block.Amount = 0
	block.Hash = "genesis-block-placeholder"
	// block.Sender = peer.ID.
	// block.SenderNick = "an"

	return block
}

/*SubscribeToChain tries to subscribe to the topic and returns a ChainSubscription object
on success*/
func SubscribeToChain(ctx context.Context, ps *pubsub.PubSub, self peer.ID, topicName string, nickName string, typePos string) (*ChainSubscription, error) {
	//join the topic ps
	topic, err := ps.Join(topicName)
	if err != nil {
		return nil, err
	}

	//now subscribe to the topic
	sub, err := topic.Subscribe()
	if err != nil {
		return nil, err
	}

	cs := &ChainSubscription{
		ctx:       ctx,
		ps:        ps,
		topic:     topic,
		sub:       sub,
		topicName: topicName,
		self:      self,
		nickName:  nickName,
		typePos:   typePos,
		Blocks:    make(chan *Block, BlockChainSizeLimit),
		Chain:     make([]Block, 0, BlockChainSizeLimit),
		balance:   make(map[int]float32),
	}
	genesisBlock := GetGenesisBlock()
	cs.Chain = append(cs.Chain, genesisBlock)
	log.Printf("Added genesis block %s\n", cs.PrintChain())

	time.Sleep(3 * time.Second)

	//Sync the blockchain for newly signed up host
	cs.RequestIndices()
	maxIndexPeer, maxLengthChain, err := cs.ReadIndices()
	if err != nil {
		panic("Problem querying other peer indices")
	}

	log.Printf("maxIndexPeer is %s with chain of length %d", maxIndexPeer, maxLengthChain)
	if maxLengthChain > 0 {
		log.Printf("Requesting chain from %s", maxIndexPeer)
		cs.RequestMaxBlockChain(maxIndexPeer)
		log.Printf("Request message sent to %s", maxIndexPeer)
		log.Printf("Attempting to receive chain from %s", maxIndexPeer)
		cs.ReadChain()
		log.Printf("Received chain")
		log.Printf(cs.PrintChain())
	}
	//sync calling complete
	go cs.readBlocks()
	return cs, nil
}

//Publish a message to the topic
func (cs *ChainSubscription) Publish(block *Block) error {
	blockBytes, err := json.Marshal(block)
	if err != nil {
		return err
	}
	// fmt.Fprintf(f, string(blockBytes))
	// cs.self.
	return cs.topic.Publish(cs.ctx, blockBytes)
}

//this function returns a slice of peer.IDs
func (cs *ChainSubscription) ListPeers() []peer.ID {
	return cs.ps.ListPeers(cs.topicName)
}

func (cs *ChainSubscription) ValidateBlockAddition(newBlock *Block) bool {
	prevBlock := cs.GetLatestBlock()
	log.Printf("Validating block: %s\n", newBlock.pretty())
	if newBlock.Index != prevBlock.Index+1 {
		log.Printf("invalid index of new block")
		return false
	}
	if newBlock.PrevHash != prevBlock.Hash {
		log.Printf("problem with prev hash")
		return false
	}
	if calculateBlockHash(*newBlock) != newBlock.Hash {
		log.Printf("hash calculation problem")
		return false
	}
	if cs.balance[newBlock.CardId]+newBlock.Amount < 0.0 {
		log.Printf("insufficient balance")
		return false
	}
	if newBlock.CardId < 1 {
		log.Printf("invalid card id")
		return false
	}

	return true
}

//get the most recent block in the chain
func (cs *ChainSubscription) GetLatestBlock() *Block {
	return &cs.Chain[len(cs.Chain)-1]
}

//print the chain (all the blocks in the chain)
func (cs *ChainSubscription) PrintChain() string {
	var res string = ""
	for _, blk := range cs.Chain {
		res += blk.pretty()
		res += "\n"
	}
	return res
}

//readBlocks pulls messages from the topic and pushes them to the Blocks channel
func (cs *ChainSubscription) readBlocks() {
	//infinite loop
	for {
		msg, err := cs.sub.Next(cs.ctx)
		if err != nil {
			close(cs.Blocks)
		}

		if msg.ReceivedFrom == cs.self {
			continue
		}

		block := new(Block)

		err = json.Unmarshal(msg.Data, block)

		if err == nil && len(block.PrevHash) > 0 {
			//send valid blocks onto the channel
			cs.Blocks <- block
			continue
		}

		specialMsg := new(SpecialMessage)

		err2 := json.Unmarshal(msg.Data, specialMsg)
		if err2 != nil {
			continue
		}

		if specialMsg.Type == 1 {
			//publish special message with your length of blockchain
			indexReturnMsg := SpecialMessage{
				Type:       3,
				Timestamp:  time.Now().String(),
				Sender:     cs.self.Pretty(),
				SenderNick: cs.nickName,
				Receiver:   specialMsg.Sender,
				Index:      cs.GetLatestBlock().Index,
			}
			indexReturnMsgJson, err := json.Marshal(indexReturnMsg)
			if err != nil {
				log.Printf("Error in marshalling index request i.e type 1 message")
			}
			err = cs.topic.Publish(cs.ctx, indexReturnMsgJson)
			if err != nil {
				log.Printf("Error in publishing index return i.e type 3 message")
			}
		}

		if specialMsg.Type == 2 && specialMsg.Receiver == cs.self.Pretty() {
			//publish special message with your length of blockchain
			chainReturnMsg := SpecialMessage{
				Type:       4,
				Timestamp:  time.Now().String(),
				Sender:     cs.self.Pretty(),
				SenderNick: cs.nickName,
				Receiver:   specialMsg.Sender,
				Blockchain: cs.Chain,
			}
			chainReturnMsgJson, err := json.Marshal(chainReturnMsg)
			if err != nil {
				log.Printf("Error in marshalling chain request i.e type 2 message")
			}
			err = cs.topic.Publish(cs.ctx, chainReturnMsgJson)
			if err != nil {
				log.Printf("Error in publishing chain return i.e type 4 message")
			}
		}

	}
}

func (block *Block) pretty() string {
	return fmt.Sprintf("Index: %d; Prev Hash: %s; Card ID: %d; Amount: %f; Timestamp: %s; Hash: %s; Sender: %s; SenderNick: %s;\n",
		block.Index, block.PrevHash, block.CardId, block.Amount, block.Timestamp, block.Hash, block.Sender, block.SenderNick)
	// return fmt.Sprintf("Index: %d; Card ID: %d; Amount: %f;",
	// 	block.Index, block.CardId, block.Amount)
}

func (specialMsg *SpecialMessage) pretty() string {
	return fmt.Sprintf("Type: %d; Sender: %s; SenderNick: %s; Receiver: %s; Index: %d;\n",
		specialMsg.Type, specialMsg.Sender, specialMsg.SenderNick, specialMsg.Receiver, specialMsg.Index)
}
