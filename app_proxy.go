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
	if err := a.ready(); err != nil {
		return nil, err
	}
	if err := validation.ValidateProxy(server, models.ProxyProtocol(protocol)); err != nil {
		return nil, fmt.Errorf("add proxy: %w", err)
	}

	p := models.Proxy{
		ID:        uuid.New().String(),
		Server:    server,
		Protocol:  models.ProxyProtocol(protocol),
		Username:  username,
		Password:  password,
		Geo:       geo,
		Status:    models.ProxyStatusUnknown,
		CreatedAt: time.Now(),
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
	for i := range proxies {
		proxies[i].Username = maskCredential(proxies[i].Username)
		proxies[i].Password = maskCredential(proxies[i].Password)
	}
	return proxies, nil
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
