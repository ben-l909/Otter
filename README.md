# Otter

Otter is a backtester for alt-coins, existing within the Solana blockchain.  
  
The project was hackily thrown together over a week, with an emphasis on functionality - rather then readability.  
The aim of the project was to test "signal algorithms", hosted by many developers - which claim to have filters that are able to filter down the influx of new tokens on Solana, and generate calls that would leave a user in profit. Some channels advertised a win rate of up to 80%! Which is statistically very unlikely.  
  
I may get round to refactoring and cleaning up the codebase, however this project has mostly been abandoned.  

# Simulation Output
Due to the hacky nature of the project, simulations are stored as `.json` files in the `sim_output` directory.  
Each simulation has five files: `_assets`, `_balance_updates`, `_metadata`, `_portfolio` and `_trade_history`.  
  
Assets is pretty much a direct copy of the `file_metadata` database, however it only contains the data for the tokens that were bought during that simulation.  
Balance Updates stores a copy of the wallet balance (in USD) every tick (block number). It's important to note that this uses the USD/SOL conversion rate pulled from Codex to ensure that the USD balance also factors in moving SOL prices. As these simulations can span months in IRL time, this is very important. Sometimes, the SOL balance that Codex provides is invalid however. In this situation, the wallet balance data for that tick will **not** be saved.  
Metadata contains the copy of the settings that the simulation was run with. Quite important if you're comparing strategies.
Portfolio is simple, and can likely be merged into Balance Updates. It provides the ending stats for the wallet balance in SOL, and the worth of all held tokens at the finish block in SOL.  
Trade History is a log of all trades taken by the simulator.  

# Web API
The project exposes a web API, for easy integration into a CLI / Web Dashboard. I did build a web dashboard for this project, which I may release later. If I do choose to OSS the dashboard, I will leave a link here.  

The Web API exposes 4 methods.  

`/list_sims` - returns a JSON list of all simulators metadata.
```json
[
{
  "buy_amount": 0.2,
  "tps": [
    2,
    10
  ],
  "TPAmounts": [
    0.5,
    1
  ],
  "custom_opts": {
    "ny_trading_times": false
  },
  "name": "test sim one",
  "date": "2025-05-12 17:49:27",
  "id": 856384787
},
]
```

`/load_sim` - Takes in an ID as a query parameter **aswell as the panel (file) to load**. These HTTP methods were built for a web dashboard as mentioned above, and therefore the loading of data was split into multiple files, as to decrease FCP (First Contentful Paint). Previously, the simulator would return all of the data in one file, with quite a large file size. This slowed down simulation loading quite dramatically.
```json
"id": 856384787
"panel": "portfolio"

Response:
{
  "balance": 95.14731804223042,
  "token_usd_worth": 290.3096940874654,
  "token_sol_worth": 1.6864731023261175,
  "total_usd_worth": 16668.98110959002
}
```

`/run_sim` - Takes in a JSON object, and starts a simulation in a goroutine based on the provided parameters.
```go
type requestSimInput struct {
	BuyAmount      float64              `json:"buy_amount"`
	TPs            []float64            `json:"tps"`
	TPAmounts      []float64            `json:"tp_amounts"`
	CustomOpts     models.CustomOptions `json:"custom_opts"`
	Name           string               `json:"name"`
	Slippage       float64              `json:"slippage"`
	StartTimestamp int64                `json:"start_timestamp"`
	EndTimestamp   int64                `json:"end_timestamp"`
}
```

`/running_sims` - Queries a local slice, and returns any in-progress simulations.
```go
type SimStatus struct {
	StartTimestamp   int    `json:"start_timestamp"`
	CurrentTimestamp int    `json:"current_timestamp"`
	EndTimestamp     int    `json:"end_timestamp"`
	SimName          string `json:"sim_name"`
	Done             bool   `json:"done"`
}
```

# Database
The database is split into two tables.  
Each entry into the `events` table carries the foreign key `file_id` - which can be used to identify which token an event belongs to.  
As this was thrown together very quickly, the database is not normalised. I would suggest adapting this database structure if deploying into production.  

File Metadata
```s
CREATE TABLE IF NOT EXISTS file_metadata (
			file_id INTEGER,
			ca TEXT,
			call_timestamp DOUBLE,
			from_value DOUBLE,
			to_value DOUBLE,
			additional JSON,
			name TEXT,
			symbol TEXT,
			description TEXT,
			total_supply DOUBLE,
			image_uri TEXT
		);
```

Events
```s
CREATE TABLE IF NOT EXISTS events (
			file_id INTEGER,
			event_display_type TEXT,
			quote_token TEXT,
			token0_swap_value_usd DOUBLE,
			token1_swap_value_usd DOUBLE,
			timestamp INTEGER,
			block_number INTEGER
		);
```

# Harvesting Events
For collecting the data required to run the simulations, I used the [Codex](https://www.codex.io) API.  
Solana makes it incredibly difficult to harvest historical data, and therefore I opted to use their GraphQL interface.  
A query to `getTokenEvents` is made, however this query only returns a maximum of *200* results.  
Codex operates on a pay-by-request basis, and therefore this can get expensive pretty fast. I recommend limiting training data to a couple weeks, or writing a custom indexer to collect the data as it happens on-chain.  

I found the best, and most efficient way to write data into the DB was to use the built in `COPY` command e.g.  
`COPY file_metadata FROM 'metadata.csv' (FORMAT CSV, HEADER true)`  

You can attempt to write your events in with individual queries, however when testing on a dataset that contained 30 days of data for 1000 tokens, I had over 73 billion datapoints. Writing the data individually instead of with a bulk insert will not only take an extreme amount of time, but you will likely run into I/O issues.  

