package middleware

import (
	"net/http"

	"github.com/pulcy/rest-kit"
	api "github.com/pulcy/robin-api"
	"gopkg.in/macaron.v1"
)

// All handles an API.All request
func (m *Middleware) All(res http.ResponseWriter, req *http.Request) error {
	result, err := m.Service.All()
	if err != nil {
		return m.mapError(res, maskAny(err))
	}
	return restkit.JSON(res, result, http.StatusOK)
}

// Get handles an API.Get request
func (m *Middleware) Get(ctx *macaron.Context, res http.ResponseWriter, req *http.Request) error {
	id := ctx.Params("id")
	result, err := m.Service.Get(id)
	if err != nil {
		return m.mapError(res, maskAny(err))
	}
	return restkit.JSON(res, result, http.StatusOK)
}

// Add handles an API.Add request
func (m *Middleware) Add(ctx *macaron.Context, res http.ResponseWriter, req *http.Request) error {
	id := ctx.Params("id")
	var record api.FrontendRecord
	if err := parseBody(req, &record); err != nil {
		return m.mapError(res, maskAny(err))
	}
	err := m.Service.Add(id, record)
	if err != nil {
		return m.mapError(res, maskAny(err))
	}
	result := map[string]string{
		"status": "ok",
	}
	return restkit.JSON(res, result, http.StatusOK)
}

// Remove handles an API.Remove request
func (m *Middleware) Remove(ctx *macaron.Context, res http.ResponseWriter, req *http.Request) error {
	id := ctx.Params("id")
	err := m.Service.Remove(id)
	if err != nil {
		return m.mapError(res, maskAny(err))
	}
	result := map[string]string{
		"status": "ok",
	}
	return restkit.JSON(res, result, http.StatusOK)
}
