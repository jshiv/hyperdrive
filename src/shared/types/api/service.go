package api

import (
	"github.com/ethereum/go-ethereum/common"
	nmc_types "github.com/rocket-pool/node-manager-core/api/types"
)

type ServiceTerminateDataFolderData struct {
	FolderExisted bool `json:"folderExisted"`
}

type ServiceCreateFeeRecipientFileData struct {
	Distributor common.Address `json:"distributor"`
}

type ServiceClientStatusData struct {
	EcManagerStatus nmc_types.ClientManagerStatus `json:"ecManagerStatus"`
	BcManagerStatus nmc_types.ClientManagerStatus `json:"bcManagerStatus"`
}

type ServiceGetConfigData struct {
	Config map[string]any `json:"config"`
}

type ServiceVersionData struct {
	Version string `json:"version"`
}
