package reports

import (
	"time"

	"github.com/selivandex/trader-bot/pkg/models"
)

// DailyReport contains comprehensive daily trading report
type DailyReport struct {
	Period      Period
	Date        time.Time
	GeneratedAt time.Time
	Metrics     *DayMetrics
	State       *models.AgentState
	AgentID     string
	AgentName   string
	Symbol      string
	Decisions   []models.AgentDecision
	Insights    []string
}

// WeeklyReport contains weekly summary
type WeeklyReport struct {
	WeekStart    time.Time
	WeekEnd      time.Time
	GeneratedAt  time.Time
	WeekMetrics  *WeekMetrics
	AgentID      string
	Symbol       string
	DailyReports []*DailyReport
}

// CustomReport for arbitrary time period
type CustomReport struct {
	Period      Period
	GeneratedAt time.Time
	Metrics     *PeriodMetrics
	AgentID     string
	Symbol      string
	Decisions   []models.AgentDecision
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
	StartDate *time.Time
	EndDate   *time.Time
	AgentID   string
	Symbol    string
	Period    ReportPeriod
	Format    ReportFormat
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
