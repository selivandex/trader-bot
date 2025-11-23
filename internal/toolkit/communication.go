package toolkit

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/selivandex/trader-bot/pkg/logger"
)

// ============ Communication Tools Implementation ============

// SendUrgentAlert sends urgent message to agent owner via Telegram
func (t *LocalToolkit) SendUrgentAlert(ctx context.Context, message string, priority string) error {
	logger.Info("🚨 toolkit: send_urgent_alert",
		zap.String("agent_id", t.agentID),
		zap.String("priority", priority),
		zap.String("message_preview", truncate(message, 50)),
	)

	// Get agent config to find user ID
	agent, err := t.agentRepo.GetAgent(ctx, t.agentID)
	if err != nil {
		return fmt.Errorf("failed to get agent config: %w", err)
	}

	// Check if notifier available
	if t.notifier == nil {
		logger.Warn("notifier not available, alert not sent",
			zap.String("agent_id", t.agentID),
			zap.String("message", message),
		)
		return fmt.Errorf("notifier not configured")
	}

	// Format message with agent name and priority
	formattedMsg := fmt.Sprintf("🤖 *%s* [%s]\n\n%s", agent.Name, priority, message)

	// Send via notifier's error alert (reusing existing infrastructure)
	if err := t.notifier.SendErrorAlert(ctx, agent.UserID, agent.Name, formattedMsg); err != nil {
		return fmt.Errorf("failed to send alert: %w", err)
	}

	logger.Info("✅ urgent alert sent to owner",
		zap.String("agent_id", t.agentID),
		zap.String("user_id", agent.UserID),
	)

	return nil
}

// LogThought logs agent's internal reasoning (for debugging/transparency)
func (t *LocalToolkit) LogThought(ctx context.Context, thought string, confidence float64) error {
	logger.Debug("💭 toolkit: log_thought",
		zap.String("agent_id", t.agentID),
		zap.Float64("confidence", confidence),
		zap.String("thought", truncate(thought, 100)),
	)

	// Store thought in reasoning log (could be database table in future)
	// For now, just log it
	logger.Info("agent thought",
		zap.String("agent_id", t.agentID),
		zap.String("thought", thought),
		zap.Float64("confidence", confidence),
	)

	return nil
}

// RequestHumanInput requests input from owner (advanced feature for future)
func (t *LocalToolkit) RequestHumanInput(ctx context.Context, question string, options []string) (string, error) {
	logger.Info("❓ toolkit: request_human_input",
		zap.String("agent_id", t.agentID),
		zap.String("question", question),
		zap.Strings("options", options),
	)

	// TODO: Implement interactive telegram bot conversation
	// For now, return empty (agent continues without human input)

	logger.Warn("human input requested but not implemented yet",
		zap.String("agent_id", t.agentID),
		zap.String("question", question),
	)

	return "", fmt.Errorf("human input not implemented")
}
