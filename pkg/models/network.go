// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package models

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	avago_constants "github.com/ava-labs/avalanchego/utils/constants"
)

type NetworkKind int64

const (
	Undefined NetworkKind = iota
	Mainnet
	Fuji
	Local
	Devnet
)

func (s NetworkKind) String() string {
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
	return "invalid network"
}

type Network struct {
	kind NetworkKind
	id uint32
	endpoint string
}

var (
	UndefinedNetwork = NewNetwork(Undefined, 0, "")
	LocalNetwork = NewNetwork(Local, 0, "")
	DevnetNetwork = NewNetwork(Devnet, 0, "")
	FujiNetwork = NewNetwork(Fuji, 0, "")
	MainnetNetwork = NewNetwork(Mainnet, 0, "")
)

func NewNetwork(kind NetworkKind, id uint32, endpoint string) Network {
	return Network{
		kind: kind,
		id: id,
		endpoint: endpoint,
	}
}

func (s Network) Kind() NetworkKind {
	return s.kind
}

func (s Network) NetworkID() (uint32, error) {
	switch s.kind {
	case Mainnet:
		return avago_constants.MainnetID, nil
	case Fuji:
		return avago_constants.FujiID, nil
	case Local:
		return constants.LocalNetworkID, nil
	case Devnet:
		return s.id, nil
	}
	return 0, fmt.Errorf("invalid network")
}

func (s Network) Endpoint() (string, error) {
	switch s.kind {
	case Mainnet:
		return constants.MainnetAPIEndpoint, nil
	case Fuji:
		return constants.FujiAPIEndpoint, nil
	case Local:
		return constants.LocalAPIEndpoint, nil
	case Devnet:
		return s.endpoint, nil
	}
	return "", fmt.Errorf("invalid network")
}

func NetworkFromString(s string) Network {
	switch s {
	case Mainnet.String():
		return NewNetwork(Mainnet, 0, "")
	case Fuji.String():
		return NewNetwork(Fuji, 0, "")
	case Local.String():
		return NewNetwork(Local, 0, "")
	case Devnet.String():
		return NewNetwork(Devnet, 0, "")
	}
	return UndefinedNetwork
}

func NetworkFromNetworkID(networkID uint32) Network {
	switch networkID {
	case avago_constants.MainnetID:
		return NewNetwork(Mainnet, 0, "")
	case avago_constants.FujiID:
		return NewNetwork(Fuji, 0, "")
	case constants.LocalNetworkID:
		return NewNetwork(Local, 0, "")
	case constants.DevnetNetworkID:
		return NewNetwork(Devnet, 0, "")
	}
	return UndefinedNetwork
}
