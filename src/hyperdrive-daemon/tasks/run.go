package tasks

import (
	"context"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/nodeset-org/hyperdrive/hyperdrive-daemon/common"
	"github.com/rocket-pool/node-manager-core/utils"
	"github.com/rocket-pool/node-manager-core/utils/log"
)

// Config
const (
	tasksInterval time.Duration = time.Minute * 5
	taskCooldown  time.Duration = time.Second * 10

	ErrorColor             = color.FgRed
	WarningColor           = color.FgYellow
	UpdateDepositDataColor = color.FgHiWhite
)

type TaskLoop struct {
	ctx context.Context
	sp  *common.ServiceProvider
	wg  *sync.WaitGroup
}

func NewTaskLoop(sp *common.ServiceProvider, wg *sync.WaitGroup) *TaskLoop {
	return &TaskLoop{
		sp:  sp,
		ctx: sp.GetContext(),
		wg:  wg,
	}
}

// Run daemon
func (t *TaskLoop) Run() error {
	// Initialize loggers
	errorLog := log.NewColorLogger(ErrorColor)

	// Initialize tasks
	// Nothing here yet

	// Run the loop
	t.wg.Add(1)
	go func() {
		for {
			// Check the EC status
			err := t.sp.WaitEthClientSynced(t.ctx, false) // Force refresh the primary / fallback EC status
			if err != nil {
				errorLog.Println(err)
				if utils.SleepWithCancel(t.ctx, taskCooldown) {
					break
				}
				continue
			}

			// Check the BC status
			err = t.sp.WaitBeaconClientSynced(t.ctx, false) // Force refresh the primary / fallback BC status
			if err != nil {
				errorLog.Println(err)
				if utils.SleepWithCancel(t.ctx, taskCooldown) {
					break
				}
				continue
			}

			// Tasks go here

			if utils.SleepWithCancel(t.ctx, tasksInterval) {
				break
			}
		}

		// Signal the task loop is done
		t.wg.Done()
	}()

	/*
		// Run metrics loop
		go func() {
			err := runMetricsServer(sp, log.NewColorLogger(MetricsColor), stateLocker)
			if err != nil {
				errorLog.Println(err)
			}
			wg.Done()
		}()
	*/
	return nil
}
