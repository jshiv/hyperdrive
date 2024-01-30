package wallet

import (
	"fmt"
	"time"

	"github.com/nodeset-org/hyperdrive/hyperdrive-cli/client"
	"github.com/nodeset-org/hyperdrive/hyperdrive-cli/utils"
	"github.com/nodeset-org/hyperdrive/hyperdrive-cli/utils/terminal"
	"github.com/nodeset-org/hyperdrive/shared/utils/input"
	"github.com/urfave/cli/v2"
)

var (
	generateKeysCountFlag *cli.Uint64Flag = &cli.Uint64Flag{
		Name:    "count",
		Aliases: []string{"c"},
		Usage:   "The number of keys to generate",
	}
)

func generateKeys(c *cli.Context) error {
	// Get Stakewise client
	sw := client.NewStakewiseClientFromCtx(c)

	// Get the count
	var err error
	count := c.Uint64(generateKeysCountFlag.Name)
	if count == 0 {
		countString := utils.Prompt("How many keys would you like to generate?", "^\\d+$", "Invalid count, try again")
		count, err = input.ValidateUint("count", countString)
		if err != nil {
			return fmt.Errorf("invalid count [%s]: %w", countString, err)
		}
	}

	fmt.Println("Note: key generation is an expensive process, this may take a long time! Progress will be printed as each key is generated.")
	fmt.Println()

	// Generate the new keys
	startTime := time.Now()
	latestTime := startTime
	for i := uint64(0); i < count; i++ {
		response, err := sw.Api.Wallet.GenerateKeys(1)
		if err != nil {
			return fmt.Errorf("error generating keys: %w", err)
		}
		if len(response.Data.Pubkeys) == 0 {
			return fmt.Errorf("server did not return any pubkeys")
		}

		elapsed := time.Since(latestTime)
		latestTime = time.Now()
		pubkey := response.Data.Pubkeys[0]
		fmt.Printf("Generated %s (%d/%d) in %s\n", pubkey.Hex(), (i + 1), count, elapsed)
	}
	fmt.Printf("Completed in %s.\n", time.Since(startTime))
	fmt.Println()

	fmt.Println("Regenerating complete deposit data, please wait...")
	regenResponse, err := sw.Api.Wallet.RegenerateDepositData()
	if err != nil {
		fmt.Println("%sThere was an error regenerating your deposit data. Please run it manually with `hyperdrive stakewise wallet regen-deposit-data` to try again.%s", terminal.ColorYellow, terminal.ColorReset)
		return fmt.Errorf("error regenerating deposit data: %w", err)
	}

	// Print the total
	fmt.Printf("Total keys loaded: %s%d%s\n", terminal.ColorGreen, len(regenResponse.Data.Pubkeys), terminal.ColorReset)
	fmt.Println()

	if c.Bool(utils.YesFlag.Name) || utils.Confirm("Would you like to restart the Stakewise Operator service so it loads the new keys and deposit data?") {
		fmt.Println("NYI")
	} else {
		fmt.Println("Please restart the container at your convenience.")
	}
	fmt.Println()

	if !(c.Bool(utils.YesFlag.Name) || utils.Confirm("Would you like to upload the deposit data with the new keys to the NodeSet server, so they can be used for new validator assignments?")) {
		fmt.Println("Please upload the deposit data for all of your keys with `hyperdrive stakewise service upload-deposit-data` when you're ready. Without it, NodeSet won't be able to assign new deposits to your validators.")
		return nil
	}

	// TODO

	fmt.Println("<NYI>")

	return nil
}
