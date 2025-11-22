package sentiment

import (
	"strings"

	"github.com/selivandex/trader-bot/pkg/models"
)

// ImpactScorer scores news impact on market
type ImpactScorer struct {
	highImpactKeywords map[string]int
	urgencyKeywords    map[string]string
}

// NewImpactScorer creates new impact scorer
func NewImpactScorer() *ImpactScorer {
	return &ImpactScorer{
		highImpactKeywords: buildHighImpactKeywords(),
		urgencyKeywords:    buildUrgencyKeywords(),
	}
}

// ScoreImpact calculates impact score (1-10) and urgency for news
func (is *ImpactScorer) ScoreImpact(title, content string) (impact int, urgency string) {
	text := strings.ToLower(title + " " + content)
	
	impact = 5 // Default medium impact
	urgency = "HOURS" // Default
	
	// Check for high impact keywords
	maxImpact := 5
	for keyword, score := range is.highImpactKeywords {
		if strings.Contains(text, keyword) {
			if score > maxImpact {
				maxImpact = score
			}
		}
	}
	
	impact = maxImpact
	
	// Determine urgency
	for keyword, urg := range is.urgencyKeywords {
		if strings.Contains(text, keyword) {
			// IMMEDIATE overrides HOURS overrides DAYS
			if urg == "IMMEDIATE" || (urgency != "IMMEDIATE" && urg == "HOURS") {
				urgency = urg
			}
		}
	}
	
	return impact, urgency
}

// ScoreNewsItem scores complete news item
func (is *ImpactScorer) ScoreNewsItem(item *models.NewsItem) {
	impact, urgency := is.ScoreImpact(item.Title, item.Content)
	item.Impact = impact
	item.Urgency = urgency
}

// buildHighImpactKeywords returns keywords with impact scores
func buildHighImpactKeywords() map[string]int {
	return map[string]int{
		// 10/10 - Market-moving events
		"etf approval":              10,
		"etf approved":              10,
		"sec approves":              10,
		"country adopts":            10,
		"legal tender":              10,
		"major exchange hack":       10,
		"exchange hacked":           10,
		"billion dollar":            10,
		
		// 9/10 - Highly significant
		"institutional buying":      9,
		"microstrategy buys":        9,
		"tesla buys":                9,
		"blackrock":                 9,
		"fidelity":                  9,
		"grayscale":                 9,
		"spot etf":                  9,
		"halving":                   9,
		
		// 8/10 - Very significant
		"regulation":                8,
		"sec lawsuit":               8,
		"government ban":            8,
		"major partnership":         8,
		"institutional adoption":    8,
		"central bank":              8,
		"federal reserve":           8,
		
		// 7/10 - Significant
		"exchange listing":          7,
		"upgrade":                   7,
		"hard fork":                 7,
		"whale movement":            7,
		"large transaction":         7,
		"institutional interest":    7,
		
		// 6/10 - Notable
		"analyst prediction":        6,
		"price target":              6,
		"technical analysis":        6,
		"on-chain metrics":          6,
		
		// 5/10 - Standard
		"market update":             5,
		"price analysis":            5,
		
		// 3/10 - Low impact
		"opinion":                   3,
		"speculation":               3,
	}
}

// buildUrgencyKeywords returns keywords mapping to urgency levels
func buildUrgencyKeywords() map[string]string {
	return map[string]string{
		// IMMEDIATE (affects price within minutes/hours)
		"breaking":       "IMMEDIATE",
		"just announced": "IMMEDIATE",
		"just in":        "IMMEDIATE",
		"alert":          "IMMEDIATE",
		"emergency":      "IMMEDIATE",
		"hack":           "IMMEDIATE",
		"exploit":        "IMMEDIATE",
		"approved":       "IMMEDIATE",
		"rejected":       "IMMEDIATE",
		
		// HOURS (affects within 4-24 hours)
		"scheduled":      "HOURS",
		"upcoming":       "HOURS",
		"expected":       "HOURS",
		"announcement":   "HOURS",
		
		// DAYS (gradual impact)
		"regulation":     "DAYS",
		"policy":         "DAYS",
		"proposal":       "DAYS",
		"roadmap":        "DAYS",
		"long-term":      "DAYS",
	}
}

