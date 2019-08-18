package cmd

import (
	"context"
	"time"

	"github.com/pegnet/pegnet/api"
	"github.com/pegnet/pegnet/common"
	"github.com/pegnet/pegnet/controlPanel"
	"github.com/pegnet/pegnet/mining"
	"github.com/pegnet/pegnet/opr"
	"github.com/zpatrick/go-config"
)

func LaunchFactomMonitor(config *config.Config) *common.Monitor {
	monitor := common.GetMonitor()
	monitor.SetTimeout(time.Duration(Timeout) * time.Second)

	go func() {
		errListener := monitor.NewErrorListener()
		err := <-errListener
		panic("Monitor threw error: " + err.Error())
	}()

	return monitor
}

func LaunchGrader(config *config.Config, monitor *common.Monitor) *opr.Grader {
	grader := opr.NewGrader()
	go grader.Run(config, monitor)
	return grader
}

func LaunchStatistics(config *config.Config, ctx context.Context) *mining.GlobalStatTracker {
	statTracker := mining.NewGlobalStatTracker()

	go statTracker.Collect(ctx) // Will stop collecting on ctx cancel
	return statTracker
}

func LaunchAPI(config *config.Config, stats *mining.GlobalStatTracker) *api.APIServer {
	s := api.NewApiServer()
	port, err := config.Int("Miner.APIPort")
	if err != nil || port < 1 || port > 65536 {
		panic("Error parsing APIPort value in config file")
	}
	
	go s.Listen(port)
	return s
}

func LaunchControlPanel(config *config.Config, ctx context.Context, monitor common.IMonitor, stats *mining.GlobalStatTracker) *controlPanel.ControlPanel {
	cp := controlPanel.NewControlPanel(config, monitor, stats)
	go cp.ServeControlPanel()
	return cp
}

func LaunchMiners(config *config.Config, ctx context.Context, monitor common.IMonitor, grader opr.IGrader, stats *mining.GlobalStatTracker) *mining.MiningCoordinator {
	coord := mining.NewMiningCoordinatorFromConfig(config, monitor, grader, stats)
	err := coord.InitMinters()
	if err != nil {
		panic(err)
	}

	// TODO: Make this unblocking
	coord.LaunchMiners(ctx) // Inf loop unless context cancelled
	return coord
}
