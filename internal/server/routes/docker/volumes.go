package docker

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

// VolumeCreateRequest represents the json structure for volume creation.
type VolumeCreateRequest struct {
	Name       string            `json:"Name"`
	Driver     string            `json:"Driver"`
	DriverOpts map[string]string `json:"DriverOpts"`
	Labels     map[string]string `json:"Labels"`
}

// VolumeCreate - create a volume.
// https://docs.docker.com/engine/api/v1.41/#operation/VolumeCreate
// POST "/volumes/create"
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
		c.JSON(http.StatusCreated, volumeToJSON(existing))
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

	c.JSON(http.StatusCreated, volumeToJSON(vol))
}

// VolumeList - list volumes.
// https://docs.docker.com/engine/api/v1.41/#operation/VolumeList
// GET "/volumes"
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
			res = append(res, volumeToJSON(vol))
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"Volumes":  res,
		"Warnings": []string{},
	})
}

// VolumeInfo - inspect a volume.
// https://docs.docker.com/engine/api/v1.41/#operation/VolumeInspect
// GET "/volumes/:id"
func VolumeInfo(cr *common.ContextRouter, c *gin.Context) {
	id := c.Param("id")
	vol, err := cr.DB.GetVolumeByNameOrID(id)
	if err != nil {
		httputil.Error(c, http.StatusNotFound, err)
		return
	}
	c.JSON(http.StatusOK, volumeToJSON(vol))
}

// VolumeDelete - remove a volume.
// https://docs.docker.com/engine/api/v1.41/#operation/VolumeDelete
// DELETE "/volumes/:id"
func VolumeDelete(cr *common.ContextRouter, c *gin.Context) {
	id := c.Param("id")
	vol, err := cr.DB.GetVolumeByNameOrID(id)
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

// VolumesPrune - Delete unused volumes.
// https://docs.docker.com/engine/api/v1.41/#operation/VolumePrune
// POST "/volumes/prune"
func VolumesPrune(cr *common.ContextRouter, c *gin.Context) {
	vols, err := cr.DB.GetVolumes()
	if err != nil {
		httputil.Error(c, http.StatusInternalServerError, err)
		return
	}

	names := []string{}
	for _, vol := range vols {
		if err := cr.Backend.DeleteVolume(vol); err != nil {
			klog.Warningf("error deleting k8s PVC for volume %s: %s", vol.Name, err)
		}
		if err := cr.DB.DeleteVolume(vol); err != nil {
			klog.Warningf("error deleting volume %s from db: %s", vol.Name, err)
			continue
		}
		names = append(names, vol.Name)
	}

	c.JSON(http.StatusOK, gin.H{
		"VolumesDeleted": names,
		"SpaceReclaimed": 0,
	})
}

// volumeToJSON returns a gin.H containing the details of the given volume.
func volumeToJSON(vol *types.Volume) gin.H {
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
		"UsageData":  nil,
	}
}
