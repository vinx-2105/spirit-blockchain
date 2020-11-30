package main

import (
	"encoding/json"
	"log"
	"time"
)

func (cs *ChainSubscription) RequestIndices() error {
	log.Printf("Starting request indices")
	m := SpecialMessage{
		Type:       1,
		Timestamp:  time.Now().String(),
		Sender:     cs.self.Pretty(),
		SenderNick: cs.nickName,
	}
	jsonM, err := json.Marshal(m)
	if err != nil {
		log.Printf("Error in request indices")
		return err
	}
	return cs.topic.Publish(cs.ctx, jsonM)
}

func (cs *ChainSubscription) ReadIndices() (string, int, error) {
	log.Printf("Starting read indices")
	numPeers := len(cs.topic.ListPeers())
	peerIndices := make(map[string]int)

	for i := 0; i < numPeers; {
		msg, err := cs.sub.Next(cs.ctx)
		if err != nil {
			log.Printf("Error in read indices loop: %s", err)
		}
		var indexMsg SpecialMessage
		err = json.Unmarshal(msg.Data, &indexMsg)
		if err != nil {
			continue
		}
		log.Printf("Logging msg in read indices; %s", indexMsg.pretty())
		if indexMsg.Type == 3 && indexMsg.Receiver == cs.self.Pretty() {
			i++
			log.Printf("Read Index Message message from %s", indexMsg.SenderNick)
			peerIndices[indexMsg.Sender] = indexMsg.Index
		}
	}

	maxTillNow := -1
	var maxSender string
	for k, v := range peerIndices {
		if v > maxTillNow {
			maxTillNow = v
			maxSender = k
		}
	}
	log.Printf("Exiting read indices")

	return maxSender, maxTillNow, nil

}

func (cs *ChainSubscription) RequestMaxBlockChain(receiverID string) error {
	log.Printf("Starting request indices")
	m := SpecialMessage{
		Type:       2,
		Timestamp:  time.Now().String(),
		Sender:     cs.self.Pretty(),
		SenderNick: cs.nickName,
		Receiver:   receiverID,
	}
	jsonM, err := json.Marshal(m)
	if err != nil {
		log.Printf("Error in request indices")
		return err
	}
	return cs.topic.Publish(cs.ctx, jsonM)
}

func (cs *ChainSubscription) ReadChain() error {
	log.Printf("Starting read chain")

	for {
		msg, err := cs.sub.Next(cs.ctx)
		if err != nil {
			log.Printf("Error in read indices loop: %s", err)
		}
		var chainMsg SpecialMessage
		err = json.Unmarshal(msg.Data, &chainMsg)
		if err != nil {
			log.Printf("Error in ReadChain: %s", err)
			panic("Error in read chain")
		}
		log.Printf("Logging msg in read chain; %s", chainMsg.pretty())

		if chainMsg.Type == 4 && chainMsg.Receiver == cs.self.Pretty() {
			log.Printf("Read Chain message from %s", chainMsg.SenderNick)
			cs.Chain = chainMsg.Blockchain
			break
		}
	}
	//sync the balance on cards as well
	for _, blk := range cs.Chain {
		cs.balance[blk.CardId] += blk.Amount
		log.Printf("Syncing Balances: CardID-->%d Balance-->%f", blk.CardId, blk.Amount)
	}
	log.Printf("Exiting read chain")
	return nil

}
