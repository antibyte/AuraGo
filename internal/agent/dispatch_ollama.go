package agent

import (
	"log/slog"

	"aurago/internal/config"
	"aurago/internal/tools"
)

func dispatchOllama(cfg *config.Config, req ollamaArgs, logger *slog.Logger) string {
	if !cfg.Ollama.Enabled {
		return `Tool Output: {"status":"error","message":"Ollama integration is not enabled. Set ollama.enabled=true in config.yaml."}`
	}
	if cfg.Ollama.ReadOnly {
		switch req.Operation {
		case "pull", "download", "delete", "remove", "copy", "load", "unload":
			return `Tool Output: {"status":"error","message":"Ollama is in read-only mode. Disable ollama.readonly to allow changes."}`
		}
	}
	olCfg := tools.OllamaConfig{URL: cfg.Ollama.URL, ReadOnly: cfg.Ollama.ReadOnly}
	modelName := req.modelName()
	switch req.Operation {
	case "list", "list_models":
		logger.Info("LLM requested Ollama list models")
		return "Tool Output: " + tools.OllamaListModels(olCfg)
	case "running", "ps":
		logger.Info("LLM requested Ollama list running")
		return "Tool Output: " + tools.OllamaListRunning(olCfg)
	case "show", "info":
		logger.Info("LLM requested Ollama show model", "model", modelName)
		return "Tool Output: " + tools.OllamaShowModel(olCfg, modelName)
	case "pull", "download":
		logger.Info("LLM requested Ollama pull model", "model", modelName)
		return "Tool Output: " + tools.OllamaPullModel(olCfg, modelName)
	case "delete", "remove":
		logger.Info("LLM requested Ollama delete model", "model", modelName)
		return "Tool Output: " + tools.OllamaDeleteModel(olCfg, modelName)
	case "copy":
		src := req.Source
		dst := req.destinationName()
		logger.Info("LLM requested Ollama copy model", "source", src, "destination", dst)
		return "Tool Output: " + tools.OllamaCopyModel(olCfg, src, dst)
	case "load":
		logger.Info("LLM requested Ollama load model", "model", modelName)
		return "Tool Output: " + tools.OllamaLoadModel(olCfg, modelName)
	case "unload":
		logger.Info("LLM requested Ollama unload model", "model", modelName)
		return "Tool Output: " + tools.OllamaUnloadModel(olCfg, modelName)
	default:
		return `Tool Output: {"status":"error","message":"Unknown ollama operation. Use: list, running, show, pull, delete, copy, load, unload"}`
	}
}
