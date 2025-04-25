package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"otter/database"
	"otter/models"
	"otter/simulator"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"

	"github.com/gin-gonic/gin"
)

var forever chan bool
var ShutdownSignal = make(chan os.Signal, 1)

var DBConnection database.Database

type Sim struct {
	Meta models.SimulatorMetadata `json:"meta"`
}

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

var RunningSims []*simulator.SimStatus

func main() {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"OPTIONS", "PUT", "POST", "GET"},
		AllowHeaders:     []string{"Origin", "ngrok-skip-browser-warning", "Content-Type"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))
	r.GET("/list_sims", listSimsHandler)
	r.GET("/load_sim", loadSimHandler)
	r.POST("/run_sim", requestSimHandler)
	r.GET("/running_sims", runningSimsHandler)

	r.Run(":80")

	signal.Notify(ShutdownSignal, syscall.SIGINT, syscall.SIGTERM)
	go shutdown()
}

// requestSimHandler starts a new simulation based on JSON input
func requestSimHandler(c *gin.Context) {
	var input requestSimInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON payload"})
		return
	}

	dbConn := database.Connect()
	fmt.Println(input)
	s := simulator.Init(&dbConn,
		input.BuyAmount,
		input.TPs,
		input.TPAmounts,
		models.CustomOptions{NYTradingTimes: input.CustomOpts.NYTradingTimes},
		input.Name,
		input.Slippage,
		input.StartTimestamp,
		input.EndTimestamp,
	)

	var simStatus = &simulator.SimStatus{}

	RunningSims = append(RunningSims, simStatus)

	go s.Run(simStatus)

	c.JSON(http.StatusAccepted, gin.H{"status": "simulation started"})
}

// listSimsHandler scans sim_output for all .json files, parses them into Sim structs,
// and returns a JSON array of all available sims
func listSimsHandler(c *gin.Context) {
	dir := "sim_output"
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var sims []models.SimulatorMetadata
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".json_meta") {
			continue
		}

		path := filepath.Join(dir, f.Name())
		data, err := ioutil.ReadFile(path)
		if err != nil {
			continue
		}

		var sim models.SimulatorMetadata
		if err := json.Unmarshal(data, &sim); err != nil {
			fmt.Println(err)
			continue
		}

		sims = append(sims, sim)
	}

	c.JSON(http.StatusOK, sims)
}

// runningSimsHandler returns currently processing simulations and prunes completed ones
func runningSimsHandler(c *gin.Context) {
	var active []*simulator.SimStatus
	var remaining []*simulator.SimStatus
	for _, s := range RunningSims {
		if !s.Done {
			active = append(active, s)
			remaining = append(remaining, s)
		}
	}
	// update global slice to only include still-running sims
	RunningSims = remaining

	c.JSON(http.StatusOK, active)
}

// loadSimHandler returns the raw JSON payload for a given simulation
// Call: GET /load_sim?id=<filename_without_extension>
func loadSimHandler(c *gin.Context) {
	dir := "sim_output"
	id := c.Query("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing id parameter"})
		return
	}

	// sanitize filename to avoid directory traversal
	safeId := filepath.Base(id)
	target := filepath.Join(dir, safeId+".json")

	data, err := ioutil.ReadFile(target)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.Data(http.StatusOK, "application/json", data)
}

func shutdown() {
	DBConnection.Disconnect()
}
