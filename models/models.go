package models

type Event struct {
	FileID           int
	EventDisplayType string
	QuoteToken       string
	SOLPrice         float64 // token0_swap_value_usd represents the SOL.
	TokenPrice       float64 // token1_swap_value_usd represents the Token value, in SOL.
	Timestamp        int64
	BlockNumber      int64
}

type CustomOptions struct {
	NYTradingTimes bool `json:"ny_trading_times"`
}

type SimulatorMetadata struct {
	BuyAmount  float64       `json:"buy_amount"`
	TPs        []float64     `json:"tps"`
	TPAmounts  []float64     `json:"tp_amounts`
	CustomOpts CustomOptions `json:"custom_opts"`
	Name       string        `json:"name"`
	Date       string        `json:"date"`
	ID         int           `json:"id"`
}

type Wallet struct {
	Balance         float64           `json:"balance"` // SOL
	TokenUSDWorth   float64           `json:"token_usd_worth"`
	TokenSOLWorth   float64           `json:"token_sol_worth"`
	TotalUSDWorth   float64           `json:"total_usd_worth"`
	BalanceTracking map[int64]float64 `json:"balance_tracking"` // USD
	Assets          map[int]Asset     `json:"assets"`           // map[file_id]Asset
	Events          []SimEvent        `json:"sim_events"`
}

type Portfolio struct {
	SOLBalance    float64 `json:"balance"`
	TokenUSDWorth float64 `json:"token_usd_worth"`
	TokenSOLWorth float64 `json:"token_sol_worth"`
	TotalUSDWorth float64 `json:"total_usd_worth"`
}

type SimEvent struct {
	BlockNumber int64
	Type        string  `json:"type"`
	SOLChange   float64 `json:"sol_change"` // details +- of sol on the event
	FileID      int     `json:"file_id"`
	TokenPrice  float64 `json:"token_price"`
}

type Asset struct {
	FileID          int               `json:"file_id"`
	Name            string            `json:"name"`
	ContractAddress string            `json:"contract_address"`
	Description     string            `json:"description"`
	CallTimestamp   int64             `json:"call_timestamp"`
	EntryPrice      float64           `json:"entry_price"`
	TPPrice         float64           `json:"tp_price"`
	TPStage         int               `json:"tp_stage"`
	QueuedTP        int64             `json:"queued_tp"`
	QueuedPrice     float64           `json:"queued_price"`
	ImageURL        string            `json:"image_url"`
	Price           float64           `json:"price"`
	Balance         float64           `json:"balance"`
	TradingHistory  map[int64]float64 `json:"trading_history"` // map[blockNumber]TokenPrice
}

func DeepCopyWallet(src *Wallet) *Wallet {
	if src == nil {
		return nil
	}

	dst := &Wallet{
		Balance:         src.Balance,
		TokenUSDWorth:   src.TokenUSDWorth,
		TokenSOLWorth:   src.TokenSOLWorth,
		TotalUSDWorth:   src.TotalUSDWorth,
		Assets:          make(map[int]Asset),
		BalanceTracking: src.BalanceTracking,
		Events:          src.Events,
	}

	for k, asset := range src.Assets {
		copiedAsset := asset
		copiedAsset.TradingHistory = make(map[int64]float64, 0)
		// if asset.TradingHistory != nil {
		// copiedHistory := make(map[int64]float64)
		// for block, price := range asset.TradingHistory {
		// 	copiedHistory[block] = price
		// }
		// copiedAsset.TradingHistory = copiedHistoryq
		// }
		dst.Assets[k] = copiedAsset
	}

	return dst
}
