package simulator

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"math/rand"
	"otter/database"
	"otter/models"
	"sort"
	"time"
)

type Simulator struct {
	DBConnection *database.Database
	CAInfo       map[int]models.Asset

	SimulatorStartBlock int64
	BuyingEnabled       bool
	SimulatorEndBlock   int64

	Name string

	BuyAmount float64

	SlippagePercentage float64
	Status             *SimStatus

	TPs       []float64
	TPAmounts []float64

	CustomOpts models.CustomOptions

	Wallet *models.Wallet
	Stats  Statistics
}

type Statistics struct {
	TotalBuys       int
	TotalSells      int
	TotalBuyAmount  float64
	TotalSellAmount float64
}

type SimStatus struct {
	StartTimestamp   int    `json:"start_timestamp"`
	CurrentTimestamp int    `json:"current_timestamp"`
	EndTimestamp     int    `json:"end_timestamp"`
	SimName          string `json:"sim_name"`
	Done             bool   `json:"done"`
}

const BATCH_SIZE = 100

// const TAKE_PROFIT_1 = 20

func Init(db *database.Database, buyAmount float64, TPs []float64, TPAmounts []float64, CustomOpts models.CustomOptions, SimulatorName string, slippage float64, startTimestamp int64, endTimestamp int64) Simulator {
	s := Simulator{
		DBConnection: db,
		Stats: Statistics{
			TotalBuys:       0,
			TotalSells:      0,
			TotalBuyAmount:  0.0,
			TotalSellAmount: 0.0,
		},
		TPs:       []float64{2, 10},
		TPAmounts: []float64{0.5, 1},
		CustomOpts: models.CustomOptions{
			NYTradingTimes: false,
		},
		Name:                SimulatorName,
		BuyAmount:           buyAmount,
		SlippagePercentage:  slippage,
		SimulatorStartBlock: startTimestamp,
		SimulatorEndBlock:   endTimestamp,
	}

	s.CAInfo, _ = db.GetContractAddressInfo()

	return s
}

func (s *Simulator) UpdateWalletBalance(e models.Event) {
	tokenSOLWorth := 0.0

	for _, asset := range s.Wallet.Assets {
		tokenSOLWorth += asset.Balance * asset.Price
	}

	tokenUSDWorth := e.SOLPrice * tokenSOLWorth
	SOLUSDWorth := s.Wallet.Balance * e.SOLPrice

	s.Wallet.TotalUSDWorth = tokenUSDWorth + SOLUSDWorth
	s.Wallet.TokenSOLWorth = tokenSOLWorth
	s.Wallet.TokenUSDWorth = tokenUSDWorth
	if !(math.IsNaN(e.SOLPrice)) && !math.IsNaN(tokenSOLWorth) && !math.IsNaN(s.Wallet.Balance) {
		s.Wallet.BalanceTracking[e.BlockNumber] = (tokenSOLWorth + s.Wallet.Balance) * e.SOLPrice
	} else {
		fmt.Println(e.SOLPrice, tokenSOLWorth, s.Wallet.Balance)
	}
}

func (s *Simulator) process_events_chronologically(events []models.Event) (int, bool) {
	sort.Slice(events, func(i, j int) bool {
		return events[i].BlockNumber < events[j].BlockNumber
	})

	previous_block_number := 0
	last_known_timestamp := 0
	for _, event := range events {
		if asset, ok := s.Wallet.Assets[event.FileID]; ok {
			if !math.IsNaN(event.TokenPrice) {
				// buy tx
				if asset.Balance == 0 && (event.Timestamp >= asset.CallTimestamp-2 && event.Timestamp <= asset.CallTimestamp+2) {
					if s.Wallet.Balance > s.BuyAmount+0.1 {
						tm := time.Unix(event.Timestamp, 0)

						if s.CustomOpts.NYTradingTimes {
							if tm.Hour() < 9 || tm.Hour() > 16 {
								continue
							}
						}

						fmt.Println("[BUY] Bought token for 0.2 SOL at ", tm)
						// 0.2 is the buy amount
						asset.Balance = s.BuyAmount / event.TokenPrice
						s.Wallet.Balance -= s.BuyAmount
						asset.TPPrice = event.TokenPrice * s.TPs[0]
						asset.EntryPrice = event.TokenPrice

						s.Stats.TotalBuys += 1
						s.Stats.TotalBuyAmount += s.BuyAmount

						event := models.SimEvent{
							BlockNumber: event.BlockNumber,
							Type:        "BUY",
							SOLChange:   -s.BuyAmount,
							FileID:      event.FileID,
							TokenPrice:  event.TokenPrice,
						}

						s.Wallet.Events = append(s.Wallet.Events, event)
					}
				}

				if asset.Balance != 0 {

					if event.TokenPrice > asset.TPPrice && asset.QueuedTP == 0 {
						asset.QueuedTP = event.BlockNumber
						asset.QueuedPrice = event.TokenPrice
					}

					if event.BlockNumber > asset.QueuedTP+3 && asset.QueuedTP != 0 {
						// slippage estimation
						slippage := (((event.TokenPrice - asset.QueuedPrice) / asset.QueuedPrice) * 100)
						if slippage >= -s.SlippagePercentage && slippage <= s.SlippagePercentage {
							tokenSaleAmount := asset.Balance * s.TPAmounts[asset.TPStage]
							saleValue := tokenSaleAmount * (event.TokenPrice)

							fmt.Println("[TP-"+fmt.Sprint(asset.TPStage+1)+"] TP Hit (3 block delay) | Sell amount:", saleValue, "SOL", "| Sell Percentage:", s.TPAmounts[asset.TPStage]*100)
							s.Wallet.Balance += saleValue
							s.Stats.TotalSellAmount += saleValue

							s.UpdateWalletBalance(event)
							fmt.Println("Wallet Balance Update: new SOL Balance:", s.Wallet.Balance, "new Total USD worth: ", s.Wallet.TotalUSDWorth, "new token sol worth: ", s.Wallet.TokenSOLWorth, "new token usd worth: ", s.Wallet.TokenUSDWorth)

							asset.Balance -= tokenSaleAmount

							if len(s.TPs) > 1 && len(s.TPs) >= asset.TPStage+2 {
								fmt.Println(asset.TPStage)
								asset.TPStage += 1
								asset.TPPrice = asset.EntryPrice * s.TPs[asset.TPStage]
							}

							event := models.SimEvent{
								BlockNumber: event.BlockNumber,
								Type:        "SELL",
								SOLChange:   saleValue,
								FileID:      event.FileID,
								TokenPrice:  event.TokenPrice,
							}

							s.Wallet.Events = append(s.Wallet.Events, event)

							s.Stats.TotalSells += 1
							asset.QueuedTP = 0
							asset.QueuedPrice = 0.0
						} else {
							// fmt.Println("Slippage exceeded 10%. Cancelling TP.")
							asset.QueuedTP = 0
							asset.QueuedPrice = 0.0
						}

					}

					asset.Price = event.TokenPrice
					asset.TradingHistory[event.BlockNumber] = event.TokenPrice
				}
			}

			s.Wallet.Assets[event.FileID] = asset
		}

		// update wallet balance every (tick), block number
		if previous_block_number != int(event.BlockNumber) {
			s.UpdateWalletBalance(event)
		}

		previous_block_number = int(event.BlockNumber)
		last_known_timestamp = int(event.Timestamp)

		s.Status.CurrentTimestamp = int(event.Timestamp)
	}

	return last_known_timestamp, true
}

func (s *Simulator) fetch_next_event_batch(next_ts int64) []models.Event {
	events, _ := s.DBConnection.BatchGetEventsForTimestamps(next_ts, BATCH_SIZE)
	return events
}

func (s *Simulator) Run(simStatus *SimStatus) {
	simStatus.StartTimestamp = int(s.SimulatorStartBlock)
	simStatus.CurrentTimestamp = int(s.SimulatorStartBlock)
	simStatus.EndTimestamp = int(s.SimulatorEndBlock)
	simStatus.SimName = s.Name

	s.Status = simStatus

	s.InitWallet()

	// sim_start is used if you want to run a full simulation, start to end.
	// However, date range can be narrowed, and should be. Remember to add support for that.
	nextBlock := s.SimulatorStartBlock
	fmt.Println("NEXT BLOCK ->", nextBlock)
	for {
		if nextBlock > s.SimulatorEndBlock {
			break
		}
		events := s.fetch_next_event_batch(nextBlock)

		b, con := s.process_events_chronologically(events)
		nextBlock = int64(b) + 1

		if !con {
			break
		}
	}

	fmt.Println("Final Wallet Balance Update: new SOL Balance:", s.Wallet.Balance, "new Total USD worth: ", s.Wallet.TotalUSDWorth, "new token sol worth: ", s.Wallet.TokenSOLWorth, "new token usd worth: ", s.Wallet.TokenUSDWorth)

	fmt.Println("Processed from", time.Unix(s.SimulatorStartBlock, 0), "->", time.Unix(nextBlock, 0))

	fmt.Println("-- Sim Stats -- ")
	fmt.Println("Total Signals Bought:", s.Stats.TotalBuys)
	fmt.Println("Total Signals Sold:", s.Stats.TotalSells)
	fmt.Println("Total spent on signals:", s.Stats.TotalBuyAmount)
	fmt.Println("Total SOL recieved from sells:", s.Stats.TotalSellAmount)

	finalAssets := models.DeepCopyWallet(s.Wallet)
	finalAssets.Assets = make(map[int]models.Asset)
	for i, asset := range s.Wallet.Assets {
		if len(asset.TradingHistory) != 0 {
			asset.TradingHistory = make(map[int64]float64, 0)
			finalAssets.Assets[i] = asset
		}
	}

	newBalances := make(map[int64]float64, 0)

	keys := make([]int64, 0, len(s.Wallet.BalanceTracking))
	for id := range s.Wallet.BalanceTracking {
		keys = append(keys, id)
	}

	sort.Slice(keys, func(i, j int) bool {
		return keys[i] < keys[j]
	})

	for i, block := range keys {
		bal := s.Wallet.BalanceTracking[block]
		if i == 0 {
			newBalances[block] = bal
		} else {
			prev := keys[i-1]
			if bal != s.Wallet.BalanceTracking[prev] {
				newBalances[block] = bal
			}
		}
	}

	finalAssets.BalanceTracking = newBalances

	jsonBytes, err := json.MarshalIndent(finalAssets, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	currentTime := time.Now()

	simID := rand.Intn(999999999-111111111+1) + 111111111

	simulatorMetadata := models.SimulatorMetadata{
		BuyAmount:  s.BuyAmount,
		TPs:        s.TPs,
		TPAmounts:  s.TPAmounts,
		CustomOpts: s.CustomOpts,
		Name:       s.Name,
		Date:       currentTime.Format("2006-01-02 15:04:05"),
		ID:         simID,
	}

	metaBytes, metaErr := json.MarshalIndent(simulatorMetadata, "", "  ")
	if metaErr != nil {
		log.Fatal("meta err" + err.Error())
	}

	err = ioutil.WriteFile("sim_output/"+fmt.Sprint(simID)+".json", jsonBytes, 0644)
	err = ioutil.WriteFile("sim_output/"+fmt.Sprint(simID)+".json_meta", metaBytes, 0644)

	if err != nil {
		panic(err)
	}

	s.Status.Done = true
}

func (s *Simulator) InitWallet() {
	w := models.Wallet{
		Balance:       100,
		TokenUSDWorth: 0.0,
		TokenSOLWorth: 0.0,
		TotalUSDWorth: 0.0,

		Assets:          make(map[int]models.Asset, 0),
		BalanceTracking: make(map[int64]float64, 0),
		Events:          []models.SimEvent{},
	}

	for file_id, asset := range s.CAInfo {
		w.Assets[file_id] = asset
	}

	s.Wallet = &w
}
