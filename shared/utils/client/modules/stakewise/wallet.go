package swclient

import (
	"github.com/nodeset-org/hyperdrive/shared/types/api"
	swapi "github.com/nodeset-org/hyperdrive/shared/types/api/modules/stakewise"
	"github.com/nodeset-org/hyperdrive/shared/utils/client"
)

type WalletRequester struct {
	context *client.RequesterContext
}

func NewWalletRequester(context *client.RequesterContext) *WalletRequester {
	return &WalletRequester{
		context: context,
	}
}

func (r *WalletRequester) GetName() string {
	return "Wallet"
}
func (r *WalletRequester) GetRoute() string {
	return "wallet"
}
func (r *WalletRequester) GetContext() *client.RequesterContext {
	return r.context
}

// Export the wallet in encrypted ETH key format
func (r *WalletRequester) Initialize() (*api.ApiResponse[swapi.WalletInitializeData], error) {
	return client.SendGetRequest[swapi.WalletInitializeData](r, "initialize", "Initialize", nil)
}