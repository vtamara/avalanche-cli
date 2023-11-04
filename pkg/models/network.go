// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package models

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	avago_constants "github.com/ava-labs/avalanchego/utils/constants"
)

type Network int64

const (
	Undefined Network = iota
	Mainnet
	Fuji
	Local
	Devnet
)

func (s Network) String() string {
	switch s {
	case Mainnet:
		return "Mainnet"
	case Fuji:
		return "Fuji"
	case Local:
		return "Local Network"
	case Devnet:
		return "Devnet"
	}
	return "Unknown Network"
}

func (s Network) NetworkID() (uint32, error) {
	switch s {
	case Mainnet:
		return avago_constants.MainnetID, nil
	case Fuji:
		return avago_constants.FujiID, nil
	case Local:
		return constants.LocalNetworkID, nil
	case Devnet:
		return constants.DevnetNetworkID, nil
	}
	return 0, fmt.Errorf("unsupported network")
}

func (s Network) Endpoint() (string, error) {
	switch s {
	case Mainnet:
		return constants.MainnetAPIEndpoint, nil
	case Fuji:
		return constants.FujiAPIEndpoint, nil
	case Local:
		return constants.LocalAPIEndpoint, nil
	case Devnet:
		return constants.DevnetAPIEndpoint, nil
	}
	return "", fmt.Errorf("unsupported network")
}

func NetworkFromString(s string) Network {
	switch s {
	case Mainnet.String():
		return Mainnet
	case Fuji.String():
		return Fuji
	case Local.String():
		return Local
	case Devnet.String():
		return Devnet
	}
	return Undefined
}

func NetworkFromNetworkID(networkID uint32) Network {
	switch networkID {
	case avago_constants.MainnetID:
		return Mainnet
	case avago_constants.FujiID:
		return Fuji
	case constants.LocalNetworkID:
		return Local
	case constants.DevnetNetworkID:
		return Devnet
	}
	return Undefined
}
