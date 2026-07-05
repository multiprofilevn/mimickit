package standalone

// PluginHandshakeEnv returns the four environment variables mimic chrome's
// "initial check" expects from a genuine adapter go-plugin handshake. Setting
// them on the chrome process replicates that handshake so the check passes and
// chrome enables its client-port fingerprint fetch.
func PluginHandshakeEnv() []string {
	return []string{
		"mlxba=adapter",
		"PLUGIN_MIN_PORT=10000",
		"PLUGIN_MAX_PORT=25000",
		"PLUGIN_PROTOCOL_VERSIONS=1",
	}
}
