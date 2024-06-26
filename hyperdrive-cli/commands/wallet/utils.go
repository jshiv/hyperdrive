package wallet

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/nodeset-org/hyperdrive/hyperdrive-cli/commands/wallet/bip39"
	"github.com/nodeset-org/hyperdrive/hyperdrive-cli/utils"
	"github.com/nodeset-org/hyperdrive/hyperdrive-cli/utils/terminal"
	"github.com/rocket-pool/node-manager-core/utils/input"
	"github.com/urfave/cli/v2"
)

var (
	PasswordFlag *cli.StringFlag = &cli.StringFlag{
		Name:    "password",
		Aliases: []string{"p"},
		Usage:   "The password to secure the wallet with (if not already set)",
	}
	SavePasswordFlag *cli.StringFlag = &cli.StringFlag{
		Name:    "save-password",
		Aliases: []string{"s"},
		Usage:   "Save the node wallet password to disk, so the wallet can be automatically reloaded upon starting up",
	}
	derivationPathFlag *cli.StringFlag = &cli.StringFlag{
		Name:    "derivation-path",
		Aliases: []string{"d"},
		Usage:   "Specify the derivation path for the wallet.\nOmit this flag (or leave it blank) for the default of \"m/44'/60'/0'/0/%d\" (where %d is the index).\nSet this to \"ledgerLive\" to use Ledger Live's path of \"m/44'/60'/%d/0/0\".\nSet this to \"mew\" to use MyEtherWallet's path of \"m/44'/60'/0'/%d\".\nFor custom paths, simply enter them here.",
	}
	walletIndexFlag *cli.Uint64Flag = &cli.Uint64Flag{
		Name:    "wallet-index",
		Aliases: []string{"i"},
		Usage:   "Specify the index to use with the derivation path when recovering your wallet",
		Value:   0,
	}
	mnemonicFlag *cli.StringFlag = &cli.StringFlag{
		Name:    "mnemonic",
		Aliases: []string{"m"},
		Usage:   "The mnemonic phrase to recover the wallet from",
	}
	addressFlag *cli.StringFlag = &cli.StringFlag{
		Name:    "address",
		Aliases: []string{"a"},
		Usage:   "If you are recovering a wallet that was not generated by Hyperdrive and don't know the derivation path or index of it, enter the address here. Hyperdrive will search through its library of paths and indices to try to find it.",
	}
)

// Prompt for a new wallet password
func PromptNewPassword() string {
	for {
		password := utils.PromptPassword(
			"Please enter a password to secure your wallet with:",
			fmt.Sprintf("^.{%d,}$", input.MinPasswordLength),
			fmt.Sprintf("Your password must be at least %d characters long. Please try again:", input.MinPasswordLength),
		)
		confirmation := utils.PromptPassword("Please confirm your password:", "^.*$", "")
		if password == confirmation {
			return password
		}
		fmt.Println("Password confirmation does not match.")
		fmt.Println("")
	}
}

// Prompt for the password to a wallet that already exists
func PromptExistingPassword() string {
	for {
		password := utils.PromptPassword(
			"Please enter the password your wallet was originally secured with:",
			fmt.Sprintf("^.{%d,}$", input.MinPasswordLength),
			fmt.Sprintf("Your password must be at least %d characters long. Please try again:", input.MinPasswordLength),
		)
		return password
	}
}

// Prompt for a recovery mnemonic phrase
func PromptMnemonic() string {
	for {
		lengthInput := utils.Prompt(
			"Please enter the "+terminal.ColorBold+"number"+terminal.ColorReset+" of words in your mnemonic phrase (24 by default):",
			"^[1-9][0-9]*$",
			"Please enter a valid number.")

		length, err := strconv.Atoi(lengthInput)
		if err != nil {
			fmt.Println("Please enter a valid number.")
			continue
		}

		mv := bip39.Create(length)
		if mv == nil {
			fmt.Println("Please enter a valid mnemonic length.")
			continue
		}

		i := 0
		for !mv.Filled() {
			prompt := fmt.Sprintf("Enter %sWord Number %d%s of your mnemonic:", terminal.ColorBold, i+1, terminal.ColorReset)
			word := utils.PromptPassword(prompt, "^[a-zA-Z]+$", "Please enter a single word only.")

			if err := mv.AddWord(strings.ToLower(word)); err != nil {
				fmt.Println("Inputted word not valid, please retry.")
				continue
			}

			i++
		}

		mnemonic, err := mv.Finalize()
		if err != nil {
			fmt.Printf("Error validating mnemonic: %s\n", err)
			fmt.Println("Please try again.")
			fmt.Println("")
			continue
		}

		return mnemonic
	}
}

// Confirm a recovery mnemonic phrase
func confirmMnemonic(mnemonic string) {
	for {
		fmt.Println("Please enter your mnemonic phrase to confirm.")
		confirmation := PromptMnemonic()
		if mnemonic == confirmation {
			return
		}
		fmt.Println("The mnemonic phrase you entered does not match your recovery phrase. Please try again.")
		fmt.Println("")
	}
}
