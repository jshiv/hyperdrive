package wallet

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/goccy/go-json"
	"github.com/mitchellh/go-homedir"
	"github.com/nodeset-org/hyperdrive/hyperdrive-cli/client"
	"github.com/nodeset-org/hyperdrive/hyperdrive-cli/commands/wallet/bip39"
	"github.com/nodeset-org/hyperdrive/hyperdrive-cli/utils"
	"github.com/nodeset-org/hyperdrive/hyperdrive-cli/utils/terminal"
	"github.com/nodeset-org/hyperdrive/shared/config"
	"github.com/nodeset-org/hyperdrive/shared/utils/input"
	nmc_beacon "github.com/rocket-pool/node-manager-core/beacon"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v2"
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
	skipValidatorRecoveryFlag *cli.BoolFlag = &cli.BoolFlag{
		Name:    "skip-validator-key-recovery",
		Aliases: []string{"k"},
		Usage:   "Recover the node wallet, but do not regenerate its validator keys",
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
		for mv.Filled() == false {
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

// Check for custom keys, prompt for their passwords, and store them in the custom keys file
func promptForCustomKeyPasswords(hd *client.HyperdriveClient, cfg *config.HyperdriveConfig, testOnly bool) (string, error) {
	// Check for the custom key directory
	datapath, err := homedir.Expand(cfg.UserDataPath.Value)
	if err != nil {
		return "", fmt.Errorf("error expanding data directory: %w", err)
	}
	customKeyDir := filepath.Join(datapath, "custom-keys")
	info, err := os.Stat(customKeyDir)
	if os.IsNotExist(err) || !info.IsDir() {
		return "", nil
	}

	// Get the custom keystore files
	files, err := os.ReadDir(customKeyDir)
	if err != nil {
		return "", fmt.Errorf("error enumerating custom keystores: %w", err)
	}
	if len(files) == 0 {
		return "", nil
	}

	// Prompt the user with a warning message
	if !testOnly {
		fmt.Printf("%sWARNING:\nHyperdrive has detected that you have custom (externally-derived) validator keys for your minipools.\nIf these keys were actively used for validation by a service such as Allnodes, you MUST CONFIRM WITH THAT SERVICE that they have stopped validating and disabled those keys, and will NEVER validate with them again.\nOtherwise, you may both run the same keys at the same time which WILL RESULT IN YOUR VALIDATORS BEING SLASHED.%s\n\n", terminal.ColorRed, terminal.ColorReset)

		if !utils.Confirm("Please confirm that you have coordinated with the service that was running your minipool validators previously to ensure they have STOPPED validation for your minipools, will NEVER start them again, and you have manually confirmed on a Blockchain explorer such as https://beaconcha.in that your minipools are no longer attesting.") {
			fmt.Println("Cancelled.")
			os.Exit(0)
		}
	}

	// Get the pubkeys for the custom keystores
	customPubkeys := []nmc_beacon.ValidatorPubkey{}
	for _, file := range files {
		// Read the file
		bytes, err := os.ReadFile(filepath.Join(customKeyDir, file.Name()))
		if err != nil {
			return "", fmt.Errorf("error reading custom keystore %s: %w", file.Name(), err)
		}

		// Deserialize it
		keystore := nmc_beacon.ValidatorKeystore{}
		err = json.Unmarshal(bytes, &keystore)
		if err != nil {
			return "", fmt.Errorf("error deserializing custom keystore %s: %w", file.Name(), err)
		}

		customPubkeys = append(customPubkeys, keystore.Pubkey)
	}

	// Notify the user
	fmt.Println("It looks like you have some custom keystores for your minipool's validators.\nYou will be prompted for the passwords each one was encrypted with, so they can be loaded into the Validator Client that Hyperdrive manages for you.\n")

	// Get the passwords for each one
	pubkeyPasswords := map[string]string{}
	for _, pubkey := range customPubkeys {
		password := utils.PromptPassword(
			fmt.Sprintf("Please enter the password that the keystore for %s was encrypted with:", pubkey.HexWithPrefix()), "^.*$", "",
		)

		formattedPubkey := strings.ToUpper(pubkey.HexWithPrefix())
		pubkeyPasswords[formattedPubkey] = password

		fmt.Println()
	}

	// Store them in the file
	fileBytes, err := yaml.Marshal(pubkeyPasswords)
	if err != nil {
		return "", fmt.Errorf("error serializing keystore passwords file: %w", err)
	}
	passwordFile := filepath.Join(datapath, "custom-key-passwords")
	err = os.WriteFile(passwordFile, fileBytes, 0600)
	if err != nil {
		return "", fmt.Errorf("error writing keystore passwords file: %w", err)
	}

	return passwordFile, nil

}

// Deletes the custom key password file
func deleteCustomKeyPasswordFile(passwordFile string) error {
	_, err := os.Stat(passwordFile)
	if os.IsNotExist(err) {
		return nil
	}

	err = os.Remove(passwordFile)
	return err
}
