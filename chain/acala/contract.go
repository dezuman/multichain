package acala

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/centrifuge/go-substrate-rpc-client/types"
	"github.com/renproject/multichain/api/address"
	"github.com/renproject/multichain/api/contract"
	"github.com/renproject/pack"
	"github.com/renproject/surge"
)

type BurnContractCallInput struct {
	Blockhash pack.Bytes32
	Nonce     pack.U32
}

type BurnContractCallOutput struct {
	Amount    pack.U256
	Recipient address.RawAddress
	Confs     pack.U64
}

type BurnEventData struct {
	Recipient types.Bytes
	Amount    types.U128
}

func (client *Client) ContractCall(_ context.Context, _ address.Address, calldata contract.CallData) (pack.Bytes, error) {
	// Deserialise the calldata bytes.
	input := BurnContractCallInput{}
	if err := surge.FromBinary(&input, calldata); err != nil {
		return pack.Bytes{}, fmt.Errorf("deserialise calldata: %v\n", err)
	}

	// Get chain metadata.
	meta, err := client.api.RPC.State.GetMetadataLatest()
	if err != nil {
		return pack.Bytes{}, fmt.Errorf("get metadata: %v", err)
	}

	// Get the block in which the burn event was logged.
	block, err := client.api.RPC.Chain.GetBlock(types.Hash(input.Blockhash))
	if err != nil {
		return pack.Bytes{}, fmt.Errorf("get block: %v", err)
	}

	nonceBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(nonceBytes, uint32(input.Nonce))

	// This key is used to read the state storage at the block of interest.
	key, err := types.CreateStorageKey(meta, "Template", "BurnEvents", nonceBytes, nil)
	if err != nil {
		return pack.Bytes{}, fmt.Errorf("create storage key: %v", err)
	}

	// Retrieve and decode bytes from storage at the block and storage key.
	burnEventData := BurnEventData{}
	ok, err := client.api.RPC.State.GetStorage(key, &burnEventData, types.Hash(input.Blockhash))
	if err != nil || !ok {
		return pack.Bytes{}, fmt.Errorf("get storage: %v", err)
	}

	// Get the latest block header. This will be used to calculate number of block
	// confirmations of the burn log of interest.
	header, err := client.api.RPC.Chain.GetHeaderLatest()
	if err != nil {
		return pack.Bytes{}, fmt.Errorf("get header: %v", err)
	}

	// Calculate block confirmations for the event.
	confs := header.Number - block.Block.Header.Number + 1

	burnLogOutput := BurnContractCallOutput{
		Amount:    pack.NewU256FromInt(burnEventData.Amount.Int),
		Recipient: address.RawAddress(burnEventData.Recipient),
		Confs:     pack.NewU64(uint64(confs)),
	}

	out, err := surge.ToBinary(burnLogOutput)
	if err != nil {
		return pack.Bytes{}, fmt.Errorf("serialise output: %v", err)
	}

	return pack.Bytes(out), nil
}
