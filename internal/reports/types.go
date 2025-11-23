package reports

import (
	"time"

	"github.com/selivandex/trader-bot/pkg/models"
)

// DailyReport contains comprehensive daily trading report
type DailyReport struct {
	AgentID     string
	AgentName   string
	Symbol      string
	Date        time.Time
	Period      Period
	Metrics     *DayMetrics
	Decisions   []models.AgentDecision
	Insights    []string
	State       *models.AgentState
	GeneratedAt time.Time
}

// WeeklyReport contains weekly summary
type WeeklyReport struct {
	AgentID      string
	Symbol       string
	WeekStart    time.Time
	WeekEnd      time.Time
	DailyReports []*DailyReport
	WeekMetrics  *WeekMetrics
	GeneratedAt  time.Time
}

// CustomReport for arbitrary time period
type CustomReport struct {
	AgentID     string
	Symbol      string
	Period      Period
	Metrics     *PeriodMetrics
	Decisions   []models.AgentDecision
	GeneratedAt time.Time
}

// Period represents time range
type Period struct {
	Start time.Time
	End   time.Time
}

// DayMetrics contains metrics for one trading day
type DayMetrics struct {
	TotalDecisions      int
	ExecutedTrades      int
	HoldCount           int
	LongCount           int
	ShortCount          int
	CloseCount          int
	HighConfidenceCount int
	WinningTrades       int
	LosingTrades        int
	WinRate             float64
	TotalPnL            float64
	BestTrade           float64
	WorstTrade          float64
	StartBalance        float64
	EndBalance          float64
	StartEquity         float64
	EndEquity           float64
	DailyReturn         float64
}

// WeekMetrics contains aggregated weekly metrics
type WeekMetrics struct {
	TradingDays    int
	TotalDecisions int
	ExecutedTrades int
	WinningTrades  int
	LosingTrades   int
	WinRate        float64
	TotalPnL       float64
	BestDay        float64
	WorstDay       float64
	StartBalance   float64
	EndBalance     float64
	WeeklyReturn   float64
	LongCount      int
	ShortCount     int
	SharpeRatio    float64
}

// PeriodMetrics for custom periods
type PeriodMetrics struct {
	Duration       time.Duration
	TotalDecisions int
	ExecutedTrades int
	TotalPnL       float64
	WinRate        float64
}

// ReportFormat specifies output format
type ReportFormat string

const (
	FormatText     ReportFormat = "text"
	FormatMarkdown ReportFormat = "markdown"
	FormatJSON     ReportFormat = "json"
	FormatHTML     ReportFormat = "html"
)

// ReportRequest contains parameters for report generation
type ReportRequest struct {
	AgentID string
	Symbol  string
	Period  ReportPeriod
	Format  ReportFormat
	// For custom period
	StartDate *time.Time
	EndDate   *time.Time
}

// ReportPeriod specifies time period for report
type ReportPeriod string

const (
	PeriodToday     ReportPeriod = "today"
	PeriodYesterday ReportPeriod = "yesterday"
	PeriodWeek      ReportPeriod = "week"
	PeriodMonth     ReportPeriod = "month"
	PeriodCustom    ReportPeriod = "custom"
)
