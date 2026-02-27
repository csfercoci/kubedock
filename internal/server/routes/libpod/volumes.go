package libpod

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"k8s.io/klog"

	"github.com/joyrex2001/kubedock/internal/model/types"
	"github.com/joyrex2001/kubedock/internal/server/filter"
	"github.com/joyrex2001/kubedock/internal/server/httputil"
	"github.com/joyrex2001/kubedock/internal/server/routes/common"
)

// VolumeCreateRequest represents the json structure for libpod volume creation.
type VolumeCreateRequest struct {
	Name    string            `json:"Name"`
	Driver  string            `json:"Driver"`
	Options map[string]string `json:"Options"`
	Labels  map[string]string `json:"Labels"`
}

// VolumeCreate - create a volume.
// https://docs.podman.io/en/latest/_static/api.html?version=v4.2#tag/volumes/operation/VolumeCreateLibpod
// POST "/libpod/volumes/create"
func VolumeCreate(cr *common.ContextRouter, c *gin.Context) {
	in := &VolumeCreateRequest{}
	if err := json.NewDecoder(c.Request.Body).Decode(&in); err != nil {
		httputil.Error(c, http.StatusInternalServerError, err)
		return
	}

	driver := in.Driver
	if driver == "" {
		driver = "local"
	}

	// Check if volume already exists
	if existing, err := cr.DB.GetVolumeByName(in.Name); err == nil {
		c.JSON(http.StatusCreated, volumeToLibpodJSON(existing))
		return
	}

	vol := &types.Volume{
		Name:   in.Name,
		Driver: driver,
		Labels: in.Labels,
	}

	if err := cr.Backend.CreateVolume(vol); err != nil {
		httputil.Error(c, http.StatusInternalServerError, err)
		return
	}

	if err := cr.DB.SaveVolume(vol); err != nil {
		httputil.Error(c, http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusCreated, volumeToLibpodJSON(vol))
}

// VolumeList - list volumes.
// https://docs.podman.io/en/latest/_static/api.html?version=v4.2#tag/volumes/operation/VolumeListLibpod
// GET "/libpod/volumes/json"
func VolumeList(cr *common.ContextRouter, c *gin.Context) {
	vols, err := cr.DB.GetVolumes()
	if err != nil {
		httputil.Error(c, http.StatusInternalServerError, err)
		return
	}

	filtr, err := filter.New(c.Query("filters"))
	if err != nil {
		klog.V(5).Infof("unsupported filter: %s", err)
	}

	res := []gin.H{}
	for _, vol := range vols {
		if filtr.Match(vol) {
			res = append(res, volumeToLibpodJSON(vol))
		}
	}

	c.JSON(http.StatusOK, res)
}

// VolumeInfo - inspect a volume.
// https://docs.podman.io/en/latest/_static/api.html?version=v4.2#tag/volumes/operation/VolumeInspectLibpod
// GET "/libpod/volumes/:name/json"
func VolumeInfo(cr *common.ContextRouter, c *gin.Context) {
	name := c.Param("name")
	vol, err := cr.DB.GetVolumeByNameOrID(name)
	if err != nil {
		httputil.Error(c, http.StatusNotFound, err)
		return
	}
	c.JSON(http.StatusOK, volumeToLibpodJSON(vol))
}

// VolumeExists - check if volume exists.
// https://docs.podman.io/en/latest/_static/api.html?version=v4.2#tag/volumes/operation/VolumeExistsLibpod
// GET "/libpod/volumes/:name/exists"
func VolumeExists(cr *common.ContextRouter, c *gin.Context) {
	name := c.Param("name")
	_, err := cr.DB.GetVolumeByNameOrID(name)
	if err != nil {
		httputil.Error(c, http.StatusNotFound, err)
		return
	}
	c.Writer.WriteHeader(http.StatusNoContent)
}

// VolumeDelete - remove a volume.
// https://docs.podman.io/en/latest/_static/api.html?version=v4.2#tag/volumes/operation/VolumeDeleteLibpod
// DELETE "/libpod/volumes/:name"
func VolumeDelete(cr *common.ContextRouter, c *gin.Context) {
	name := c.Param("name")
	vol, err := cr.DB.GetVolumeByNameOrID(name)
	if err != nil {
		httputil.Error(c, http.StatusNotFound, err)
		return
	}

	if err := cr.Backend.DeleteVolume(vol); err != nil {
		klog.Warningf("error deleting k8s PVC for volume: %s", err)
	}

	if err := cr.DB.DeleteVolume(vol); err != nil {
		httputil.Error(c, http.StatusNotFound, err)
		return
	}

	c.Writer.WriteHeader(http.StatusNoContent)
}

// VolumePrune - prune unused volumes.
// https://docs.podman.io/en/latest/_static/api.html?version=v4.2#tag/volumes/operation/VolumePruneLibpod
// POST "/libpod/volumes/prune"
func VolumePrune(cr *common.ContextRouter, c *gin.Context) {
	vols, err := cr.DB.GetVolumes()
	if err != nil {
		httputil.Error(c, http.StatusInternalServerError, err)
		return
	}

	pruned := []gin.H{}
	for _, vol := range vols {
		if err := cr.Backend.DeleteVolume(vol); err != nil {
			klog.Warningf("error deleting k8s PVC for volume %s: %s", vol.Name, err)
		}
		if err := cr.DB.DeleteVolume(vol); err != nil {
			klog.Warningf("error deleting volume %s from db: %s", vol.Name, err)
			continue
		}
		pruned = append(pruned, gin.H{
			"Id":   vol.ID,
			"Size": 0,
		})
	}

	c.JSON(http.StatusOK, pruned)
}

// volumeToLibpodJSON returns a gin.H containing volume details in libpod format.
func volumeToLibpodJSON(vol *types.Volume) gin.H {
	driver := vol.Driver
	if driver == "" {
		driver = "local"
	}
	labels := vol.Labels
	if labels == nil {
		labels = map[string]string{}
	}
	mountpoint := vol.Mountpoint
	if mountpoint == "" {
		mountpoint = "/var/lib/kubedock/volumes/" + vol.Name
	}
	return gin.H{
		"Name":       vol.Name,
		"Driver":     driver,
		"Mountpoint": mountpoint,
		"Labels":     labels,
		"Scope":      "local",
		"CreatedAt":  vol.Created.Format(time.RFC3339),
		"Options":    map[string]string{},
	}
}
