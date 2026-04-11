package main

import (
	"fmt"
	"strings"
	"time"

	"flowpilot/internal/models"
	"flowpilot/internal/validation"

	"github.com/google/uuid"
)

func (a *App) AddProxy(server, protocol, username, password, geo string) (*models.Proxy, error) {
	return a.AddProxyWithRateLimit(server, protocol, username, password, geo, 0)
}

func (a *App) AddProxyWithRateLimit(server, protocol, username, password, geo string, maxRequestsPerMinute int) (*models.Proxy, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	if err := validation.ValidateProxy(server, models.ProxyProtocol(protocol)); err != nil {
		return nil, fmt.Errorf("add proxy: %w", err)
	}
	if maxRequestsPerMinute < 0 {
		return nil, fmt.Errorf("add proxy: maxRequestsPerMinute must be >= 0")
	}

	p := models.Proxy{
		ID:                   uuid.New().String(),
		Server:               server,
		Protocol:             models.ProxyProtocol(protocol),
		Username:             username,
		Password:             password,
		Geo:                  strings.ToUpper(strings.TrimSpace(geo)),
		Status:               models.ProxyStatusUnknown,
		MaxRequestsPerMinute: maxRequestsPerMinute,
		CreatedAt:            time.Now(),
	}

	if err := a.db.CreateProxy(a.ctx, p); err != nil {
		return nil, fmt.Errorf("add proxy: %w", err)
	}
	return &p, nil
}

func (a *App) ListProxies() ([]models.Proxy, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	proxies, err := a.db.ListProxies(a.ctx)
	if err != nil {
		return nil, err
	}
	var localStats map[string]int
	if a.localProxyManager != nil {
		localStats = a.localProxyManager.EndpointStatsByProxy(proxies)
	}
	for i := range proxies {
		if a.localProxyManager != nil {
			proxies[i].LocalEndpoint = a.localProxyManager.EndpointAddr(proxies[i].ToProxyConfig())
			proxies[i].LocalEndpointOn = proxies[i].LocalEndpoint != ""
			proxies[i].LocalAuthEnabled = proxies[i].LocalEndpointOn
			proxies[i].ActiveLocalUsers = localStats[proxies[i].ID]
		}
		proxies[i].Username = maskCredential(proxies[i].Username)
		proxies[i].Password = maskCredential(proxies[i].Password)
	}
	return proxies, nil
}

func (a *App) ListProxyCountryStats() ([]models.ProxyCountryStats, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	proxies, err := a.db.ListProxies(a.ctx)
	if err != nil {
		return nil, err
	}
	activeLocalEndpoints := map[string]int(nil)
	if a.localProxyManager != nil {
		activeLocalEndpoints = a.localProxyManager.EndpointStatsByProxy(proxies)
	}
	if a.proxyManager == nil {
		return nil, fmt.Errorf("proxy manager unavailable")
	}
	return a.proxyManager.CountryStats(proxies, activeLocalEndpoints), nil
}

// validateProxyRoutingPreset validates proxy routing preset parameters (S3776)
func (a *App) validateProxyRoutingPreset(preset *models.ProxyRoutingPreset) error {
	if preset.Name == "" {
		return fmt.Errorf("preset name is required")
	}
	if preset.Fallback == "" {
		preset.Fallback = models.ProxyFallbackStrict
	} else if !isValidProxyFallback(preset.Fallback) {
		return fmt.Errorf("invalid fallback value %q; must be one of: strict, any_healthy, direct", preset.Fallback)
	}
	if preset.RandomByCountry && preset.Country == "" {
		return fmt.Errorf("country is required for random-by-country presets")
	}
	return nil
}

// isValidProxyFallback checks if a fallback value is valid (S1192)
func isValidProxyFallback(fallback models.ProxyRoutingFallback) bool {
	switch fallback {
	case models.ProxyFallbackStrict, models.ProxyFallbackAny, models.ProxyFallbackDirect:
		return true
	default:
		return false
	}
}

func (a *App) CreateProxyRoutingPreset(name, country, fallback string, randomByCountry bool) (*models.ProxyRoutingPreset, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	preset := models.ProxyRoutingPreset{
		ID:              uuid.New().String(),
		Name:            strings.TrimSpace(name),
		Country:         strings.ToUpper(strings.TrimSpace(country)),
		Fallback:        models.ProxyRoutingFallback(strings.TrimSpace(fallback)),
		RandomByCountry: randomByCountry,
		CreatedAt:       time.Now(),
	}
	if err := a.validateProxyRoutingPreset(&preset); err != nil {
		return nil, err
	}
	if err := a.db.CreateProxyRoutingPreset(a.ctx, preset); err != nil {
		return nil, err
	}
	return &preset, nil
}

func (a *App) ListProxyRoutingPresets() ([]models.ProxyRoutingPreset, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.db.ListProxyRoutingPresets(a.ctx)
}

func (a *App) DeleteProxyRoutingPreset(id string) error {
	if err := a.ready(); err != nil {
		return err
	}
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("preset id is required")
	}
	return a.db.DeleteProxyRoutingPreset(a.ctx, id)
}

func (a *App) GetLocalProxyGatewayStats() (models.LocalProxyGatewayStats, error) {
	if err := a.ready(); err != nil {
		return models.LocalProxyGatewayStats{}, err
	}
	if a.localProxyManager == nil {
		return models.LocalProxyGatewayStats{}, fmt.Errorf("local proxy manager unavailable")
	}
	return a.localProxyManager.Stats(), nil
}

func maskCredential(s string) string {
	runes := []rune(s)
	if len(runes) <= 2 {
		return strings.Repeat("*", len(runes))
	}
	return string(runes[0]) + strings.Repeat("*", len(runes)-2) + string(runes[len(runes)-1])
}

func (a *App) DeleteProxy(id string) error {
	if err := a.ready(); err != nil {
		return err
	}
	if id == "" {
		return fmt.Errorf("delete proxy: id is required")
	}
	return a.db.DeleteProxy(a.ctx, id)
}
