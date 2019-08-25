// Copyright (c) of parts are held by the various contributors (see the CLA)
// Licensed under the MIT License. See LICENSE file in the project root for full license information.

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pegnet/pegnet/database"

	"github.com/pegnet/pegnet/balances"

	"github.com/pegnet/pegnet/node"

	"github.com/FactomProject/factom"
	"github.com/pegnet/pegnet/api"
	"github.com/pegnet/pegnet/common"
	"github.com/pegnet/pegnet/mining"
	"github.com/pegnet/pegnet/networkMiner"
	"github.com/pegnet/pegnet/opr"
	"github.com/pegnet/pegnet/polling"
	"github.com/spf13/cobra"
)

var (
	blockRangeStart int64
	blockRangeEnd   int64
)

func init() {
	// Add commands to the root cmd
	rootCmd.AddCommand(getEncoding)
	rootCmd.AddCommand(newAddress)
	rootCmd.AddCommand(grader)
	rootCmd.AddCommand(networkCoordinator)
	rootCmd.AddCommand(networkMinerCmd)
	rootCmd.AddCommand(datasources)
	rootCmd.AddCommand(pegnetNode)

	dataWriter.AddCommand(minerStats)
	dataWriter.AddCommand(priceStats)
	rootCmd.AddCommand(dataWriter)

	burn.Flags().Bool("dryrun", false, "Dryrun creates the TX without actually submitting it to the network.")
	rootCmd.AddCommand(burn)

	dataWriter.PersistentFlags().StringP("output", "o", "stats.csv", "output file for the csv")

	// RPC Wrappers
	getPerformance.Flags().Int64Var(&blockRangeStart, "start", -1, "First block in the block range requested "+
		"(negative numbers are interpreted relative to current block head)")
	getPerformance.Flags().Int64Var(&blockRangeEnd, "end", -1, "Last block in the block range requested "+
		"(negative numbers are ignored)")
	rootCmd.AddCommand(getPerformance)
	rootCmd.AddCommand(getBalance)
}

var getEncoding = &cobra.Command{
	Use:     "getencoding <fct address> [encoding]",
	Short:   "Takes a FCT address and returns an asset encoding (or all encodings) for that FCT address",
	Example: "pegnet getencoding FA2RwVjKe4Jrr7M7E62fZi8mFYqEAoQppmpEDXqAumGkiropSAbk usd\npegnet getencoding FA2RwVjKe4Jrr7M7E62fZi8mFYqEAoQppmpEDXqAumGkiropSAbk all",
	// TODO: Verify this functionality.
	ValidArgs: ValidOwnedFCTAddresses(),

	Long: "" +
		"All Pegnet assets are controlled by the same private key as a FCT address. You can specify\n" +
		"an asset, and this command will give you the encoding for that asset.  If you specify 'all',\n" +
		"or you don't specify an asset, you will get all assets.",
	// TODO: Check the encoding is a valid option
	Args: CombineCobraArgs(cobra.RangeArgs(1, 2), CustomArgOrderValidationBuilder(false, ArgValidatorFCTAddress, ArgValidatorAssetAndAll)),
	Run: func(cmd *cobra.Command, args []string) {
		asset := "all"
		if len(args) == 2 {
			asset = strings.ToLower(args[1])
		}
		assets, err := common.ConvertFCTtoAllPegNetAssets(os.Args[2])
		if err != nil {
			// TODO: Verify this error message?
			fmt.Println("Must provide a valid FCT public key")
			return
		}
		sort.Strings(assets)
		for _, s := range assets {
			if asset == "all" || asset == strings.ToLower(s[1:4]) {
				if strings.Contains(s, "PNT_") {
					fmt.Println("  *", s[1:4], " ", s)
				} else {
					fmt.Println("   ", s[1:4], " ", s)
				}
			}
		}
	},
}

// addGetEncodingCommands adds commands so the autocomplete can fill in the second param
func addGetEncodingCommands() {
	for _, ass := range append([]string{"all"}, common.AllAssets...) {
		getEncoding.AddCommand(&cobra.Command{Use: strings.ToLower(ass), Run: func(cmd *cobra.Command, args []string) {}})
	}
}

var newAddress = &cobra.Command{
	Use:   "newaddress",
	Short: "creates a new FCT address in your wallet, and provides the list of all asset addresses.",
	Long: "Creates a new FCT address and puts the private key for that address in your wallet. All " +
		"PegNet assets are secured by the same private key, and this command provides you the " +
		"human/wallet addresses to use to access those assets",
	Args: cobra.ExactArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		fa, err := factom.GenerateFactoidAddress()
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Print(fa.String(), "\n\n")
		assets, err := common.ConvertFCTtoAllPegNetAssets(fa.String())
		if err != nil {
			fmt.Println(err)
			return
		}
		sort.Strings(assets)
		for _, s := range assets {
			if strings.Contains(s, "PNT_") {
				fmt.Println("  *", s[1:4], " ", s)
			} else {
				fmt.Println("   ", s[1:4], " ", s)
			}
		}
	},
}

var burn = &cobra.Command{
	Use:   "burn <fct address> <fct amount>",
	Short: "Burns the specied amount of FCT into PNT",
	Long: "Burning FCT will turn it into PNT. The PNT burn address is an EC address, and the transaction has " +
		"an input with # of FCT, and an output of 0 EC. This means the entire tx input becomes the fee. " +
		"This command costs FCT, so be careful when using it.",
	Example: "pegnet burn FA3EPZYqodgyEGXNMbiZKE5TS2x2J9wF8J9MvPZb52iGR78xMgCb 1",
	// TODO: Verify this functionality.
	ValidArgs: ValidOwnedFCTAddresses(),
	Args:      CombineCobraArgs(CustomArgOrderValidationBuilder(true, ArgValidatorFCTAddress, ArgValidatorFCTAmount)),
	Run: func(cmd *cobra.Command, args []string) {
		name := "burn"                       // The tmp tx name in the wallet.
		factom.DeleteTransaction(name)       // Delete if existing tmp tx
		defer factom.DeleteTransaction(name) // Any cleanup from errors

		network, err := common.LoadConfigNetwork(Config)
		if err != nil {
			CmdError(cmd, err.Error())
		}

		ownedAddress, fctBurnAmount := args[0], args[1]
		// First see if we own the specified FCT address
		_, err = factom.FetchFactoidAddress(ownedAddress)
		if err != nil {
			CmdError(cmd, err.Error())
		}

		// Get our balance
		factoshiBalance, err := factom.GetFactoidBalance(ownedAddress)
		if err != nil {
			CmdError(cmd, err.Error())
		}

		// Ensure our balance is enough to cover the burn
		factoshiBurn := factom.FactoidToFactoshi(fctBurnAmount)
		if factoshiBurn > uint64(factoshiBalance) {
			fctBal := factom.FactoshiToFactoid(uint64(factoshiBalance))
			CmdErrorf(cmd, "You only have %s FCT, you specified to burn %s\n", fctBal, fctBurnAmount)
		}

		if _, err := factom.NewTransaction(name); err != nil {
			CmdError(cmd, err.Error())
		}
		if _, err := factom.AddTransactionInput(name, ownedAddress, factoshiBurn); err != nil {
			CmdError(cmd, err.Error())
		}

		if _, err := factom.AddTransactionECOutput(name, common.PegnetBurnAddress(network), 0); err != nil {
			CmdError(cmd, err.Error())
		}

		// Signing the tx without a force makes the wallet check the fee amount
		if _, err := factom.SignTransaction(name, false); err != nil {
			// Only care about the insufficient fee error here
			if strings.Contains(err.Error(), "Insufficient Fee") {
				CmdError(cmd, err.Error())
			}
		}

		// We will force the transaction to ignore any fee too high errors
		if _, err := factom.SignTransaction(name, true); err != nil {
			CmdError(cmd, err.Error())
		}

		if dryrun, _ := cmd.Flags().GetBool("dryrun"); dryrun {
			tx, err := factom.ComposeTransaction(name)
			if err != nil {
				CmdError(cmd, err.Error())
			}
			fmt.Println("This transaction was not submitted to the network.")
			fmt.Println(string(tx))
			os.Exit(0)
		}
		tx, err := factom.SendTransaction(name)
		if err != nil {
			CmdError(cmd, err.Error())
		}

		fmt.Println("Burn transaction sent to the network")
		fmt.Printf("Transaction: %s\n", tx.TxID)
	},
}

var datasources = &cobra.Command{
	Use:   "datasources [assets or datasource]",
	Short: "Reads a config and outputs the data sources and their priorities",
	Long: "When setting up a datasource config, this cmd will help you verify your config is set " +
		"correctly. It will also help you ensure you have redudent data sources. " +
		"This command can also provide all datasources, and what assets they support. As well as the " +
		"opposite; given an asset what datasources include it.",
	Example:   "pegnet datasources FCT\npegnet datasources CoinMarketCap",
	Args:      CombineCobraArgs(CustomArgOrderValidationBuilder(false, ArgValidatorAssetOrExchange)),
	ValidArgs: append(common.AllAssets, polling.AllDataSourcesList()...),
	Run: func(cmd *cobra.Command, args []string) {
		ValidateConfig(Config) // Will fatal log if it fails

		// User selected a data source or asset
		if len(args) == 1 {
			if common.AssetListContainsCaseInsensitive(common.AllAssets, args[0]) {
				// Specified an asset
				asset := strings.ToUpper(args[0])

				// Find all exchanges for the asset
				fmt.Printf("Asset : %s\n", asset)

				var sources []string
				for k, v := range polling.AllDataSources {
					if common.AssetListContains(v.SupportedPegs(), asset) {
						sources = append(sources, k)
					}
				}
				fmt.Printf("Datasources : %v\n", sources)
			} else if common.AssetListContainsCaseInsensitive(polling.AllDataSourcesList(), args[0]) {
				// Specified an exchange
				source := polling.CorrectCasing(args[0])
				s, ok := polling.AllDataSources[source]
				if !ok {
					CmdErrorf(cmd, "%s is not a supported datasource", args[0])
				}

				fmt.Printf("Datasource : %s\n", s.Name())
				fmt.Printf("Datasource URL : %s\n", s.Url())
				fmt.Printf("Supported peg pricing\n")
				for _, asset := range s.SupportedPegs() {
					fmt.Printf("\t%s\n", asset)
				}
			} else {
				// Should never happen
				fmt.Println("This should never happen. The provided argument is invalid")
			}
			return
		}

		// Default to printing everything
		d := polling.NewDataSources(Config)

		// Time to print
		fmt.Println("Data sources in priority order")
		fmt.Printf("\t%s\n", d.PriorityListString())

		fmt.Println()
		fmt.Println("Assets and their data source order. The order left to right is the fallback order.")
		for _, asset := range common.AllAssets {
			if asset == "PNT" {
				continue
			}
			str := d.AssetPriorityString(asset)
			fmt.Printf("\t%4s (%d) : %s\n", asset, len(d.AssetSources[asset]), str)
		}
	},
}

// TODO: Flesh this out, just using it for testing the miner
var grader = &cobra.Command{
	Use: "grader ",
	Run: func(cmd *cobra.Command, args []string) {
		opr.InitLX()
		ValidateConfig(Config) // Will fatal log if it fails

		monitor := common.GetMonitor()
		monitor.SetTimeout(time.Duration(Timeout) * time.Second)

		go func() {
			errListener := monitor.NewErrorListener()
			err := <-errListener
			panic("Monitor threw error: " + err.Error())
		}()

		b := balances.NewBalanceTracker()
		q := LaunchGrader(Config, monitor, b, context.Background(), true)

		alert := q.GetAlert("cmd")

		for {
			select {
			case a := <-alert:
				fmt.Println(a)
			}
		}
	},
}

// -------------------------------------------------------------
// RPC Wrapper Commands

// sendRequestAndPrintResults does exactly what it says, prints in JSON for now (pipe output to jq manually)
// TODO: pretty print instead
func sendRequestAndPrintResults(req *api.PostRequest) {
	response, err := api.SendRequest(req)
	if err != nil {
		fmt.Printf("Failed to make request: %v\n", err)
	}
	responseJSON, _ := json.Marshal(response)
	fmt.Println(string(responseJSON))
}

var getPerformance = &cobra.Command{
	Use:   "performance <miner identifier> [--start START_BLOCK] [--end END_BLOCK]",
	Short: "Returns the performance of the miner at the specified identifier.",
	Long: "Every block, miners submissions are first ranked according to hash-power/difficulty computed, then by " +
		"the quality of their pricing estimates.\nThis function returns statistics to evaluate where a given miner " +
		"stands in the rankings for both categories over a specific range of blocks.",
	Example: "pegnet performance prototypeminer001 --start=-144",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		blockRange := api.BlockRange{Start: &blockRangeStart}
		if blockRangeEnd > 0 {
			blockRange.End = &blockRangeEnd
		}
		req := api.PostRequest{
			Method: "performance",
			Params: api.PerformanceParameters{
				BlockRange: blockRange,
				DigitalID:  id,
			},
		}
		sendRequestAndPrintResults(&req)
	},
}

var getBalance = &cobra.Command{
	Use:     "balance <type> <factoid address>",
	Short:   "Returns the balance for the given asset type and Factoid address",
	Example: "pegnet balance PNT FA2jK2HcLnRdS94dEcU27rF3meoJfpUcZPSinpb7AwQvPRY6RL1Q",
	Args:    CombineCobraArgs(CustomArgOrderValidationBuilder(true, ArgValidatorAsset, ArgValidatorFCTAddress)),
	Run: func(cmd *cobra.Command, args []string) {
		ticker := args[0]
		address := args[1]

		networkString, err := common.LoadConfigNetwork(Config)
		if err != nil {
			fmt.Println("Error: invalid network string")
		}
		pntAddress, err := common.ConvertFCTtoPegNetAsset(networkString, ticker, address)
		if err != nil {
			fmt.Println("Error: invalid Factoid address")
		}
		req := api.PostRequest{
			Method: "balance",
			Params: api.GenericParameters{
				Address: &pntAddress,
			},
		}
		sendRequestAndPrintResults(&req)
	},
}

var networkCoordinator = &cobra.Command{
	Use:   "netcoordinator",
	Short: "Enables running of remote miners against this machine",
	Long: "The net coordinator will facilitate all communication with factomd and remote data sources. " +
		"Remote miners therefore can directly and ONLY communicate with the coordinator.",
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithCancel(context.Background())
		common.GlobalExitHandler.AddCancel(cancel)
		ValidateConfig(Config) // Will fatal log if it fails

		b := balances.NewBalanceTracker()
		// Services
		monitor := LaunchFactomMonitor(Config)
		grader := LaunchGrader(Config, monitor, b, ctx, true)
		statTracker := LaunchStatistics(Config, ctx)
		apiserver := LaunchAPI(Config, statTracker, grader, b, true)
		LaunchControlPanel(Config, ctx, monitor, statTracker, b)
		var _ = apiserver

		srv := networkMiner.NewMiningServer(Config, monitor, grader, statTracker)
		go srv.Listen()
		srv.ForwardMonitorEvents()

		var _ = cancel
	},
}

var networkMinerCmd = &cobra.Command{
	Use: "netminer",
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithCancel(context.Background())
		common.GlobalExitHandler.AddCancel(cancel)
		ValidateConfig(Config) // Will fatal log if it fails

		cl := networkMiner.NewMiningClient(Config)
		err := cl.Connect()
		if err != nil {
			panic(err)
		}
		// Pass the cancel func to stop the system
		go cl.Listen(cancel)
		go cl.Forwarder()
		monitor, grader, oprMaker := cl.Listeners()

		go func() {
			errListener := monitor.NewErrorListener()
			err := <-errListener
			panic("Monitor threw error: " + err.Error())
		}()

		// Services
		statTracker := LaunchStatistics(Config, ctx)
		// TODO: Api on remote? CP on remote?
		//apiserver := LaunchAPI(Config, statTracker)
		//LaunchControlPanel(Config, ctx, monitor, statTracker)
		//var _ = apiserver

		cl.UpstreamStats = statTracker.GetUpstream("netcoord") // Send stats upstream

		coord := mining.NewNetworkedMiningCoordinatorFromConfig(Config, monitor, grader, statTracker)
		coord.OPRMaker = oprMaker
		coord.FactomEntryWriter = cl.NewEntryForwarder()
		err = coord.InitMinters()
		if err != nil {
			panic(err)
		}

		coord.LaunchMiners(ctx) // Inf loop unless context cancelled

		// Calling cancel() will cancel the stat tracker collection AND the miners
		var _ = cancel
	},
}

var pegnetNode = &cobra.Command{
	Use:   "node",
	Short: "Runs a pegnet node",
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithCancel(context.Background())
		common.GlobalExitHandler.AddCancel(cancel)
		ValidateConfig(Config) // Will fatal log if it fails
		b := balances.NewBalanceTracker()

		// Services
		monitor := LaunchFactomMonitor(Config)
		grader := LaunchGrader(Config, monitor, b, ctx, false)

		pegnetnode, err := node.NewPegnetNode(Config, monitor, grader)
		if err != nil {
			CmdError(cmd, err)
		}
		common.GlobalExitHandler.AddExit(pegnetnode.Close)

		go pegnetnode.Run(ctx)

		var _ = cancel
		apiserver := LaunchAPI(Config, nil, grader, b, false)
		apiserver.Mux.Handle("/node/v1", pegnetnode.APIMux())
		// Let's add the pegnet node timeseries to the handle
		apiport, err := Config.Int(common.ConfigAPIPort)
		if err != nil {
			CmdError(cmd, err)
		}
		apiserver.Listen(apiport)
		var _ = apiserver

	},
}

var dataWriter = &cobra.Command{
	Use:   "csv <data_request>",
	Short: "Ability to create csvs for some analysis",
	Long: "Adds the ability to run analysis commands on a network and output csvs. " +
		"This is helpful as this cmd already has access to the pegnet internals, and could " +
		"help us create analysis tooling.",
	Example: "csv minerstats",
}

// priceStats is used to analyse data sources chosen
var priceStats = &cobra.Command{
	Use:   "pricestats <height>",
	Short: "Creates a csv showing the price related stats from the blocks on chain.",
	Long: "Will output each opr and a column per asset. Each column is the % difference from the average of the " +
		"entire set. They are ordered in self reported difficulty order.",
	Args:    cobra.ExactArgs(1),
	Example: "csv pricestats",
	Run: func(cmd *cobra.Command, args []string) {
		height, err := strconv.Atoi(args[0])
		if err != nil {
			CmdError(cmd, err)
		}

		path, err := cmd.Flags().GetString("output")
		if err != nil {
			CmdError(cmd, err)
		}

		c, err := opr.NewChainRecorder(Config, path)
		if err != nil {
			CmdError(cmd, err)
		}

		// Use a mapdb over a ldb so we can get the full oprs
		// vs just graded
		ldb := database.NewMapDb()

		err = c.WritePriceCSV(ldb, int64(height))
		if err != nil {
			CmdError(cmd, err)
		}
	},
}

var minerStats = &cobra.Command{
	Use:   "minerstats",
	Short: "Creates a csv showing the miner related stats from the blocks on chain.",
	Long: "Will let you analyze the difficulty changes over time, and test difficulty targeting" +
		" against on chain data.",
	Example: "csv minerstats",
	Run: func(cmd *cobra.Command, args []string) {
		// minerstats.csv
		path, err := cmd.Flags().GetString("output")
		if err != nil {
			CmdError(cmd, err)
		}

		c, err := opr.NewChainRecorder(Config, path)
		if err != nil {
			CmdError(cmd, err)
		}

		err = c.WriteMinerCSV()
		if err != nil {
			CmdError(cmd, err)
		}
	},
}
