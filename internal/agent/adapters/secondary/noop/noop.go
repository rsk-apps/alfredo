package noop

import (
	"context"

	"go.uber.org/zap"

	"github.com/rafaelsoares/alfredo/internal/agent/port"
)

const Reply = "O agente está desativado neste ambiente. Configure a chave da Anthropic para usar comandos por voz."

type Adapter struct {
	logger *zap.Logger
}

func NewAdapter(logger *zap.Logger) *Adapter {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Adapter{logger: logger}
}

func (a *Adapter) Complete(_ context.Context, _ port.LLMInput) (port.LLMOutput, error) {
	a.logger.Info("agent noop llm called")
	return port.LLMOutput{FinalText: Reply}, nil
}
