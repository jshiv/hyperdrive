package common

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/nodeset-org/hyperdrive/daemon-utils/services"
	"github.com/rocket-pool/node-manager-core/eth"
)

const (
	EthClientSyncTimeout    int64 = 8 // 8 seconds
	BeaconClientSyncTimeout int64 = 8 // 8 seconds

	ethClientStatusRefreshInterval time.Duration = 60 * time.Second
	ethClientSyncPollInterval      time.Duration = 5 * time.Second
	beaconClientSyncPollInterval   time.Duration = 5 * time.Second
)

var (
	ethClientSyncLock    sync.Mutex
	beaconClientSyncLock sync.Mutex
)

func (sp *ServiceProvider) RequireNodeAddress() error {
	status, err := sp.GetWallet().GetStatus()
	if err != nil {
		return err
	}
	if !status.Address.HasAddress {
		return errors.New("The node currently does not have an address set. Please run 'hyperdrive wallet init' and try again.")
	}
	return nil
}

func (sp *ServiceProvider) RequireWalletReady() error {
	status, err := sp.GetWallet().GetStatus()
	if err != nil {
		return err
	}
	if !status.Address.HasAddress {
		return errors.New("The node currently does not have an address set. Please run 'hyperdrive wallet init' and try again.")
	}
	if !status.Wallet.IsLoaded {
		if status.Wallet.IsOnDisk {
			if !status.Password.IsPasswordSaved {
				return errors.New("The node has a node wallet on disk but does not have the password for it loaded. Please run `hyperdrive wallet set-password` to load it.")
			}
			return errors.New("The node has a node wallet and a password on disk but there was an error loading it - perhaps the password is incorrect? Please check the daemon logs for more information.")
		}
		return errors.New("The node currently does not have a node wallet keystore. Please run 'hyperdrive wallet init' and try again.")
	}
	if status.Wallet.WalletAddress != status.Address.NodeAddress {
		return errors.New("The node's wallet keystore does not match the node address. This node is currently in read-only mode.")
	}
	return nil
}

func (sp *ServiceProvider) RequireEthClientSynced(ctx context.Context) error {
	ethClientSynced, err := sp.waitEthClientSynced(ctx, false, EthClientSyncTimeout)
	if err != nil {
		return err
	}
	if !ethClientSynced {
		return errors.New("The Execution client is currently syncing. Please try again later.")
	}
	return nil
}

func (sp *ServiceProvider) RequireBeaconClientSynced(ctx context.Context) error {
	beaconClientSynced, err := sp.waitBeaconClientSynced(ctx, false, BeaconClientSyncTimeout)
	if err != nil {
		return err
	}
	if !beaconClientSynced {
		return errors.New("The Beacon client is currently syncing. Please try again later.")
	}
	return nil
}

// Wait for the Executon client to sync; timeout of 0 indicates no timeout
func (sp *ServiceProvider) WaitEthClientSynced(ctx context.Context, verbose bool) error {
	_, err := sp.waitEthClientSynced(ctx, verbose, 0)
	return err
}

// Wait for the Beacon client to sync; timeout of 0 indicates no timeout
func (sp *ServiceProvider) WaitBeaconClientSynced(ctx context.Context, verbose bool) error {
	_, err := sp.waitBeaconClientSynced(ctx, verbose, 0)
	return err
}

// Check if the primary and fallback Execution clients are synced
// TODO: Move this into ec-manager and stop exposing the primary and fallback directly...
func (sp *ServiceProvider) checkExecutionClientStatus(ctx context.Context) (bool, eth.IExecutionClient, error) {
	// Check the EC status
	ecMgr := sp.GetEthClient()
	mgrStatus := ecMgr.CheckStatus(ctx)
	if ecMgr.IsPrimaryReady() {
		return true, nil, nil
	}

	// If the primary isn't synced but there's a fallback and it is, return true
	if ecMgr.IsFallbackReady() {
		if mgrStatus.PrimaryClientStatus.Error != "" {
			log.Printf("Primary execution client is unavailable (%s), using fallback execution client...\n", mgrStatus.PrimaryClientStatus.Error)
		} else {
			log.Printf("Primary execution client is still syncing (%.2f%%), using fallback execution client...\n", mgrStatus.PrimaryClientStatus.SyncProgress*100)
		}
		return true, nil, nil
	}

	// If neither is synced, go through the status to figure out what to do

	// Is the primary working and syncing? If so, wait for it
	if mgrStatus.PrimaryClientStatus.IsWorking && mgrStatus.PrimaryClientStatus.Error == "" {
		log.Printf("Fallback execution client is not configured or unavailable, waiting for primary execution client to finish syncing (%.2f%%)\n", mgrStatus.PrimaryClientStatus.SyncProgress*100)
		return false, ecMgr.GetPrimaryExecutionClient(), nil
	}

	// Is the fallback working and syncing? If so, wait for it
	if mgrStatus.FallbackEnabled && mgrStatus.FallbackClientStatus.IsWorking && mgrStatus.FallbackClientStatus.Error == "" {
		log.Printf("Primary execution client is unavailable (%s), waiting for the fallback execution client to finish syncing (%.2f%%)\n", mgrStatus.PrimaryClientStatus.Error, mgrStatus.FallbackClientStatus.SyncProgress*100)
		return false, ecMgr.GetFallbackExecutionClient(), nil
	}

	// If neither client is working, report the errors
	if mgrStatus.FallbackEnabled {
		return false, nil, fmt.Errorf("Primary execution client is unavailable (%s) and fallback execution client is unavailable (%s), no execution clients are ready.", mgrStatus.PrimaryClientStatus.Error, mgrStatus.FallbackClientStatus.Error)
	}

	return false, nil, fmt.Errorf("Primary execution client is unavailable (%s) and no fallback execution client is configured.", mgrStatus.PrimaryClientStatus.Error)
}

// Check if the primary and fallback Beacon clients are synced
func (sp *ServiceProvider) checkBeaconClientStatus(ctx context.Context) (bool, error) {
	// Check the BC status
	bcMgr := sp.GetBeaconClient()
	mgrStatus := bcMgr.CheckStatus(ctx)
	if bcMgr.IsPrimaryReady() {
		return true, nil
	}

	// If the primary isn't synced but there's a fallback and it is, return true
	if bcMgr.IsFallbackReady() {
		if mgrStatus.PrimaryClientStatus.Error != "" {
			log.Printf("Primary Beacon Node is unavailable (%s), using fallback Beacon Node...\n", mgrStatus.PrimaryClientStatus.Error)
		} else {
			log.Printf("Primary Beacon Node is still syncing (%.2f%%), using fallback Beacon Node...\n", mgrStatus.PrimaryClientStatus.SyncProgress*100)
		}
		return true, nil
	}

	// If neither is synced, go through the status to figure out what to do

	// Is the primary working and syncing? If so, wait for it
	if mgrStatus.PrimaryClientStatus.IsWorking && mgrStatus.PrimaryClientStatus.Error == "" {
		log.Printf("Fallback Beacon Node is not configured or unavailable, waiting for primary Beacon Node to finish syncing (%.2f%%)\n", mgrStatus.PrimaryClientStatus.SyncProgress*100)
		return false, nil
	}

	// Is the fallback working and syncing? If so, wait for it
	if mgrStatus.FallbackEnabled && mgrStatus.FallbackClientStatus.IsWorking && mgrStatus.FallbackClientStatus.Error == "" {
		log.Printf("Primary cosnensus client is unavailable (%s), waiting for the fallback Beacon Node to finish syncing (%.2f%%)\n", mgrStatus.PrimaryClientStatus.Error, mgrStatus.FallbackClientStatus.SyncProgress*100)
		return false, nil
	}

	// If neither client is working, report the errors
	if mgrStatus.FallbackEnabled {
		return false, fmt.Errorf("Primary Beacon Node is unavailable (%s) and fallback Beacon Node is unavailable (%s), no Beacon Nodes are ready.", mgrStatus.PrimaryClientStatus.Error, mgrStatus.FallbackClientStatus.Error)
	}

	return false, fmt.Errorf("Primary Beacon Node is unavailable (%s) and no fallback Beacon Node is configured.", mgrStatus.PrimaryClientStatus.Error)
}

// Wait for the primary or fallback Execution client to be synced
func (sp *ServiceProvider) waitEthClientSynced(ctx context.Context, verbose bool, timeout int64) (bool, error) {
	// Prevent multiple waiting goroutines from requesting sync progress
	ethClientSyncLock.Lock()
	defer ethClientSyncLock.Unlock()

	synced, clientToCheck, err := sp.checkExecutionClientStatus(ctx)
	if err != nil {
		return false, err
	}
	if synced {
		return true, nil
	}

	// Get wait start time
	startTime := time.Now()

	// Get EC status refresh time
	ecRefreshTime := startTime

	// Wait for sync
	for {
		// Check timeout
		if (timeout > 0) && (time.Since(startTime).Seconds() > float64(timeout)) {
			return false, nil
		}

		// Check if the EC status needs to be refreshed
		if time.Since(ecRefreshTime) > ethClientStatusRefreshInterval {
			log.Println("Refreshing primary / fallback execution client status...")
			ecRefreshTime = time.Now()
			synced, clientToCheck, err = sp.checkExecutionClientStatus(ctx)
			if err != nil {
				return false, err
			}
			if synced {
				return true, nil
			}
		}

		// Get sync progress
		progress, err := clientToCheck.SyncProgress(context.Background())
		if err != nil {
			return false, err
		}

		// Check sync progress
		if progress != nil {
			if verbose {
				p := float64(progress.CurrentBlock-progress.StartingBlock) / float64(progress.HighestBlock-progress.StartingBlock)
				if p > 1 {
					log.Println("Eth 1.0 node syncing...")
				} else {
					log.Printf("Eth 1.0 node syncing: %.2f%%\n", p*100)
				}
			}
		} else {
			// Eth 1 client is not in "syncing" state but may be behind head
			// Get the latest block it knows about and make sure it's recent compared to system clock time
			isUpToDate, _, err := services.IsSyncWithinThreshold(clientToCheck)
			if err != nil {
				return false, err
			}
			// Only return true if the last reportedly known block is within our defined threshold
			if isUpToDate {
				return true, nil
			}
		}

		// Pause before next poll
		time.Sleep(ethClientSyncPollInterval)
	}
}

// Wait for the primary or fallback Beacon client to be synced
func (sp *ServiceProvider) waitBeaconClientSynced(ctx context.Context, verbose bool, timeout int64) (bool, error) {
	// Prevent multiple waiting goroutines from requesting sync progress
	beaconClientSyncLock.Lock()
	defer beaconClientSyncLock.Unlock()

	synced, err := sp.checkBeaconClientStatus(ctx)
	if err != nil {
		return false, err
	}
	if synced {
		return true, nil
	}

	// Get wait start time
	startTime := time.Now()

	// Get BC status refresh time
	bcRefreshTime := startTime

	// Wait for sync
	for {
		// Check timeout
		if (timeout > 0) && (time.Since(startTime).Seconds() > float64(timeout)) {
			return false, nil
		}

		// Check if the BC status needs to be refreshed
		if time.Since(bcRefreshTime) > ethClientStatusRefreshInterval {
			log.Println("Refreshing primary / fallback Beacon Node status...")
			bcRefreshTime = time.Now()
			synced, err = sp.checkBeaconClientStatus(ctx)
			if err != nil {
				return false, err
			}
			if synced {
				return true, nil
			}
		}

		// Get sync status
		syncStatus, err := sp.GetBeaconClient().GetSyncStatus(ctx)
		if err != nil {
			return false, err
		}

		// Check sync status
		if syncStatus.Syncing {
			if verbose {
				log.Println("Eth 2.0 node syncing: %.2f%%\n", syncStatus.Progress*100)
			}
		} else {
			return true, nil
		}

		// Pause before next poll
		time.Sleep(beaconClientSyncPollInterval)
	}
}
