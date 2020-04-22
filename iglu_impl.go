package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	sdk "github.com/Nacdlow/plugin-sdk"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
)

var (
	token string = ""
)

// LifxPlugin is an implementation of IgluPlugin
type LifxPlugin struct {
	logger hclog.Logger
}

func (g *LifxPlugin) OnLoad() error {
	g.logger.Debug("Loading LIFX integration plugin!")
	return nil
}

func (g *LifxPlugin) GetManifest() sdk.PluginManifest {
	return sdk.PluginManifest{
		Id:      "lifx",
		Name:    "LIFX Integration Plugin",
		Author:  "Nacdlow",
		Version: "v0.1.0",
	}
}

func (g *LifxPlugin) OnDeviceToggle(id string, status bool) error {
	var statusStr string
	if status {
		statusStr = "on"
	} else {
		statusStr = "off"
	}

	body := strings.NewReader(`power=` + statusStr)
	req, err := http.NewRequest("PUT", "https://api.lifx.com/v1/lights/id:"+id+"/state", body)
	if err != nil {
		g.logger.Error(fmt.Sprintf("on toggle: error creating new request: %v", err))
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		g.logger.Error(fmt.Sprintf("on toggle: error getting response: %v", err))
		return err
	}
	defer resp.Body.Close()
	if deviceStates == nil {
		deviceStates = make(map[string]bool)
	}
	deviceStates[id] = status
	return nil
}

func (g *LifxPlugin) GetDeviceStatus(id string) bool {
	updateLightStatus(g)
	return deviceStates[id]
}

func (g *LifxPlugin) GetPluginConfiguration() []sdk.PluginConfig {
	return []sdk.PluginConfig{
		{Title: "Personal Integration Token", Description: "Get your token from cloud.lifx.com!", Key: "pak",
			Type: sdk.StringValue, IsUserSpecific: false},
	}
}

func (g *LifxPlugin) OnConfigurationUpdate(config []sdk.ConfigKV) {
}

type ListDevices []struct {
	ID        string `json:"id"`
	UUID      string `json:"uuid"`
	Label     string `json:"label"`
	Connected bool   `json:"connected"`
	Power     string `json:"power"`
	Color     struct {
		Hue        int `json:"hue"`
		Saturation int `json:"saturation"`
		Kelvin     int `json:"kelvin"`
	} `json:"color"`
	Brightness int    `json:"brightness"`
	Effect     string `json:"effect"`
	Group      struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"group"`
	Location struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"location"`
	Product struct {
		Name         string `json:"name"`
		Identifier   string `json:"identifier"`
		Company      string `json:"company"`
		Capabilities struct {
			HasColor             bool `json:"has_color"`
			HasVariableColorTemp bool `json:"has_variable_color_temp"`
			HasIr                bool `json:"has_ir"`
			HasChain             bool `json:"has_chain"`
			HasMatrix            bool `json:"has_matrix"`
			HasMultizone         bool `json:"has_multizone"`
			MinKelvin            int  `json:"min_kelvin"`
			MaxKelvin            int  `json:"max_kelvin"`
		} `json:"capabilities"`
	} `json:"product"`
	LastSeen         time.Time `json:"last_seen"`
	SecondsSinceSeen int       `json:"seconds_since_seen"`
}

func getAllDevices(g *LifxPlugin) *ListDevices {
	req, err := http.NewRequest("GET", "https://api.lifx.com/v1/lights/all", nil)
	if err != nil {
		g.logger.Error(fmt.Sprintf("get available: error getting new request: %v", err))
		return nil
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		g.logger.Error(fmt.Sprintf("get available: error getting response: %v", err))
		return nil
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		g.logger.Error(fmt.Sprintf("get available: error reading body: %v", err))
		return nil
	}
	var list ListDevices
	err = json.Unmarshal(body, &list)
	if err != nil {
		g.logger.Error(fmt.Sprintf("get available: error unmarshalling: %v", err))
		return nil
	}
	return &list
}

var (
	lastPull     time.Time
	available    []sdk.AvailableDevice
	deviceStates map[string]bool
)

func updateLightStatus(g *LifxPlugin) {
	if deviceStates == nil {
		deviceStates = make(map[string]bool)
	}
	if lastPull.IsZero() || time.Since(lastPull) > 30*time.Second {
		lastPull = time.Now()
		list := getAllDevices(g)
		if list == nil {
			return
		}
		available = []sdk.AvailableDevice{}
		for _, d := range *list {
			available = append(available, sdk.AvailableDevice{
				UniqueID:         d.ID,
				ManufacturerName: d.Product.Company,
				ModelName:        d.Product.Name,
				Type:             0,
			})
			deviceStates[d.ID] = (d.Power == "on")
		}
	}
}

func (g *LifxPlugin) GetAvailableDevices() []sdk.AvailableDevice {
	updateLightStatus(g)
	return available
}

func (g *LifxPlugin) GetWebExtensions() []sdk.WebExtension {
	return []sdk.WebExtension{}
}

var handshakeConfig = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "IGLU_PLUGIN",
	MagicCookieValue: "MzlK0OGpIRs",
}

func main() {
	logger := hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Trace,
		Output:     os.Stderr,
		JSONFormat: true,
	})

	test := &LifxPlugin{
		logger: logger,
	}

	// pluginMap is the map of plugins we can dispense.
	var pluginMap = map[string]plugin.Plugin{
		"iglu_plugin": &sdk.IgluPlugin{Impl: test},
	}

	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: handshakeConfig,
		Plugins:         pluginMap,
	})
}
