package mimickit

import (
	"encoding/gob"
	"encoding/json"
	"net/rpc"

	"github.com/hashicorp/go-plugin"
)

func init() {
	// The adapter server reads Start args as *map[int]interface{}; gob requires
	// explicit registration of the concrete map type.
	gob.Register(map[int]interface{}{})
}

// HandshakeConfig matches adapter.exe's HandshakeConfig.
var HandshakeConfig = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "mlxba",
	MagicCookieValue: "adapter",
}

// pluginKey is the Dispenser key adapter.exe expects.
const pluginKey = "plugin_browser_adapter"

// PluginMap wires the browser adapter plugin under the key adapter.exe dispenses.
var PluginMap = plugin.PluginSet{
	pluginKey: &browserAdapterPlugin{},
}

// browserAdapterV1 is the RPC surface this module drives on adapter.exe.
type browserAdapterV1 interface {
	Version() (string, error)
	Init(browserDirPath string) error
	IsActive(profileID string) (bool, error)
	Start(req StartRequest) (StartResponse, error)
	Stop(profileID string) error
}

// browserAdapterPlugin implements plugin.Plugin (NetRPC, client-only).
type browserAdapterPlugin struct{}

func (p *browserAdapterPlugin) Server(*plugin.MuxBroker) (interface{}, error) {
	return nil, nil
}

func (p *browserAdapterPlugin) Client(_ *plugin.MuxBroker, c *rpc.Client) (interface{}, error) {
	return &browserAdapterRPCClient{client: c}, nil
}

// browserAdapterRPCClient wraps rpc.Client with the adapter's typed methods.
// The RPC service name on the server is "Plugin".
type browserAdapterRPCClient struct {
	client *rpc.Client
}

func (c *browserAdapterRPCClient) Version() (string, error) {
	var reply string
	err := c.client.Call("Plugin.Version", new(interface{}), &reply)
	return reply, err
}

func (c *browserAdapterRPCClient) Init(browserDirPath string) error {
	var reply string
	return c.client.Call("Plugin.Init", browserDirPath, &reply)
}

func (c *browserAdapterRPCClient) IsActive(profileID string) (bool, error) {
	var reply bool
	err := c.client.Call("Plugin.IsActive", profileID, &reply)
	return reply, err
}

// Start marshals the full request to JSON and passes it as map[0], per the
// adapter's non-standard wire format. The server decodes:
//
//	json.Unmarshal((*args)[0].([]uint8), &startRequest)
//
// Reply is a []byte JSON blob ({"Value":"<port>"}) or null when no debug port
// was opened.
func (c *browserAdapterRPCClient) Start(req StartRequest) (StartResponse, error) {
	type wireRequest struct {
		ProfileID      string             `json:"profileId"`
		ProfileDirPath string             `json:"ProfileDirPath"`
		Selenium       bool               `json:"Selenium"`
		Puppeteer      bool               `json:"Puppeteer"`
		Headless       bool               `json:"Headless"`
		FpRaw          []byte             `json:"FpRaw"`
		Overrides      *LauncherOverrides `json:"Overrides,omitempty"`
	}
	js, err := json.Marshal(wireRequest{
		ProfileID:      req.ProfileID,
		ProfileDirPath: req.ProfileDirPath,
		Selenium:       req.Selenium,
		Puppeteer:      req.Puppeteer,
		Headless:       req.Headless,
		FpRaw:          req.FpRaw,
		Overrides:      req.Overrides,
	})
	if err != nil {
		return StartResponse{}, err
	}

	args := map[int]interface{}{0: js}
	var raw []byte
	if err := c.client.Call("Plugin.Start", args, &raw); err != nil {
		return StartResponse{}, err
	}

	// A null (or empty) reply means the browser launched without a debug port.
	var resp StartResponse
	if len(raw) > 4 {
		_ = json.Unmarshal(raw, &resp)
	}
	return resp, nil
}

func (c *browserAdapterRPCClient) Stop(profileID string) error {
	var reply string
	return c.client.Call("Plugin.Stop", profileID, &reply)
}
