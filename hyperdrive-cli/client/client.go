package client

import (
	"fmt"
	"path/filepath"

	"github.com/nodeset-org/hyperdrive/hyperdrive-cli/client/api"
	"github.com/nodeset-org/hyperdrive/hyperdrive-cli/utils/context"
	"github.com/nodeset-org/hyperdrive/hyperdrive-cli/utils/terminal"
	"github.com/nodeset-org/hyperdrive/shared/config"
	"github.com/urfave/cli/v2"
)

// Config
const (
	InstallerURL     string = "https://github.com/nodeset-org/hyperdrive/releases/download/%s/install.sh"
	UpdateTrackerURL string = "https://github.com/nodeset-org/hyperdrive/releases/download/%s/install-update-tracker.sh"

	SettingsFile             string = "user-settings.yml"
	BackupSettingsFile       string = "user-settings-backup.yml"
	PrometheusConfigTemplate string = "prometheus.tmpl"
	PrometheusFile           string = "prometheus.yml"

	templatesDir                  string = "templates"
	overrideDir                   string = "override"
	runtimeDir                    string = "runtime"
	defaultFeeRecipientFile       string = "fr-default.tmpl"
	defaultNativeFeeRecipientFile string = "fr-default-env.tmpl"

	templateSuffix    string = ".tmpl"
	composeFileSuffix string = ".yml"

	nethermindAdminUrl string = "http://127.0.0.1:7434"
)

// Hyperdrive client
type Client struct {
	Api     *api.ApiRequester
	Context *context.HyperdriveContext
}

// Create new Hyperdrive client from CLI context without checking for sync status
// Only use this function from commands that may work if the Daemon service doesn't exist
// Most users should call NewClientFromCtx(c).WithStatus() or NewClientFromCtx(c).WithReady()
func NewClientFromCtx(c *cli.Context) *Client {
	snCtx := context.GetHyperdriveContext(c)

	// Set up the default API socket file if it's not specified
	socketPath := filepath.Join(snCtx.ConfigPath, "data", config.SocketFilename)

	client := &Client{
		Api:     api.NewApiRequester(socketPath),
		Context: snCtx,
	}
	return client
}

// Check the status of a newly created client and return it
// Only use this function from commands that may work without the clients being synced-
// most users should use WithReady instead
func (c *Client) WithStatus() (*Client, bool, error) {
	ready, err := c.checkClientStatus()
	if err != nil {
		return nil, false, err
	}

	return c, ready, nil
}

// Check the status of a newly created client and ensure the eth clients are synced and ready
func (c *Client) WithReady() (*Client, error) {
	_, ready, err := c.WithStatus()
	if err != nil {
		return nil, err
	}

	if !ready {
		return nil, fmt.Errorf("clients not ready")
	}

	return c, nil
}

// Check the status of the Execution and Beacon Node(s) and provision the API with them
func (c *Client) checkClientStatus() (bool, error) {
	// Check if the primary clients are up, synced, and able to respond to requests - if not, forces the use of the fallbacks for this command
	response, err := c.Api.Service.ClientStatus()
	if err != nil {
		return false, fmt.Errorf("error checking client status: %w", err)
	}

	ecMgrStatus := response.Data.EcManagerStatus
	bcMgrStatus := response.Data.BcManagerStatus

	// Primary EC and CC are good
	if ecMgrStatus.PrimaryClientStatus.IsSynced && bcMgrStatus.PrimaryClientStatus.IsSynced {
		//c.SetClientStatusFlags(true, false)
		return true, nil
	}

	// Get the status messages
	primaryEcStatus := getClientStatusString(ecMgrStatus.PrimaryClientStatus)
	primaryBcStatus := getClientStatusString(bcMgrStatus.PrimaryClientStatus)
	fallbackEcStatus := getClientStatusString(ecMgrStatus.FallbackClientStatus)
	fallbackBcStatus := getClientStatusString(bcMgrStatus.FallbackClientStatus)

	// Check the fallbacks if enabled
	if ecMgrStatus.FallbackEnabled && bcMgrStatus.FallbackEnabled {
		// Fallback EC and CC are good
		if ecMgrStatus.FallbackClientStatus.IsSynced && bcMgrStatus.FallbackClientStatus.IsSynced {
			fmt.Printf("%sNOTE: primary clients are not ready, using fallback clients...\n\tPrimary EC status: %s\n\tPrimary CC status: %s%s\n\n", terminal.ColorYellow, primaryEcStatus, primaryBcStatus, terminal.ColorReset)
			//c.SetClientStatusFlags(true, true)
			return true, nil
		}

		// Both pairs aren't ready
		fmt.Printf("Error: neither primary nor fallback client pairs are ready.\n\tPrimary EC status: %s\n\tFallback EC status: %s\n\tPrimary CC status: %s\n\tFallback CC status: %s\n", primaryEcStatus, fallbackEcStatus, primaryBcStatus, fallbackBcStatus)
		return false, nil
	}

	// Primary isn't ready and fallback isn't enabled
	fmt.Printf("Error: primary client pair isn't ready and fallback clients aren't enabled.\n\tPrimary EC status: %s\n\tPrimary CC status: %s\n", primaryEcStatus, primaryBcStatus)
	return false, nil
}