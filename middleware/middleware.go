package middleware

import (
	"encoding/json"
	"net/http"

	"github.com/juju/errgo"
	"github.com/op/go-logging"
	"github.com/pulcy/macaron-utils"
	restkit "github.com/pulcy/rest-kit"
	"gopkg.in/macaron.v1"

	"github.com/pulcy/robin-api"
)

var (
	maskAny = errgo.MaskFunc(errgo.Any)
)

type Middleware struct {
	Logger  *logging.Logger
	Service api.API
}

func (m *Middleware) SetupRoutes(projectName, projectVersion, projectBuild string) http.Handler {
	mac := macaron.New()
	mac.Use(utils.Logger(m.Logger,
		utils.DontLogHead(),
	))
	mac.Use(utils.DefaultJSON())
	mac.Use(macaron.Recovery())
	mac.Use(macaron.Renderer())
	mac.Map(m.Service)
	mac.SetAutoHead(true)

	// Alive ping
	mac.Get("/v1/ping", utils.Ping())

	// Our API
	mac.Get("/v1/frontend", m.All)
	mac.Post("/v1/frontend/:id", m.Add)
	mac.Delete("/v1/frontend/:id", m.Remove)
	mac.Get("/v1/frontend/:id", m.Get)

	// Home
	mac.Get("/", utils.ServerInfo(projectName, projectVersion, projectBuild))

	return mac

	// receive api
}

// MapError maps an error to a proper response.
func (m *Middleware) mapError(res http.ResponseWriter, err error) error {
	m.Logger.Debugf("Error: %#v", err)
	return restkit.Error(res, err)
}

func parseBody(req *http.Request, content interface{}) error {
	decoder := json.NewDecoder(req.Body)
	defer req.Body.Close()
	return decoder.Decode(content)
}
