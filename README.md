Author: Vineet Madan<br>
Date: 27th November 2020<br>
Purpose: Assignment 3 of CS-607<br>


##Notes and Assumptions:
1. The PoS terminal software is distinguishable between retail and cash type PoSs. Retail PoS can show balance and deduct balance. Cash PoS can do a recharge (or add money to card) or show balance.
2. In the case of a network disruption, I have assumed that the program will be started again
3. Detailed logs will be stored in the Logs folder
4. The block chain copies (ledgers)  will be stored in the Chains folder for each terminal
5. Run different instances at an interval of a minimum 3 seconds to avoid synchronization difficulties
6. The program is to be given input by the user of the PoS terminal. Giving command line arguments makes less sense here.


##Build and Run Instructions:
1. To run the executable directly go to step 2 or run `go build -o posterminal` in directory where main.go is located to build the executable
2. To run an instance of a PoS terminal, run `./posterminal -nick=<NICKNAME_FOR_TERMINAL> -type=<cash|retail>`
3. On each instance, there will be a kind of full screen terminal interface for interacting with the program. A transaction is input as `<CARD_ID> <TRANSACTION_AMOUNT>`
   1. Transaction amount can be positive, negative or zero
   2. If the system is recording a `CARD_ID` for the first time, it means a new card is being issued
   3. The instance takes 3-4 seconds to startup to provide for synchronization time with the network

##Example Run Commands:<br>
`./posterminal -nick=vineet -type=cash`<br>
`./posterminal -nick=mudit -type=retail`<br>
