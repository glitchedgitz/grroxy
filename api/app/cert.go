package api

import (
	"bufio"
	"bytes"
	"io"
	"net/http"
	"os"
	"path"

	"github.com/glitchedgitz/grroxy-db/certs"
	"github.com/glitchedgitz/grroxy-db/save"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func (backend *Backend) DownloadCert(e *core.ServeEvent) error {
	e.Router.AddRoute(echo.Route{
		Method: http.MethodGet,
		Path:   "/cacert.crt",
		Handler: func(c echo.Context) error {

			homeDir, err := os.UserHomeDir()
			if err != nil {
				return err
			}

			certs, err := certs.New(&certs.Options{
				CacheSize: 256,
				Directory: path.Join(homeDir, ".config", "grroxy"),
			})
			if err != nil {
				return err
			}
			_, ca := certs.GetCA()
			reader := bytes.NewReader(ca)
			bf := bufio.NewReader(reader)
			respbody, err := io.ReadAll(bf)

			filePath := path.Join(backend.Config.ConfigDirectory, "cacert.crt")
			save.WriteFile(filePath, respbody)

			if err != nil {
				return err
			}

			return c.Attachment(filePath, "cacert.crt")

		},
		Middlewares: []echo.MiddlewareFunc{
			apis.ActivityLogger(backend.App),
		},
	})
	return nil
}
