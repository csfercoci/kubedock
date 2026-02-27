package routes

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/joyrex2001/kubedock/internal/config"
	"github.com/joyrex2001/kubedock/internal/server/httputil"
	"github.com/joyrex2001/kubedock/internal/server/routes/common"
	"github.com/joyrex2001/kubedock/internal/server/routes/libpod"
)

// LibpodHeadersMiddleware is a gin-gonic middleware that will add http headers
// that are relevant for libpod endpoints.`
func LibpodHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.Contains(c.Request.URL.Path, "/libpod/") {
			c.Writer.Header().Set("Libpod-API-Version", config.LibpodAPIVersion)
		}
	}
}

// RegisterLibpodRoutes will add all suported podman routes.
func RegisterLibpodRoutes(router *gin.Engine, cr *common.ContextRouter) {
	wrap := func(fn func(*common.ContextRouter, *gin.Context)) gin.HandlerFunc {
		return func(c *gin.Context) {
			fn(cr, c)
		}
	}

	router.Use(LibpodHeadersMiddleware())

	router.GET("/libpod/version", wrap(libpod.Version))
	router.GET("/libpod/_ping", wrap(libpod.Ping))
	router.HEAD("/libpod/_ping", wrap(libpod.Ping))

	router.POST("/libpod/containers/create", wrap(libpod.ContainerCreate))
	router.POST("/libpod/containers/:id/start", wrap(common.ContainerStart))
	router.GET("/libpod/containers/:id/exists", wrap(libpod.ContainerExists))
	router.POST("/libpod/containers/:id/attach", wrap(common.ContainerAttach))
	router.POST("/libpod/containers/:id/stop", wrap(common.ContainerStop))
	router.POST("/libpod/containers/:id/restart", wrap(common.ContainerRestart))
	router.POST("/libpod/containers/:id/kill", wrap(common.ContainerKill))
	router.POST("/libpod/containers/:id/wait", wrap(libpod.ContainerWait))
	router.POST("/libpod/containers/:id/rename", wrap(common.ContainerRename))
	router.POST("/libpod/containers/:id/resize", wrap(common.ContainerResize))
	router.DELETE("/libpod/containers/:id", wrap(libpod.ContainerDelete))
	router.GET("/libpod/containers/json", wrap(libpod.ContainerList))
	router.GET("/libpod/containers/:id/json", wrap(libpod.ContainerInfo))
	router.GET("/libpod/containers/:id/logs", wrap(common.ContainerLogs))

	router.HEAD("/libpod/containers/:id/archive", wrap(common.HeadArchive))
	router.GET("/libpod/containers/:id/archive", wrap(common.GetArchive))
	router.PUT("/libpod/containers/:id/archive", wrap(common.PutArchive))

	router.POST("/libpod/containers/:id/exec", wrap(common.ContainerExec))
	router.POST("/libpod/exec/:id/start", wrap(common.ExecStart))
	router.GET("/libpod/exec/:id/json", wrap(common.ExecInfo))
	router.POST("/libpod/exec/:id/resize", wrap(common.ExecResize))

	router.POST("/libpod/images/pull", wrap(libpod.ImagePull))
	router.GET("/libpod/images/json", wrap(common.ImageList))
	router.GET("/libpod/images/:image/*json", wrap(common.ImageJSON))

	router.GET("/libpod/info", wrap(libpod.Info))
	router.GET("/libpod/events", wrap(libpod.Events))

	router.POST("/libpod/volumes/create", wrap(libpod.VolumeCreate))
	router.GET("/libpod/volumes/json", wrap(libpod.VolumeList))
	router.GET("/libpod/volumes/:name/json", wrap(libpod.VolumeInfo))
	router.GET("/libpod/volumes/:name/exists", wrap(libpod.VolumeExists))
	router.DELETE("/libpod/volumes/:name", wrap(libpod.VolumeDelete))
	router.POST("/libpod/volumes/prune", wrap(libpod.VolumePrune))

	router.GET("/libpod/networks/json", wrap(libpod.NetworkList))
	router.GET("/libpod/networks/:id/json", wrap(libpod.NetworkInfo))
	router.GET("/libpod/networks/:id/exists", wrap(libpod.NetworkExists))
	router.POST("/libpod/networks/create", wrap(libpod.NetworkCreate))
	router.DELETE("/libpod/networks/:id", wrap(libpod.NetworkDelete))
	router.POST("/libpod/networks/:id/connect", wrap(libpod.NetworkConnect))
	router.POST("/libpod/networks/:id/disconnect", wrap(libpod.NetworkDisconnect))
	router.POST("/libpod/networks/prune", wrap(libpod.NetworkPrune))

	// not supported podman api at the moment
	router.POST("/libpod/build", httputil.NotImplemented)
	router.POST("/libpod/images/load", httputil.NotImplemented)
}
