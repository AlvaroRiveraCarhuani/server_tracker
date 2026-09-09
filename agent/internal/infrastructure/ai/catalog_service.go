package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/alvaroriverac/server_tracker_agent/internal/core/domain"
)

// CatalogCacheData representa el esquema serializado en ~/.solv/catalog_cache.json.
type CatalogCacheData struct {
	LastSynced     time.Time                         `json:"last_synced"`
	Models         []domain.AIModel                  `json:"models"`
	RuntimeMetrics map[string]domain.RuntimeMetrics  `json:"runtime_metrics"`
}

// CatalogService coordina la sincronización de catálogos, métricas de inferencia y evaluación AIOps.
type CatalogService struct {
	providers      map[domain.AIProvider]CatalogProvider
	cachePath      string
	models         []domain.AIModel
	assessments    map[string]domain.ModelAssessment
	runtimeMetrics map[string]*domain.RuntimeMetrics
	latencyWindows map[string][]time.Duration
	lastSynced     time.Time
	syncMu         sync.RWMutex
}

// NewCatalogService crea un nuevo servicio de catálogo con soporte para OpenRouter y Ollama.
func NewCatalogService(cachePath string, customProviders ...CatalogProvider) *CatalogService {
	provMap := make(map[domain.AIProvider]CatalogProvider)
	for _, cp := range customProviders {
		if cp != nil {
			provMap[cp.ID()] = cp
		}
	}

	if _, ok := provMap[domain.ProviderOpenRouter]; !ok {
		provMap[domain.ProviderOpenRouter] = NewOpenRouterCatalogProvider(nil)
	}
	if _, ok := provMap[domain.ProviderOllama]; !ok {
		provMap[domain.ProviderOllama] = NewOllamaCatalogProvider(nil)
	}

	cs := &CatalogService{
		providers:      provMap,
		cachePath:      cachePath,
		models:         make([]domain.AIModel, 0),
		assessments:    make(map[string]domain.ModelAssessment),
		runtimeMetrics: make(map[string]*domain.RuntimeMetrics),
		latencyWindows: make(map[string][]time.Duration),
	}

	cs.LoadCacheOrSeeds()
	return cs
}

// LoadCacheOrSeeds carga inmediatamente desde la caché local segura o inicializa con las 4 semillas de rescate.
func (cs *CatalogService) LoadCacheOrSeeds() []domain.AIModel {
	cs.syncMu.Lock()
	defer cs.syncMu.Unlock()

	loaded := false
	if cs.cachePath != "" {
		if data, err := os.ReadFile(cs.cachePath); err == nil {
			var cacheData CatalogCacheData
			if err := json.Unmarshal(data, &cacheData); err == nil && len(cacheData.Models) > 0 {
				cs.models = cacheData.Models
				cs.lastSynced = cacheData.LastSynced
				if cacheData.RuntimeMetrics != nil {
					for k, v := range cacheData.RuntimeMetrics {
						val := v
						cs.runtimeMetrics[k] = &val
					}
				}
				loaded = true
			}
		}
	}

	if !loaded {
		// Fallback inmediato a semillas de arranque
		cs.models = make([]domain.AIModel, len(domain.BootstrapSeedModels))
		copy(cs.models, domain.BootstrapSeedModels)
	}

	cs.recomputeAssessmentsLocked()
	return cs.models
}

// recomputeAssessmentsLocked actualiza las evaluaciones técnicas de todos los modelos en memoria.
func (cs *CatalogService) recomputeAssessmentsLocked() {
	cs.assessments = make(map[string]domain.ModelAssessment, len(cs.models))
	for _, m := range cs.models {
		rt := domain.RuntimeMetrics{
			SampleState: domain.SampleStateEstimated,
		}
		if stored, ok := cs.runtimeMetrics[m.ID]; ok && stored != nil {
			rt = *stored
		}
		cs.assessments[m.ID] = domain.AssessModel(m, rt)
	}
}

// SaveCache persiste el catálogo y métricas observadas con permisos 0600 (blindaje D2).
func (cs *CatalogService) SaveCache() error {
	cs.syncMu.RLock()
	defer cs.syncMu.RUnlock()

	if cs.cachePath == "" {
		return nil
	}

	dir := filepath.Dir(cs.cachePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("error creando directorio de cache: %w", err)
	}

	metricsMap := make(map[string]domain.RuntimeMetrics, len(cs.runtimeMetrics))
	for k, v := range cs.runtimeMetrics {
		if v != nil {
			metricsMap[k] = *v
		}
	}

	data := CatalogCacheData{
		LastSynced:     cs.lastSynced,
		Models:         cs.models,
		RuntimeMetrics: metricsMap,
	}

	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("error serializando cache de catalogo: %w", err)
	}

	tmpFile := cs.cachePath + ".tmp"
	if err := os.WriteFile(tmpFile, b, 0600); err != nil {
		return fmt.Errorf("error escribiendo cache temporal: %w", err)
	}

	if err := os.Rename(tmpFile, cs.cachePath); err != nil {
		return fmt.Errorf("error reemplazando archivo de cache: %w", err)
	}

	return nil
}

// SyncIfExpired sincroniza en segundo plano con los proveedores registrados si expiró el TTL.
func (cs *CatalogService) SyncIfExpired(ctx context.Context, ttl time.Duration, pConfigs map[domain.AIProvider]domain.ProviderConfig) error {
	cs.syncMu.RLock()
	if !cs.lastSynced.IsZero() && time.Since(cs.lastSynced) < ttl {
		cs.syncMu.RUnlock()
		return nil // TTL aún válido
	}
	cs.syncMu.RUnlock()

	var allFetched []domain.AIModel
	modelMap := make(map[string]domain.AIModel)

	for provID, provider := range cs.providers {
		var pCfg domain.ProviderConfig
		if pConfigs != nil {
			pCfg = pConfigs[provID]
		}

		fetched, err := provider.FetchModels(ctx, pCfg)
		if err == nil && len(fetched) > 0 {
			for _, m := range fetched {
				modelMap[m.ID] = m
				allFetched = append(allFetched, m)
			}
		}
	}

	if len(allFetched) == 0 {
		return fmt.Errorf("no se pudieron sincronizar modelos de ningun proveedor")
	}

	cs.syncMu.Lock()
	cs.models = allFetched
	cs.lastSynced = time.Now()
	cs.recomputeAssessmentsLocked()
	cs.syncMu.Unlock()

	_ = cs.SaveCache()
	return nil
}

// RecordInference registra la latencia y resultado de una inferencia, alimentando el rolling window P95.
func (cs *CatalogService) RecordInference(modelID string, latency time.Duration, success bool) {
	cs.syncMu.Lock()
	defer cs.syncMu.Unlock()

	window := cs.latencyWindows[modelID]
	window = append(window, latency)
	if len(window) > 50 {
		window = window[len(window)-50:]
	}
	cs.latencyWindows[modelID] = window

	rt, ok := cs.runtimeMetrics[modelID]
	if !ok || rt == nil {
		rt = &domain.RuntimeMetrics{
			SampleState: domain.SampleStateEstimated,
		}
		cs.runtimeMetrics[modelID] = rt
	}

	rt.RequestCount++
	rt.LastMeasured = time.Now()

	// Actualizar SuccessRate
	prevSuccesses := float64(rt.RequestCount-1) * rt.SuccessRate
	if success {
		prevSuccesses += 1.0
	}
	rt.SuccessRate = prevSuccesses / float64(rt.RequestCount)

	// Actualizar promedio
	var sum time.Duration
	for _, d := range window {
		sum += d
	}
	rt.AvgLatency = sum / time.Duration(len(window))

	// Actualizar P95 solo si tenemos al menos 5 muestras (significancia estadística)
	if len(window) >= 5 {
		rt.SampleState = domain.SampleStateObserved
		sorted := make([]time.Duration, len(window))
		copy(sorted, window)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
		idx := int(float64(len(sorted))*0.95) - 1
		if idx < 0 {
			idx = 0
		}
		if idx >= len(sorted) {
			idx = len(sorted) - 1
		}
		rt.P95Latency = sorted[idx]
	} else {
		rt.SampleState = domain.SampleStateEstimated
		rt.P95Latency = rt.AvgLatency
	}

	// Recalcular assessment para este modelo específico
	for _, m := range cs.models {
		if m.ID == modelID {
			cs.assessments[modelID] = domain.AssessModel(m, *rt)
			break
		}
	}
}

// FreshnessString devuelve una cadena concisa sobre la antigüedad del catálogo.
func (cs *CatalogService) FreshnessString() string {
	cs.syncMu.RLock()
	defer cs.syncMu.RUnlock()

	if cs.lastSynced.IsZero() {
		return "CATALOG: en cache (offline)"
	}

	d := time.Since(cs.lastSynced)
	if d < time.Minute {
		return "CATALOG: actualizado hace unos segundos"
	}
	if d < time.Hour {
		return fmt.Sprintf("CATALOG: actualizado hace %dm", int(d.Minutes()))
	}
	return fmt.Sprintf("CATALOG: en cache (%dh)", int(d.Hours()))
}

// GetSlotFast retorna el modelo más adecuado para diagnóstico rápido (<2s, structured output, bajo costo).
func (cs *CatalogService) GetSlotFast() (domain.AIModel, domain.ModelAssessment) {
	cs.syncMu.RLock()
	defer cs.syncMu.RUnlock()

	var bestM domain.AIModel
	var bestA domain.ModelAssessment
	highest := -1.0

	for _, m := range cs.models {
		a := cs.assessments[m.ID]
		if a.FastSuitability > highest && m.Capabilities.StructuredOutput {
			highest = a.FastSuitability
			bestM = m
			bestA = a
		}
	}

	if highest < 0 && len(cs.models) > 0 {
		bestM = cs.models[0]
		bestA = cs.assessments[bestM.ID]
	}
	return bestM, bestA
}

// GetSlotDeep retorna el modelo más idóneo para análisis profundo de causas complejas.
func (cs *CatalogService) GetSlotDeep() (domain.AIModel, domain.ModelAssessment) {
	cs.syncMu.RLock()
	defer cs.syncMu.RUnlock()

	var bestM domain.AIModel
	var bestA domain.ModelAssessment
	highest := -1.0

	for _, m := range cs.models {
		a := cs.assessments[m.ID]
		if a.DeepSuitability > highest {
			highest = a.DeepSuitability
			bestM = m
			bestA = a
		}
	}

	if highest < 0 && len(cs.models) > 0 {
		bestM = cs.models[0]
		bestA = cs.assessments[bestM.ID]
	}
	return bestM, bestA
}

// GetSlotFree retorna el router openrouter/free o el modelo gratuito con mayor evaluación.
func (cs *CatalogService) GetSlotFree() (domain.AIModel, domain.ModelAssessment) {
	cs.syncMu.RLock()
	defer cs.syncMu.RUnlock()

	// 1. Priorizar router openrouter/free
	for _, m := range cs.models {
		if m.IsRouter && m.Pricing.IsFree {
			return m, cs.assessments[m.ID]
		}
	}

	// 2. Modelo gratuito con mayor score
	var bestM domain.AIModel
	var bestA domain.ModelAssessment
	highest := -1.0

	for _, m := range cs.models {
		if m.Pricing.IsFree || m.Pricing.Tier == domain.PricingFree {
			a := cs.assessments[m.ID]
			if a.OverallScore > highest {
				highest = a.OverallScore
				bestM = m
				bestA = a
			}
		}
	}

	if highest >= 0 {
		return bestM, bestA
	}

	return cs.GetSlotFast()
}

// GetSlotLocal retorna el mejor modelo disponible en el runtime local de Ollama.
func (cs *CatalogService) GetSlotLocal() (domain.AIModel, domain.ModelAssessment) {
	cs.syncMu.RLock()
	defer cs.syncMu.RUnlock()

	var bestM domain.AIModel
	var bestA domain.ModelAssessment
	highest := -1.0

	for _, m := range cs.models {
		if m.ProviderID == domain.ProviderOllama {
			a := cs.assessments[m.ID]
			if a.OverallScore > highest {
				highest = a.OverallScore
				bestM = m
				bestA = a
			}
		}
	}

	if highest >= 0 {
		return bestM, bestA
	}

	return cs.GetSlotFast()
}

// GetModelsByProvider retorna los modelos correspondientes a un proveedor específico.
func (cs *CatalogService) GetModelsByProvider(prov domain.AIProvider) []domain.AIModel {
	cs.syncMu.RLock()
	defer cs.syncMu.RUnlock()

	var res []domain.AIModel
	for _, m := range cs.models {
		if m.ProviderID == prov {
			res = append(res, m)
		}
	}
	return res
}

// GetAssessment retorna la evaluación técnica calculada para un modelo.
func (cs *CatalogService) GetAssessment(modelID string) domain.ModelAssessment {
	cs.syncMu.RLock()
	defer cs.syncMu.RUnlock()
	return cs.assessments[modelID]
}

// SearchGlobal busca transversalmente en todos los proveedores por nombre, id o creador.
func (cs *CatalogService) SearchGlobal(query string) []domain.AIModel {
	cs.syncMu.RLock()
	defer cs.syncMu.RUnlock()

	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		res := make([]domain.AIModel, len(cs.models))
		copy(res, cs.models)
		return res
	}

	var res []domain.AIModel
	for _, m := range cs.models {
		if strings.Contains(strings.ToLower(m.DisplayName), q) ||
			strings.Contains(strings.ToLower(m.ID), q) ||
			strings.Contains(strings.ToLower(m.Creator), q) ||
			strings.Contains(strings.ToLower(string(m.ProviderID)), q) {
			res = append(res, m)
		}
	}
	return res
}

// AllModels retorna todos los modelos en memoria.
func (cs *CatalogService) AllModels() []domain.AIModel {
	cs.syncMu.RLock()
	defer cs.syncMu.RUnlock()
	res := make([]domain.AIModel, len(cs.models))
	copy(res, cs.models)
	return res
}
