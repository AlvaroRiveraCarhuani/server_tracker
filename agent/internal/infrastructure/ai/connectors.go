package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/alvaroriverac/server_tracker_agent/internal/core/domain"
)

// Connector defines the lifecycle and parsing operations for an AI provider.
type Connector interface {
	ID() domain.AIProvider
	Name() string
	Family() domain.ConnectorFamily
	DefaultURL() string
	Health(ctx context.Context, endpoint string, apiKey string) (domain.ConnectorStatus, error)
	ModelsEndpoint(endpoint string) string
	ParseModels(body []byte) ([]domain.ModelRef, error)
}

// OpenRouterConnector implements Connector for OpenRouter.
type OpenRouterConnector struct {
	client *http.Client
}

func NewOpenRouterConnector(c *http.Client) *OpenRouterConnector {
	if c == nil {
		c = &http.Client{Timeout: 3 * time.Second}
	}
	return &OpenRouterConnector{client: c}
}

func (c *OpenRouterConnector) ID() domain.AIProvider         { return domain.ProviderOpenRouter }
func (c *OpenRouterConnector) Name() string                  { return "OpenRouter" }
func (c *OpenRouterConnector) Family() domain.ConnectorFamily { return domain.FamilyCloudManaged }
func (c *OpenRouterConnector) DefaultURL() string            { return "https://openrouter.ai/api/v1" }

func (c *OpenRouterConnector) Health(ctx context.Context, endpoint, apiKey string) (domain.ConnectorStatus, error) {
	if strings.TrimSpace(apiKey) == "" {
		return domain.StatusNoKey, nil
	}
	return domain.StatusConnected, nil
}

func (c *OpenRouterConnector) ModelsEndpoint(endpoint string) string {
	if endpoint == "" {
		endpoint = c.DefaultURL()
	}
	return strings.TrimSuffix(endpoint, "/") + "/models"
}

func (c *OpenRouterConnector) ParseModels(body []byte) ([]domain.ModelRef, error) {
	var resp struct {
		Data []struct {
			ID            string `json:"id"`
			Name          string `json:"name"`
			ContextLength int64  `json:"context_length"`
			Pricing       struct {
				Prompt     string `json:"prompt"`
				Completion string `json:"completion"`
			} `json:"pricing"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("error parsing OpenRouter models: %w", err)
	}

	var models []domain.ModelRef
	hasFreeRouter := false

	for _, item := range resp.Data {
		isFree := item.Pricing.Prompt == "0" && item.Pricing.Completion == "0"
		tier := domain.PricingLow
		if isFree {
			tier = domain.PricingFree
		}
		if item.ID == "openrouter/free" {
			hasFreeRouter = true
			isFree = true
			tier = domain.PricingFree
		}

		dispName := item.Name
		if dispName == "" {
			dispName = item.ID
		}

		models = append(models, domain.ModelRef{
			ID:            item.ID,
			DisplayName:   dispName,
			ProviderID:    domain.ProviderOpenRouter,
			ContextLength: item.ContextLength,
			IsFree:        isFree,
			PricingTier:   tier,
			EvalOK:        true,
		})
	}

	// Always ensure openrouter/free router is present at the front
	if !hasFreeRouter {
		freeRouter := domain.ModelRef{
			ID:            "openrouter/free",
			DisplayName:   "Free Router (openrouter/free)",
			ProviderID:    domain.ProviderOpenRouter,
			ContextLength: 131072,
			IsFree:        true,
			PricingTier:   domain.PricingFree,
			EvalOK:        true,
		}
		models = append([]domain.ModelRef{freeRouter}, models...)
	}

	return models, nil
}

// OpenAIConnector implements Connector for OpenAI.
type OpenAIConnector struct {
	client *http.Client
}

func NewOpenAIConnector(c *http.Client) *OpenAIConnector {
	if c == nil {
		c = &http.Client{Timeout: 3 * time.Second}
	}
	return &OpenAIConnector{client: c}
}

func (c *OpenAIConnector) ID() domain.AIProvider         { return domain.ProviderOpenAI }
func (c *OpenAIConnector) Name() string                  { return "OpenAI" }
func (c *OpenAIConnector) Family() domain.ConnectorFamily { return domain.FamilyCloudManaged }
func (c *OpenAIConnector) DefaultURL() string            { return "https://api.openai.com/v1" }

func (c *OpenAIConnector) Health(ctx context.Context, endpoint, apiKey string) (domain.ConnectorStatus, error) {
	if strings.TrimSpace(apiKey) == "" {
		return domain.StatusNoKey, nil
	}
	return domain.StatusConnected, nil
}

func (c *OpenAIConnector) ModelsEndpoint(endpoint string) string {
	if endpoint == "" {
		endpoint = c.DefaultURL()
	}
	return strings.TrimSuffix(endpoint, "/") + "/models"
}

func (c *OpenAIConnector) ParseModels(body []byte) ([]domain.ModelRef, error) {
	var resp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("error parsing OpenAI models: %w", err)
	}

	var models []domain.ModelRef
	for _, item := range resp.Data {
		if !strings.HasPrefix(item.ID, "gpt-") && !strings.HasPrefix(item.ID, "o1") && !strings.HasPrefix(item.ID, "o3") {
			continue
		}
		models = append(models, domain.ModelRef{
			ID:            item.ID,
			DisplayName:   item.ID,
			ProviderID:    domain.ProviderOpenAI,
			ContextLength: 128000,
			IsFree:        false,
			PricingTier:   domain.PricingLow,
			EvalOK:        true,
		})
	}
	return models, nil
}

// AnthropicConnector implements Connector for Anthropic.
type AnthropicConnector struct {
	client *http.Client
}

func NewAnthropicConnector(c *http.Client) *AnthropicConnector {
	if c == nil {
		c = &http.Client{Timeout: 3 * time.Second}
	}
	return &AnthropicConnector{client: c}
}

func (c *AnthropicConnector) ID() domain.AIProvider         { return domain.ProviderAnthropic }
func (c *AnthropicConnector) Name() string                  { return "Anthropic" }
func (c *AnthropicConnector) Family() domain.ConnectorFamily { return domain.FamilyCloudManaged }
func (c *AnthropicConnector) DefaultURL() string            { return "https://api.anthropic.com/v1" }

func (c *AnthropicConnector) Health(ctx context.Context, endpoint, apiKey string) (domain.ConnectorStatus, error) {
	if strings.TrimSpace(apiKey) == "" {
		return domain.StatusNoKey, nil
	}
	return domain.StatusConnected, nil
}

func (c *AnthropicConnector) ModelsEndpoint(endpoint string) string {
	if endpoint == "" {
		endpoint = c.DefaultURL()
	}
	return strings.TrimSuffix(endpoint, "/") + "/models"
}

func (c *AnthropicConnector) ParseModels(body []byte) ([]domain.ModelRef, error) {
	// Standard verified Anthropic models
	return []domain.ModelRef{
		{
			ID:            "claude-3-5-haiku-latest",
			DisplayName:   "Claude 3.5 Haiku",
			ProviderID:    domain.ProviderAnthropic,
			ContextLength: 200000,
			IsFree:        false,
			PricingTier:   domain.PricingLow,
			EvalOK:        true,
		},
		{
			ID:            "claude-3-5-sonnet-latest",
			DisplayName:   "Claude 3.5 Sonnet",
			ProviderID:    domain.ProviderAnthropic,
			ContextLength: 200000,
			IsFree:        false,
			PricingTier:   domain.PricingPremium,
			EvalOK:        true,
		},
	}, nil
}

// OllamaConnector implements Connector for Ollama local runtime.
type OllamaConnector struct {
	client *http.Client
}

func NewOllamaConnector(c *http.Client) *OllamaConnector {
	if c == nil {
		c = &http.Client{Timeout: 750 * time.Millisecond}
	}
	return &OllamaConnector{client: c}
}

func (c *OllamaConnector) ID() domain.AIProvider         { return domain.ProviderOllama }
func (c *OllamaConnector) Name() string                  { return "Ollama" }
func (c *OllamaConnector) Family() domain.ConnectorFamily { return domain.FamilyLocalRuntime }
func (c *OllamaConnector) DefaultURL() string            { return "http://localhost:11434" }

func (c *OllamaConnector) Health(ctx context.Context, endpoint, apiKey string) (domain.ConnectorStatus, error) {
	if endpoint == "" {
		endpoint = c.DefaultURL()
	}
	reqURL := strings.TrimSuffix(endpoint, "/") + "/api/tags"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return domain.StatusAbsent, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return domain.StatusTimeout, nil
		}
		return domain.StatusAbsent, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return domain.StatusDetected, nil
	}
	return domain.StatusAbsent, nil
}

func (c *OllamaConnector) ModelsEndpoint(endpoint string) string {
	if endpoint == "" {
		endpoint = c.DefaultURL()
	}
	return strings.TrimSuffix(endpoint, "/") + "/api/tags"
}

func (c *OllamaConnector) ParseModels(body []byte) ([]domain.ModelRef, error) {
	var resp struct {
		Models []struct {
			Name    string `json:"name"`
			Model   string `json:"model"`
			Details struct {
				ParameterSize     string `json:"parameter_size"`
				QuantizationLevel string `json:"quantization_level"`
			} `json:"details"`
		} `json:"models"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("error parsing Ollama models: %w", err)
	}

	var models []domain.ModelRef
	for _, m := range resp.Models {
		name := m.Name
		if name == "" {
			name = m.Model
		}
		models = append(models, domain.ModelRef{
			ID:            name,
			DisplayName:   name,
			ProviderID:    domain.ProviderOllama,
			ContextLength: 131072,
			IsFree:        true,
			PricingTier:   domain.PricingFree,
			EvalOK:        true,
		})
	}
	return models, nil
}

// VLLMConnector implements Connector for vLLM local runtime.
type VLLMConnector struct {
	client *http.Client
}

func NewVLLMConnector(c *http.Client) *VLLMConnector {
	if c == nil {
		c = &http.Client{Timeout: 750 * time.Millisecond}
	}
	return &VLLMConnector{client: c}
}

func (c *VLLMConnector) ID() domain.AIProvider         { return domain.ProviderVLLM }
func (c *VLLMConnector) Name() string                  { return "vLLM" }
func (c *VLLMConnector) Family() domain.ConnectorFamily { return domain.FamilyLocalRuntime }
func (c *VLLMConnector) DefaultURL() string            { return "http://localhost:8000" }

func (c *VLLMConnector) Health(ctx context.Context, endpoint, apiKey string) (domain.ConnectorStatus, error) {
	if endpoint == "" {
		endpoint = c.DefaultURL()
	}
	reqURL := strings.TrimSuffix(endpoint, "/") + "/v1/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return domain.StatusAbsent, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return domain.StatusTimeout, nil
		}
		return domain.StatusAbsent, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return domain.StatusDetected, nil
	}
	return domain.StatusAbsent, nil
}

func (c *VLLMConnector) ModelsEndpoint(endpoint string) string {
	if endpoint == "" {
		endpoint = c.DefaultURL()
	}
	return strings.TrimSuffix(endpoint, "/") + "/v1/models"
}

func (c *VLLMConnector) ParseModels(body []byte) ([]domain.ModelRef, error) {
	var resp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("error parsing vLLM models: %w", err)
	}

	var models []domain.ModelRef
	for _, m := range resp.Data {
		models = append(models, domain.ModelRef{
			ID:            m.ID,
			DisplayName:   m.ID,
			ProviderID:    domain.ProviderVLLM,
			ContextLength: 65536,
			IsFree:        true,
			PricingTier:   domain.PricingFree,
			EvalOK:        true,
		})
	}
	return models, nil
}

// LMStudioConnector implements Connector for LM Studio local runtime.
type LMStudioConnector struct {
	client *http.Client
}

func NewLMStudioConnector(c *http.Client) *LMStudioConnector {
	if c == nil {
		c = &http.Client{Timeout: 750 * time.Millisecond}
	}
	return &LMStudioConnector{client: c}
}

func (c *LMStudioConnector) ID() domain.AIProvider         { return domain.ProviderLMStudio }
func (c *LMStudioConnector) Name() string                  { return "LM Studio" }
func (c *LMStudioConnector) Family() domain.ConnectorFamily { return domain.FamilyLocalRuntime }
func (c *LMStudioConnector) DefaultURL() string            { return "http://localhost:1234" }

func (c *LMStudioConnector) Health(ctx context.Context, endpoint, apiKey string) (domain.ConnectorStatus, error) {
	if endpoint == "" {
		endpoint = c.DefaultURL()
	}
	reqURL := strings.TrimSuffix(endpoint, "/") + "/v1/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return domain.StatusAbsent, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return domain.StatusTimeout, nil
		}
		return domain.StatusAbsent, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return domain.StatusDetected, nil
	}
	return domain.StatusAbsent, nil
}

func (c *LMStudioConnector) ModelsEndpoint(endpoint string) string {
	if endpoint == "" {
		endpoint = c.DefaultURL()
	}
	return strings.TrimSuffix(endpoint, "/") + "/v1/models"
}

func (c *LMStudioConnector) ParseModels(body []byte) ([]domain.ModelRef, error) {
	var resp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("error parsing LM Studio models: %w", err)
	}

	var models []domain.ModelRef
	for _, m := range resp.Data {
		models = append(models, domain.ModelRef{
			ID:            m.ID,
			DisplayName:   m.ID,
			ProviderID:    domain.ProviderLMStudio,
			ContextLength: 65536,
			IsFree:        true,
			PricingTier:   domain.PricingFree,
			EvalOK:        true,
		})
	}
	return models, nil
}

// CustomOpenAIConnector implements Connector for custom user-configured endpoints.
type CustomOpenAIConnector struct {
	client *http.Client
}

func NewCustomOpenAIConnector(c *http.Client) *CustomOpenAIConnector {
	if c == nil {
		c = &http.Client{Timeout: 1500 * time.Millisecond}
	}
	return &CustomOpenAIConnector{client: c}
}

func (c *CustomOpenAIConnector) ID() domain.AIProvider         { return domain.ProviderCustom }
func (c *CustomOpenAIConnector) Name() string                  { return "Custom OpenAI" }
func (c *CustomOpenAIConnector) Family() domain.ConnectorFamily { return domain.FamilyCustomOpenAI }
func (c *CustomOpenAIConnector) DefaultURL() string            { return "http://localhost:8080/v1" }

func (c *CustomOpenAIConnector) Health(ctx context.Context, endpoint, apiKey string) (domain.ConnectorStatus, error) {
	if strings.TrimSpace(endpoint) == "" {
		return domain.StatusAbsent, nil
	}
	reqURL := strings.TrimSuffix(endpoint, "/") + "/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return domain.StatusAbsent, err
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return domain.StatusTimeout, nil
		}
		return domain.StatusAbsent, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return domain.StatusConnected, nil
	}
	return domain.StatusAbsent, nil
}

func (c *CustomOpenAIConnector) ModelsEndpoint(endpoint string) string {
	if endpoint == "" {
		endpoint = c.DefaultURL()
	}
	return strings.TrimSuffix(endpoint, "/") + "/models"
}

func (c *CustomOpenAIConnector) ParseModels(body []byte) ([]domain.ModelRef, error) {
	var resp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("error parsing custom endpoint models: %w", err)
	}

	var models []domain.ModelRef
	for _, m := range resp.Data {
		models = append(models, domain.ModelRef{
			ID:            m.ID,
			DisplayName:   m.ID,
			ProviderID:    domain.ProviderCustom,
			ContextLength: 32768,
			IsFree:        true,
			PricingTier:   domain.PricingFree,
			EvalOK:        true,
		})
	}
	return models, nil
}

// DiscoveryResult encapsulates the result of probing a single connector.
type DiscoveryResult struct {
	Provider domain.AIProvider
	Status   domain.ConnectorStatus
	Models   []domain.ModelRef
	Err      error
}

// DefaultConnectors returns the list of all registered connectors.
func DefaultConnectors() []Connector {
	return []Connector{
		NewOpenRouterConnector(nil),
		NewOllamaConnector(nil),
		NewOpenAIConnector(nil),
		NewAnthropicConnector(nil),
		NewVLLMConnector(nil),
		NewLMStudioConnector(nil),
		NewCustomOpenAIConnector(nil),
	}
}

// ProbeConnectors probes all given connectors in parallel without blocking the caller.
// Each local probe has a hard timeout of 750ms.
func ProbeConnectors(ctx context.Context, connectors []Connector, configs map[domain.AIProvider]domain.ProviderConfig, client *http.Client) map[domain.AIProvider]DiscoveryResult {
	results := make(map[domain.AIProvider]DiscoveryResult)
	var mu sync.Mutex
	var wg sync.WaitGroup

	if client == nil {
		client = &http.Client{Timeout: 750 * time.Millisecond}
	}

	for _, conn := range connectors {
		wg.Add(1)
		go func(c Connector) {
			defer wg.Done()

			pID := c.ID()
			var pCfg domain.ProviderConfig
			if configs != nil {
				pCfg = configs[pID]
			}
			endpoint := pCfg.Endpoint
			if endpoint == "" {
				endpoint = c.DefaultURL()
			}

			probeCtx, cancel := context.WithTimeout(ctx, 750*time.Millisecond)
			defer cancel()

			status, err := c.Health(probeCtx, endpoint, pCfg.APIKey)
			var models []domain.ModelRef

			// If status is detected or connected, optionally fetch models if local or already loaded
			if status == domain.StatusDetected || (c.Family() == domain.FamilyCloudManaged && status == domain.StatusConnected) {
				fetchCtx, fetchCancel := context.WithTimeout(ctx, 1500*time.Millisecond)
				defer fetchCancel()

				reqURL := c.ModelsEndpoint(endpoint)
				req, reqErr := http.NewRequestWithContext(fetchCtx, http.MethodGet, reqURL, nil)
				if reqErr == nil {
					if pCfg.APIKey != "" {
						req.Header.Set("Authorization", "Bearer "+pCfg.APIKey)
					}
					resp, doErr := client.Do(req)
					if doErr == nil {
						body, readErr := io.ReadAll(resp.Body)
						resp.Body.Close()
						if readErr == nil && resp.StatusCode == http.StatusOK {
							parsed, pErr := c.ParseModels(body)
							if pErr == nil {
								models = parsed
							}
						}
					}
				}
			}

			mu.Lock()
			results[pID] = DiscoveryResult{
				Provider: pID,
				Status:   status,
				Models:   models,
				Err:      err,
			}
			mu.Unlock()
		}(conn)
	}

	wg.Wait()
	return results
}
