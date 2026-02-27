package libpod

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"k8s.io/klog"

	"github.com/joyrex2001/kubedock/internal/model/types"
	"github.com/joyrex2001/kubedock/internal/server/filter"
	"github.com/joyrex2001/kubedock/internal/server/httputil"
	"github.com/joyrex2001/kubedock/internal/server/routes/common"
)

// NetworkCreateRequest represents the json structure for libpod network creation.
type NetworkCreateRequest struct {
	Name    string            `json:"name"`
	Driver  string            `json:"driver"`
	Labels  map[string]string `json:"labels"`
	Options map[string]string `json:"options"`
}

// NetworkConnectRequest represents the json structure for libpod network connect.
type NetworkConnectRequest struct {
	Container string   `json:"container"`
	Aliases   []string `json:"aliases"`
}

// NetworkDisconnectRequest represents the json structure for libpod network disconnect.
type NetworkDisconnectRequest struct {
	Container string `json:"container"`
}

// NetworkList - list networks.
// https://docs.podman.io/en/latest/_static/api.html?version=v4.2#tag/networks/operation/NetworkListLibpod
// GET "/libpod/networks/json"
func NetworkList(cr *common.ContextRouter, c *gin.Context) {
	netws, err := cr.DB.GetNetworks()
	if err != nil {
		httputil.Error(c, http.StatusInternalServerError, err)
		return
	}
	filtr, err := filter.New(c.Query("filters"))
	if err != nil {
		klog.V(5).Infof("unsupported filter: %s", err)
	}
	res := []gin.H{}
	for _, netw := range netws {
		if filtr.Match(netw) {
			res = append(res, networkToLibpodJSON(cr, netw))
		}
	}
	c.JSON(http.StatusOK, res)
}

// NetworkInfo - inspect a network.
// https://docs.podman.io/en/latest/_static/api.html?version=v4.2#tag/networks/operation/NetworkInspectLibpod
// GET "/libpod/networks/:id/json"
func NetworkInfo(cr *common.ContextRouter, c *gin.Context) {
	id := c.Param("id")
	netw, err := cr.DB.GetNetworkByNameOrID(id)
	if err != nil {
		httputil.Error(c, http.StatusNotFound, err)
		return
	}
	c.JSON(http.StatusOK, networkToLibpodJSON(cr, netw))
}

// NetworkExists - check if a network exists.
// https://docs.podman.io/en/latest/_static/api.html?version=v4.2#tag/networks/operation/NetworkExistsLibpod
// GET "/libpod/networks/:id/exists"
func NetworkExists(cr *common.ContextRouter, c *gin.Context) {
	id := c.Param("id")
	_, err := cr.DB.GetNetworkByNameOrID(id)
	if err != nil {
		httputil.Error(c, http.StatusNotFound, err)
		return
	}
	c.Writer.WriteHeader(http.StatusNoContent)
}

// NetworkCreate - create a network.
// https://docs.podman.io/en/latest/_static/api.html?version=v4.2#tag/networks/operation/NetworkCreateLibpod
// POST "/libpod/networks/create"
func NetworkCreate(cr *common.ContextRouter, c *gin.Context) {
	in := &NetworkCreateRequest{}
	if err := json.NewDecoder(c.Request.Body).Decode(&in); err != nil {
		httputil.Error(c, http.StatusInternalServerError, err)
		return
	}

	// Check if network already exists
	if existing, err := cr.DB.GetNetworkByName(in.Name); err == nil {
		c.JSON(http.StatusOK, networkToLibpodJSON(cr, existing))
		return
	}

	netw := &types.Network{
		Name:   in.Name,
		Labels: in.Labels,
	}
	if err := cr.DB.SaveNetwork(netw); err != nil {
		httputil.Error(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, networkToLibpodJSON(cr, netw))
}

// NetworkDelete - remove a network.
// https://docs.podman.io/en/latest/_static/api.html?version=v4.2#tag/networks/operation/NetworkDeleteLibpod
// DELETE "/libpod/networks/:id"
func NetworkDelete(cr *common.ContextRouter, c *gin.Context) {
	id := c.Param("id")
	netw, err := cr.DB.GetNetworkByNameOrID(id)
	if err != nil {
		httputil.Error(c, http.StatusNotFound, err)
		return
	}

	if netw.IsPredefined() {
		httputil.Error(c, http.StatusForbidden, fmt.Errorf("%s is a pre-defined network and cannot be removed", netw.Name))
		return
	}

	if len(getContainersInNetwork(cr, netw)) != 0 {
		httputil.Error(c, http.StatusConflict, fmt.Errorf("cannot delete network, containers attached"))
		return
	}

	if err := cr.DB.DeleteNetwork(netw); err != nil {
		httputil.Error(c, http.StatusNotFound, err)
		return
	}
	c.JSON(http.StatusOK, []gin.H{networkToLibpodJSON(cr, netw)})
}

// NetworkConnect - connect a container to a network.
// https://docs.podman.io/en/latest/_static/api.html?version=v4.2#tag/networks/operation/NetworkConnectLibpod
// POST "/libpod/networks/:id/connect"
func NetworkConnect(cr *common.ContextRouter, c *gin.Context) {
	in := &NetworkConnectRequest{}
	if err := json.NewDecoder(c.Request.Body).Decode(&in); err != nil {
		httputil.Error(c, http.StatusInternalServerError, err)
		return
	}
	id := c.Param("id")
	netw, err := cr.DB.GetNetworkByNameOrID(id)
	if err != nil {
		httputil.Error(c, http.StatusNotFound, err)
		return
	}
	tainr, err := cr.DB.GetContainerByNameOrID(in.Container)
	if err != nil {
		httputil.Error(c, http.StatusNotFound, err)
		return
	}
	tainr.ConnectNetwork(netw.ID)

	done := map[string]string{}
	for _, a := range tainr.NetworkAliases {
		done[a] = a
	}
	for _, a := range in.Aliases {
		alias := strings.ToLower(a)
		if _, ok := done[alias]; !ok {
			tainr.NetworkAliases = append(tainr.NetworkAliases, alias)
			done[alias] = alias
		}
	}

	if err := cr.DB.SaveContainer(tainr); err != nil {
		httputil.Error(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{})
}

// NetworkDisconnect - disconnect a container from a network.
// https://docs.podman.io/en/latest/_static/api.html?version=v4.2#tag/networks/operation/NetworkDisconnectLibpod
// POST "/libpod/networks/:id/disconnect"
func NetworkDisconnect(cr *common.ContextRouter, c *gin.Context) {
	in := &NetworkDisconnectRequest{}
	if err := json.NewDecoder(c.Request.Body).Decode(&in); err != nil {
		httputil.Error(c, http.StatusInternalServerError, err)
		return
	}
	id := c.Param("id")
	netw, err := cr.DB.GetNetworkByNameOrID(id)
	if err != nil {
		httputil.Error(c, http.StatusNotFound, err)
		return
	}
	tainr, err := cr.DB.GetContainerByNameOrID(in.Container)
	if err != nil {
		httputil.Error(c, http.StatusNotFound, err)
		return
	}
	if netw.IsPredefined() {
		httputil.Error(c, http.StatusInternalServerError, fmt.Errorf("cannot disconnect from predefined network"))
		return
	}
	if err := tainr.DisconnectNetwork(netw.ID); err != nil {
		httputil.Error(c, http.StatusNotFound, err)
		return
	}
	if err := cr.DB.SaveContainer(tainr); err != nil {
		httputil.Error(c, http.StatusInternalServerError, err)
		return
	}
	c.Writer.WriteHeader(http.StatusOK)
}

// NetworkPrune - prune unused networks.
// https://docs.podman.io/en/latest/_static/api.html?version=v4.2#tag/networks/operation/NetworkPruneLibpod
// POST "/libpod/networks/prune"
func NetworkPrune(cr *common.ContextRouter, c *gin.Context) {
	netws, err := cr.DB.GetNetworks()
	if err != nil {
		httputil.Error(c, http.StatusInternalServerError, err)
		return
	}

	pruned := []gin.H{}
	for _, netw := range netws {
		if netw.IsPredefined() || len(getContainersInNetwork(cr, netw)) != 0 {
			continue
		}
		if err := cr.DB.DeleteNetwork(netw); err != nil {
			httputil.Error(c, http.StatusNotFound, err)
			return
		}
		pruned = append(pruned, networkToLibpodJSON(cr, netw))
	}

	c.JSON(http.StatusOK, pruned)
}

// getContainersInNetwork returns all containers connected to the given network.
func getContainersInNetwork(cr *common.ContextRouter, netw *types.Network) map[string]gin.H {
	res := map[string]gin.H{}
	tainrs, err := cr.DB.GetContainers()
	if err == nil {
		for _, tainr := range tainrs {
			if _, ok := tainr.Networks[netw.ID]; ok {
				res[tainr.ID] = gin.H{
					"Name": tainr.Name,
				}
			}
		}
	} else {
		klog.Errorf("error retrieving containers: %s", err)
	}
	return res
}

// networkToLibpodJSON returns a gin.H containing network details in libpod format.
func networkToLibpodJSON(cr *common.ContextRouter, netw *types.Network) gin.H {
	labels := netw.Labels
	if labels == nil {
		labels = map[string]string{}
	}
	return gin.H{
		"name":              netw.Name,
		"id":                netw.ID,
		"driver":            "bridge",
		"network_interface": "kubedock0",
		"created":           netw.Created.Format("2006-01-02T15:04:05Z"),
		"subnets":           []gin.H{{"subnet": "10.88.0.0/16", "gateway": "10.88.0.1"}},
		"ipv6_enabled":      false,
		"internal":          false,
		"dns_enabled":       true,
		"labels":            labels,
	}
}
