package libpod

import (
	"encoding/json"
	"net/http"
	"runtime"

	"github.com/gin-gonic/gin"
	"k8s.io/klog"

	"github.com/joyrex2001/kubedock/internal/config"
	"github.com/joyrex2001/kubedock/internal/server/filter"
	"github.com/joyrex2001/kubedock/internal/server/routes/common"
)

// Version - get version.
// https://docs.podman.io/en/latest/_static/api.html?version=v4.2#tag/system/operation/SystemVersionLibpod
// GET "/libpod/version"
func Version(cr *common.ContextRouter, c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"Version":    config.Version,
		"ApiVersion": config.LibpodAPIVersion,
		"GoVersion":  config.GoVersion,
		"GitCommit":  config.Build,
		"Built":      config.Date,
		"OsArch":     config.GOOS + "/" + config.GOARCH,
		"Os":         config.OS,
	})
}

// Ping - dummy endpoint you can use to test if the server is accessible.
// https://docs.podman.io/en/latest/_static/api.html?version=v4.2#tag/system/operation/SystemPing
// GET "/libpod/_ping"
func Ping(cr *common.ContextRouter, c *gin.Context) {
	c.String(http.StatusOK, "OK")
}

// Info - return system-level information about podman.
// Adapted for OCP 4.18: reports SELinux enabled, cgroup v2, and kubernetes
// as the underlying runtime. This satisfies podman compose's system info query.
// https://docs.podman.io/en/latest/_static/api.html?version=v4.2#tag/system/operation/SystemInfoLibpod
// GET "/libpod/info"
func Info(cr *common.ContextRouter, c *gin.Context) {
	secProfile := cr.Config.SecurityContext
	selinuxEnabled := secProfile == "restricted"
	c.JSON(http.StatusOK, gin.H{
		"host": gin.H{
			"arch":            runtime.GOARCH,
			"os":              runtime.GOOS,
			"hostname":        "kubedock",
			"kernel":          "kubernetes",
			"memTotal":        0,
			"cpus":            runtime.NumCPU(),
			"distribution":    gin.H{"distribution": "Red Hat Enterprise Linux CoreOS", "version": "4.18"},
			"buildahVersion":  "0.0.0",
			"conmonVersion":   "0.0.0",
			"linkmode":        "dynamic",
			"logDriver":       "k8s-file",
			"ociRuntime":      gin.H{"name": "crun", "version": ""},
			"remoteSocket":    gin.H{"exists": true, "path": ""},
			"security":        gin.H{"selinuxEnabled": selinuxEnabled, "rootless": true, "apparmorEnabled": false},
			"slirp4netns":     gin.H{"executable": "", "package": "", "version": ""},
			"cgroupManager":   "systemd",
			"cgroupVersion":   "v2",
			"databaseBackend": "in-memory",
		},
		"store": gin.H{
			"configFile":      "",
			"containerStore":  gin.H{"number": 0, "paused": 0, "running": 0, "stopped": 0},
			"graphDriverName": "overlay",
			"graphRoot":       "/var/lib/kubedock/storage",
			"graphStatus":     map[string]string{},
			"imageStore":      gin.H{"number": 0},
			"runRoot":         "/run/kubedock",
			"volumePath":      "/var/lib/kubedock/volumes",
		},
		"registries": gin.H{
			"search": []string{"registry.redhat.io", "registry.access.redhat.com", "docker.io"},
		},
		"version": gin.H{
			"APIVersion": config.LibpodAPIVersion,
			"Version":    config.Version,
			"GoVersion":  config.GoVersion,
			"GitCommit":  config.Build,
			"BuiltTime":  config.Date,
			"Built":      0,
			"OsArch":     config.GOOS + "/" + config.GOARCH,
			"Os":         config.OS,
		},
		"plugins": gin.H{
			"volume":  []string{"local"},
			"network": []string{"bridge"},
			"log":     []string{"k8s-file"},
		},
	})
}

// Events - stream real-time events from podman.
// https://docs.podman.io/en/latest/_static/api.html?version=v4.2#tag/system/operation/SystemEventsLibpod
// GET "/libpod/events"
func Events(cr *common.ContextRouter, c *gin.Context) {
	w := c.Writer
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Flush()

	filtr, err := filter.New(c.Query("filters"))
	if err != nil {
		klog.V(5).Infof("unsupported filter: %s", err)
	}

	enc := json.NewEncoder(w)
	el, id := cr.Events.Subscribe()
	for {
		select {
		case <-c.Request.Context().Done():
			cr.Events.Unsubscribe(id)
			return
		case msg := <-el:
			if filtr.Match(&msg) {
				klog.V(5).Infof("sending message to %s", id)
				enc.Encode(gin.H{
					"Type":   msg.Type,
					"Status": msg.Action,
					"Action": msg.Action,
					"Actor": gin.H{
						"ID":         msg.ID,
						"Attributes": gin.H{},
					},
					"time":     msg.Time,
					"timeNano": msg.TimeNano,
				})
				w.Flush()
			}
		}
	}
}
