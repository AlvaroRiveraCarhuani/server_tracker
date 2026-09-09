package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/alvaroriverac/server_tracker_agent/internal/core/domain"
)

// CatalogProvider define la interfaz para consultar dinámicamente modelos de un proveedor.
type CatalogProvider interface {
	ID() domain.AIProvider
	FetchModels(ctx context.Context, pCfg domain.ProviderConfig) ([]domain.AIModel, error)
}

// OpenRouterCatalogProvider consulta el catálogo público y dinámico de OpenRouter.
type OpenRouterCatalogProvider struct {
	client *http.Client
	apiURL string
}

// NewOpenRouterCatalogProvider inicializa el proveedor para OpenRouter.
func NewOpenRouterCatalogProvider(client *http.Client, apiURL ...string) *OpenRouterCatalogProvider {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	url := "https://openrouter.ai/api/v1/models"
	if len(apiURL) > 0 && apiURL[0] != "" {
		url = apiURL[0]
	}
	return &OpenRouterCatalogProvider{client: client, apiURL: url}
}

func (p *OpenRouterCatalogProvider) ID() domain.AIProvider {
	return domain.ProviderOpenRouter
}

type openRouterModelsResponse struct {
	Data []struct {
		ID            string `json:"id"`
		Name          string `json:"name"`
		Description   string `json:"description"`
		ContextLength int64  `json:"context_length"`
		Architecture  struct {
			Modality     string `json:"modality"`
			InstructType string `json:"instruct_type"`
		} `json:"architecture"`
		Pricing struct {
			Prompt     string `json:"prompt"`
			Completion string `json:"completion"`
		} `json:"pricing"`
	} `json:"data"`
}

func (p *OpenRouterCatalogProvider) FetchModels(ctx context.Context, pCfg domain.ProviderConfig) ([]domain.AIModel, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", p.apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creando request a OpenRouter: %w", err)
	}

	if pCfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+pCfg.APIKey)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error llamando a OpenRouter models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenRouter respondió con status %d", resp.StatusCode)
	}

	var orResp openRouterModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&orResp); err != nil {
		return nil, fmt.Errorf("error decodificando json de OpenRouter: %w", err)
	}

	var models []domain.AIModel

	// 1. Siempre incluir el meta-router openrouter/free como primera opción destacada
	models = append(models, domain.AIModel{
		ID:            "openrouter/free",
		DisplayName:   "Free Router (openrouter/free)",
		ProviderID:    domain.ProviderOpenRouter,
		Creator:       "OpenRouter",
		ContextLength: 131072,
		Pricing: domain.ModelPricing{
			IsFree:      true,
			Tier:        domain.PricingFree,
			RateLimited: true,
		},
		Capabilities: domain.ModelCapabilities{
			StructuredOutput: true,
			Tools:            true,
			Reasoning:        false,
		},
		IsRouter: true,
		Source:   domain.SourceRemote,
		Verified: true,
	})

	for _, item := range orResp.Data {
		// Descartar modelos que no sean de texto/instrucción
		if strings.Contains(strings.ToLower(item.Architecture.Modality), "image->image") ||
			strings.Contains(strings.ToLower(item.ID), "embedding") {
			continue
		}

		creator := "OpenRouter"
		parts := strings.Split(item.ID, "/")
		if len(parts) > 1 {
			creator = strings.Title(parts[0])
		}

		var promptPrice, completionPrice float64
		fmt.Sscanf(item.Pricing.Prompt, "%f", &promptPrice)
		fmt.Sscanf(item.Pricing.Completion, "%f", &completionPrice)

		// Precios vienen por token en OpenRouter, convertimos a USD por 1M tokens
		inM := promptPrice * 1_000_000.0
		outM := completionPrice * 1_000_000.0

		isFree := (promptPrice == 0 && completionPrice == 0) || strings.HasSuffix(item.ID, ":free")
		tier := domain.PricingLow
		if isFree {
			tier = domain.PricingFree
		} else if inM >= 2.0 || outM >= 5.0 {
			tier = domain.PricingPremium
		}

		displayName := item.Name
		if displayName == "" {
			displayName = item.ID
		}

		isReasoning := strings.Contains(strings.ToLower(item.ID), "r1") ||
			strings.Contains(strings.ToLower(item.ID), "o1") ||
			strings.Contains(strings.ToLower(item.ID), "reason")

		ctxLen := item.ContextLength
		if ctxLen <= 0 {
			ctxLen = 8192
		}

		models = append(models, domain.AIModel{
			ID:            item.ID,
			DisplayName:   displayName,
			ProviderID:    domain.ProviderOpenRouter,
			Creator:       creator,
			ContextLength: ctxLen,
			Pricing: domain.ModelPricing{
				InputPerMillion:  inM,
				OutputPerMillion: outM,
				IsFree:           isFree,
				Tier:             tier,
				RateLimited:      isFree,
			},
			Capabilities: domain.ModelCapabilities{
				StructuredOutput: true,
				Tools:            true,
				Reasoning:        isReasoning,
			},
			IsRouter: strings.HasSuffix(item.ID, "/auto") || strings.HasSuffix(item.ID, "/free"),
			Source:   domain.SourceRemote,
			Verified: true,
		})
	}

	return models, nil
}

// OllamaCatalogProvider consulta los modelos efectivamente descargados en el runtime local de Ollama.
type OllamaCatalogProvider struct {
	client   *http.Client
	endpoint string
}

// NewOllamaCatalogProvider inicializa el proveedor para Ollama local.
func NewOllamaCatalogProvider(client *http.Client, endpoint ...string) *OllamaCatalogProvider {
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Second}
	}
	ep := "http://localhost:11434"
	if len(endpoint) > 0 && endpoint[0] != "" {
		ep = endpoint[0]
	}
	return &OllamaCatalogProvider{client: client, endpoint: ep}
}

func (p *OllamaCatalogProvider) ID() domain.AIProvider {
	return domain.ProviderOllama
}

type ollamaTagsResponse struct {
	Models []struct {
		Name       string `json:"name"`
		Model      string `json:"model"`
		ModifiedAt string `json:"modified_at"`
		Size       int64  `json:"size"`
		Details    struct {
			Family            string `json:"family"`
			ParameterSize     string `json:"parameter_size"`
			QuantizationLevel string `json:"quantization_level"`
		} `json:"details"`
	} `json:"models"`
}

func (p *OllamaCatalogProvider) FetchModels(ctx context.Context, pCfg domain.ProviderConfig) ([]domain.AIModel, error) {
	ep := p.endpoint
	if pCfg.Endpoint != "" {
		ep = pCfg.Endpoint
	}
	url := strings.TrimSuffix(ep, "/") + "/api/tags"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creando request a Ollama: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error llamando a Ollama /api/tags: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Ollama respondió con status %d", resp.StatusCode)
	}

	var tagsResp ollamaTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tagsResp); err != nil {
		return nil, fmt.Errorf("error decodificando tags de Ollama: %w", err)
	}

	var models []domain.AIModel
	for _, m := range tagsResp.Models {
		displayName := m.Name
		creator := "Local"
		family := strings.ToLower(m.Details.Family)
		if strings.Contains(family, "llama") {
			creator = "Meta"
		} else if strings.Contains(family, "qwen") {
			creator = "Alibaba"
		} else if strings.Contains(family, "mistral") {
			creator = "Mistral"
		}

		isReasoning := strings.Contains(strings.ToLower(m.Name), "r1") || strings.Contains(strings.ToLower(m.Name), "deepseek")

		models = append(models, domain.AIModel{
			ID:            m.Name,
			DisplayName:   displayName,
			ProviderID:    domain.ProviderOllama,
			Creator:       creator,
			ContextLength: 131072,
			Pricing: domain.ModelPricing{
				IsFree: true,
				Tier:   domain.PricingFree,
			},
			Capabilities: domain.ModelCapabilities{
				StructuredOutput: true,
				Tools:            false,
				Reasoning:        isReasoning,
			},
			IsRouter: false,
			Source:   domain.SourceLocal,
			Verified: true,
		})
	}

	return models, nil
}
