package exchange

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/pkg/logger"
	"github.com/selivandex/trader-bot/pkg/models"
)

// BybitWebSocket handles WebSocket connection to Bybit
type BybitWebSocket struct {
	conn           *websocket.Conn
	url            string
	symbols        []string
	timeframes     []string
	candleChan     chan models.Candle
	errorChan      chan error
	mu             sync.Mutex
	reconnectDelay time.Duration
	ctx            context.Context
	cancel         context.CancelFunc
}

// BybitWSMessage represents Bybit WebSocket message structure
type BybitWSMessage struct {
	Topic string          `json:"topic"`
	Type  string          `json:"type"`
	Data  json.RawMessage `json:"data"`
	Ts    int64           `json:"ts"`
}

// BybitKlineData represents Bybit kline data
type BybitKlineData struct {
	Start     int64  `json:"start"`
	End       int64  `json:"end"`
	Interval  string `json:"interval"`
	Open      string `json:"open"`
	Close     string `json:"close"`
	High      string `json:"high"`
	Low       string `json:"low"`
	Volume    string `json:"volume"`
	Turnover  string `json:"turnover"`
	Confirm   bool   `json:"confirm"`
	Timestamp int64  `json:"timestamp"`
}

// NewBybitWebSocket creates new Bybit WebSocket connection
func NewBybitWebSocket(symbols []string, timeframes []string, testnet bool) *BybitWebSocket {
	url := "wss://stream.bybit.com/v5/public/linear"
	if testnet {
		url = "wss://stream-testnet.bybit.com/v5/public/linear"
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &BybitWebSocket{
		url:            url,
		symbols:        symbols,
		timeframes:     timeframes,
		candleChan:     make(chan models.Candle, 1000),
		errorChan:      make(chan error, 10),
		reconnectDelay: 5 * time.Second,
		ctx:            ctx,
		cancel:         cancel,
	}
}

// Connect establishes WebSocket connection
func (bw *BybitWebSocket) Connect() error {
	bw.mu.Lock()
	defer bw.mu.Unlock()

	conn, _, err := websocket.DefaultDialer.Dial(bw.url, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to Bybit WebSocket: %w", err)
	}

	bw.conn = conn

	// Subscribe to kline topics
	if err := bw.subscribe(); err != nil {
		conn.Close()
		return fmt.Errorf("failed to subscribe: %w", err)
	}

	// Start reading messages
	go bw.readMessages()

	// Start ping/pong handler
	go bw.pingHandler()

	logger.Info("Bybit WebSocket connected",
		zap.String("url", bw.url),
		zap.Strings("symbols", bw.symbols),
		zap.Strings("timeframes", bw.timeframes),
	)

	return nil
}

// subscribe sends subscription messages
func (bw *BybitWebSocket) subscribe() error {
	// Bybit V5 kline topic: "kline.{interval}.{symbol}"
	// Example: "kline.5.BTCUSDT"
	topics := []string{}

	// Map timeframes to Bybit intervals
	intervalMap := map[string]string{
		"1m":  "1",
		"5m":  "5",
		"15m": "15",
		"1h":  "60",
		"4h":  "240",
		"1d":  "D",
	}

	for _, symbol := range bw.symbols {
		// Convert BTC/USDT -> BTCUSDT
		bybitSymbol := convertSymbolToBybit(symbol)

		for _, tf := range bw.timeframes {
			interval, ok := intervalMap[tf]
			if !ok {
				logger.Warn("unsupported timeframe for Bybit WebSocket",
					zap.String("timeframe", tf),
				)
				continue
			}

			topic := fmt.Sprintf("kline.%s.%s", interval, bybitSymbol)
			topics = append(topics, topic)
		}
	}

	if len(topics) == 0 {
		return fmt.Errorf("no valid topics to subscribe")
	}

	// Subscribe message
	subMsg := map[string]interface{}{
		"op":   "subscribe",
		"args": topics,
	}

	if err := bw.conn.WriteJSON(subMsg); err != nil {
		return fmt.Errorf("failed to send subscribe message: %w", err)
	}

	logger.Info("subscribed to Bybit kline topics",
		zap.Strings("topics", topics),
	)

	return nil
}

// readMessages reads messages from WebSocket
func (bw *BybitWebSocket) readMessages() {
	defer func() {
		bw.mu.Lock()
		if bw.conn != nil {
			bw.conn.Close()
		}
		bw.mu.Unlock()

		// Attempt reconnect
		if bw.ctx.Err() == nil {
			logger.Info("attempting to reconnect Bybit WebSocket...")
			time.Sleep(bw.reconnectDelay)
			if err := bw.Connect(); err != nil {
				logger.Error("failed to reconnect", zap.Error(err))
			}
		}
	}()

	for {
		select {
		case <-bw.ctx.Done():
			return
		default:
		}

		_, message, err := bw.conn.ReadMessage()
		if err != nil {
			logger.Error("WebSocket read error", zap.Error(err))
			bw.errorChan <- err
			return
		}

		// Parse message
		var msg BybitWSMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			logger.Warn("failed to parse WebSocket message", zap.Error(err))
			continue
		}

		// Handle kline updates
		if msg.Topic != "" && len(msg.Data) > 0 {
			bw.handleKlineMessage(msg)
		}
	}
}

// handleKlineMessage processes kline updates
func (bw *BybitWebSocket) handleKlineMessage(msg BybitWSMessage) {
	// Parse kline data
	var klines []BybitKlineData
	if err := json.Unmarshal(msg.Data, &klines); err != nil {
		logger.Warn("failed to parse kline data", zap.Error(err))
		return
	}

	// Extract symbol and timeframe from topic
	// Topic format: "kline.5.BTCUSDT"
	symbol, timeframe := parseBybitTopic(msg.Topic)
	if symbol == "" || timeframe == "" {
		return
	}

	for _, kline := range klines {
		// Only process confirmed candles
		if !kline.Confirm {
			continue
		}

		candle := models.Candle{
			Symbol:      symbol,
			Timeframe:   timeframe,
			Timestamp:   time.UnixMilli(kline.Start),
			Open:        models.NewDecimalFromString(kline.Open),
			High:        models.NewDecimalFromString(kline.High),
			Low:         models.NewDecimalFromString(kline.Low),
			Close:       models.NewDecimalFromString(kline.Close),
			Volume:      models.NewDecimalFromString(kline.Volume),
			QuoteVolume: models.NewDecimalFromString(kline.Turnover),
			Trades:      0, // Not provided in WebSocket
		}

		select {
		case bw.candleChan <- candle:
		default:
			logger.Warn("candle channel full, dropping candle")
		}
	}
}

// pingHandler sends periodic ping messages
func (bw *BybitWebSocket) pingHandler() {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-bw.ctx.Done():
			return
		case <-ticker.C:
			bw.mu.Lock()
			if bw.conn != nil {
				ping := map[string]interface{}{
					"op": "ping",
				}
				if err := bw.conn.WriteJSON(ping); err != nil {
					logger.Error("failed to send ping", zap.Error(err))
				}
			}
			bw.mu.Unlock()
		}
	}
}

// Candles returns channel for receiving candles
func (bw *BybitWebSocket) Candles() <-chan models.Candle {
	return bw.candleChan
}

// Errors returns channel for receiving errors
func (bw *BybitWebSocket) Errors() <-chan error {
	return bw.errorChan
}

// Close closes WebSocket connection
func (bw *BybitWebSocket) Close() error {
	bw.cancel()

	bw.mu.Lock()
	defer bw.mu.Unlock()

	if bw.conn != nil {
		return bw.conn.Close()
	}

	return nil
}

// Helper functions

func convertSymbolToBybit(symbol string) string {
	// BTC/USDT -> BTCUSDT
	return symbol[:3] + symbol[4:]
}

func parseBybitTopic(topic string) (symbol, timeframe string) {
	// Topic format: "kline.5.BTCUSDT"
	// Extract symbol and convert interval back to standard format
	var interval, bybitSymbol string
	fmt.Sscanf(topic, "kline.%s.%s", &interval, &bybitSymbol)

	// Convert BTCUSDT -> BTC/USDT
	if len(bybitSymbol) >= 6 {
		symbol = bybitSymbol[:3] + "/" + bybitSymbol[3:]
	}

	// Convert Bybit interval to standard timeframe
	intervalMap := map[string]string{
		"1":   "1m",
		"5":   "5m",
		"15":  "15m",
		"60":  "1h",
		"240": "4h",
		"D":   "1d",
	}

	timeframe = intervalMap[interval]

	return symbol, timeframe
}
