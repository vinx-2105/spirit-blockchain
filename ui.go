package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/rivo/tview"
)

type TerminalUI struct {
	cs              *ChainSubscription
	app             *tview.Application
	peersList       *tview.TextView
	chainViewWriter io.Writer
	inputCh         chan string
	doneCh          chan struct{}
}

func NewTerminalUI(cs *ChainSubscription) *TerminalUI {
	//create a new application
	app := tview.NewApplication()

	//make a text view to contain our blockchain blocks
	chainTextView := tview.NewTextView()
	chainTextView.SetDynamicColors(true)
	chainTextView.SetBorder(true)
	chainTextView.SetTitle(fmt.Sprintf("PoS Terminal: %s", cs.nickName))
	chainTextView.SetScrollable(true)

	//set changed function...don't automatically refresh
	chainTextView.SetChangedFunc(func() {
		app.Draw()
	})

	//channel for typing inputs into
	inputCh := make(chan string, 32)
	inputField := tview.NewInputField().
		SetLabel(fmt.Sprintf("%s PoS Terminal Input: %s >", cs.typePos, cs.nickName)).
		SetFieldWidth(0).
		SetFieldBackgroundColor(tcell.ColorBlack)

	//SetDoneFunc function is called when is called when user hits enter or tab
	inputField.SetDoneFunc(func(key tcell.Key) {
		//if not tab don't do anything
		if key != tcell.KeyEnter {
			return
		}
		//get text from the input field
		line := inputField.GetText()
		if len(line) == 0 {
			//input is blank...then don't do anything
			return
		}

		if line == "/quit" {
			app.Stop()
			return
		}

		//in other cases, send the line to the input channel and clear the input field
		inputCh <- line
		inputField.SetText("")
	})

	//another text view to show the list of peers
	peersListView := tview.NewTextView()
	peersListView.SetBorder(true)
	peersListView.SetTitle("Peers")

	chainAndPeersPanel := tview.NewFlex().AddItem(chainTextView, 0, 1, false).AddItem(peersListView, 20, 1, false)

	fullPanel := tview.NewFlex().SetDirection(tview.FlexRow).AddItem(chainAndPeersPanel, 0, 1, false).AddItem(inputField, 1, 1, true)

	//set the full panel as the root of the UI
	app.SetRoot(fullPanel, true)

	return &TerminalUI{
		cs:              cs,
		app:             app,
		peersList:       peersListView,
		chainViewWriter: chainTextView,
		inputCh:         inputCh,
		doneCh:          make(chan struct{}, 1),
	}
}

// end signals the event loop to exit gracefully
func (ui *TerminalUI) end() {
	ui.doneCh <- struct{}{}
}

//pulls the list of peers currently subscribed to the chain and displays
func (ui *TerminalUI) refreshPeers() {
	peers := ui.cs.ListPeers()
	idStrs := make([]string, len(peers))
	for i, p := range peers {
		idStrs[i] = p.Pretty()[36:]
	}

	ui.peersList.SetText(strings.Join(idStrs, "\n"))
	ui.app.Draw()
}

// withColor wraps a string with color tags for display in the messages text box.
func withColor(color, msg string) string {
	return fmt.Sprintf("[%s]%s[-]", color, msg)
}

func (ui *TerminalUI) displayBlock(block *Block) {
	prompt := withColor("green", fmt.Sprintf("<%s>:", block.SenderNick))
	fmt.Fprintf(ui.chainViewWriter, "%s %s \n", prompt, block.pretty())
}

func (ui *TerminalUI) displayOwnBlock(block *Block) {
	prompt := withColor("blue", fmt.Sprintf("<%s>:", ui.cs.nickName))
	fmt.Fprintf(ui.chainViewWriter, "%s %s \n Current Balance on Card: %f\n", prompt, block.pretty(), ui.cs.balance[block.CardId])
}

func (ui *TerminalUI) displayBalance(cardId int) {
	prompt := withColor("yellow", fmt.Sprintf("<SYSTEM>:"))
	fmt.Fprintf(ui.chainViewWriter, "%s Current Balance on Card: %f\n", prompt, ui.cs.balance[cardId])
}

func (ui *TerminalUI) displaySystemMessage(message string) {
	prompt := withColor("red", fmt.Sprintf("<SYSTEM>:"))
	fmt.Fprintf(ui.chainViewWriter, "%s %s \n", prompt, message)
}

func getBlockFromInputString(input string, cs *ChainSubscription) (*Block, error) {
	splits := strings.Split(input, " ")
	if len(splits) != 2 {
		return nil, errors.New("Invalid Input Format")
	}

	cardId, err := strconv.Atoi(splits[0])
	if err != nil {
		return nil, err
	}

	amount, err := strconv.ParseFloat(splits[1], 32)
	if err != nil {
		return nil, err
	}

	latestBlock := cs.GetLatestBlock()

	var block Block
	block.Index = latestBlock.Index + 1
	block.PrevHash = latestBlock.Hash
	block.Timestamp = time.Now().String()
	block.CardId = cardId
	block.Amount = float32(amount)
	block.Hash = calculateBlockHash(block)
	block.Sender = cs.self.Pretty()
	block.SenderNick = cs.nickName

	return &block, nil
}

//log the block chain contents to file
func (ui *TerminalUI) logBlockChain() {
	file, err := os.Create(fmt.Sprintf("Chains/%s.txt", ui.cs.nickName))
	if err != nil {
		panic("Error opening Chain log files")
	}
	for _, blk := range ui.cs.Chain {
		fmt.Fprintf(file, blk.pretty()+"\n")
	}
}

//handleEvents runs an event loop that sends user input to the chat room and displays the messages received from the chat room.
//It also periodically refreshes the list of the peers on the UI
func (ui *TerminalUI) handleEvents() {
	peerRefreshTicker := time.NewTicker(time.Second)
	defer peerRefreshTicker.Stop()

	for {
		select {
		case input := <-ui.inputCh:
			block, err := getBlockFromInputString(input, ui.cs)
			if err != nil {
				log.Printf("%s", err)
				ui.displaySystemMessage("Problem with transaction format: Valid format is <CARD_ID (int)> <AMOUNT (float)>.")
				continue
			}
			if block.Amount < 0 && ui.cs.typePos == "cash" {
				ui.displaySystemMessage("Problem with transaction: Amount cannot be deducted from card on a Cash type POS terminal")
				continue
			}
			if block.Amount > 0 && ui.cs.typePos == "retail" {
				ui.displaySystemMessage("Problem with transaction: Amount cannot be added to card on a Retail type POS terminal")
				continue
			}
			//when the user inputs a transaction, publish it to the chat room and print it to the message window
			if ui.cs.ValidateBlockAddition(block) == true {
				if block.Amount != 0 {
					err = ui.cs.Publish(block)
					if err != nil {
						printErr("Publish Err: %s", err)
					}
					ui.cs.balance[block.CardId] += block.Amount
					ui.cs.Chain = append(ui.cs.Chain, *block)
					ui.displayOwnBlock(block)
					ui.logBlockChain()
				} else {
					ui.displayBalance(block.CardId)
				}
			} else {
				ui.displaySystemMessage("Problem with transaction: Insufficient balance on card, card invalid or other internal problem. See system logs for more detail.")
			}

		case blk := <-ui.cs.Blocks:
			if ui.cs.ValidateBlockAddition(blk) == true {
				ui.cs.Chain = append(ui.cs.Chain, *blk)
				ui.cs.balance[blk.CardId] += blk.Amount
				ui.displayBlock(blk)
				ui.logBlockChain()
			} else {
				ui.displaySystemMessage("Problem with transaction: Insufficient balance on card, card invalid or other internal problem. See system logs for more detail.")
			}

		case <-peerRefreshTicker.C:
			ui.refreshPeers()

		case <-ui.cs.ctx.Done():
			return

		case <-ui.doneCh:
			return
		}
	}
}

//this function runs the handle events loop
func (ui *TerminalUI) Run() error {
	go ui.handleEvents()
	defer ui.end()

	return ui.app.Run()
}
