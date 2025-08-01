// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package integrity

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"math/big"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/golang/mock/gomock"
	iotexcrypto "github.com/iotexproject/go-pkgs/crypto"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/account"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/action/protocol/execution"
	"github.com/iotexproject/iotex-core/v2/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/v2/action/protocol/poll"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/v2/action/protocol/vote/candidatesutil"
	"github.com/iotexproject/iotex-core/v2/actpool"
	"github.com/iotexproject/iotex-core/v2/blockchain"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/v2/blockchain/filedao"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/blockindex"
	"github.com/iotexproject/iotex-core/v2/config"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/db/trie/mptrie"
	"github.com/iotexproject/iotex-core/v2/pkg/unit"
	. "github.com/iotexproject/iotex-core/v2/pkg/util/assertions"
	"github.com/iotexproject/iotex-core/v2/pkg/util/blockutil"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/state/factory"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_blockcreationsubscriber"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_poll"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

var (
	_deployHash      hash.Hash256                                                                           // in block 1
	_setHash         hash.Hash256                                                                           // in block 2
	_shrHash         hash.Hash256                                                                           // in block 3
	_shlHash         hash.Hash256                                                                           // in block 4
	_sarHash         hash.Hash256                                                                           // in block 5
	_extHash         hash.Hash256                                                                           // in block 6
	_crt2Hash        hash.Hash256                                                                           // in block 7
	_storeHash       hash.Hash256                                                                           // in block 8
	_store2Hash      hash.Hash256                                                                           // in block 9
	_setTopic, _     = hex.DecodeString("fe00000000000000000000000000000000000000000000000000000000001f40") // in block 2
	_getTopic, _     = hex.DecodeString("0000000000000000000000000000000000000000000000000000000000000001") // in block 2
	_shrTopic, _     = hex.DecodeString("00fe00000000000000000000000000000000000000000000000000000000001f") // in block 3
	_shlTopic, _     = hex.DecodeString("fe00000000000000000000000000000000000000000000000000000000001f00") // in block 4
	_sarTopic, _     = hex.DecodeString("fffe00000000000000000000000000000000000000000000000000000000001f") // in block 5
	_extTopic, _     = hex.DecodeString("4a98ce81a2fd5177f0f42b49cb25b01b720f9ce8019f3937f63b789766c938e2") // in block 6
	_crt2Topic, _    = hex.DecodeString("0000000000000000000000001895e6033cd1081f18e0bd23a4501d9376028523") // in block 7
	_preGrPreStore   *big.Int
	_preGrPostStore  *big.Int
	_postGrPostStore *big.Int
)

func fakeGetBlockTime(height uint64) (time.Time, error) {
	return time.Time{}, nil
}

func addTestingConstantinopleBlocks(bc blockchain.Blockchain, dao blockdao.BlockDAO, sf factory.Factory, ap actpool.ActPool) error {
	// Add block 1
	priKey0 := identityset.PrivateKey(27)
	ex1, err := action.SignedExecution(action.EmptyAddress, priKey0, 1, big.NewInt(0), 500000, big.NewInt(testutil.TestGasPriceInt64), _constantinopleOpCodeContract)
	if err != nil {
		return err
	}
	_deployHash, err = ex1.Hash()
	if err != nil {
		return err
	}
	if err := ap.Add(context.Background(), ex1); err != nil {
		return err
	}
	blockTime := time.Unix(1546329600, 0)
	blk, err := bc.MintNewBlock(blockTime)
	if err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	// get deployed contract address
	var contract string
	if dao != nil {
		r, err := receiptByActionHash(dao, 1, _deployHash)
		if err != nil {
			return err
		}
		contract = r.ContractAddress
	}

	addOneBlock := func(contract string, nonce uint64, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte) (hash.Hash256, error) {
		ex1, err := action.SignedExecution(contract, priKey0, nonce, amount, gasLimit, gasPrice, data)
		if err != nil {
			return hash.ZeroHash256, err
		}
		blockTime = blockTime.Add(time.Second)
		if err := ap.Add(context.Background(), ex1); err != nil {
			return hash.ZeroHash256, err
		}
		blk, err = bc.MintNewBlock(blockTime)
		if err != nil {
			return hash.ZeroHash256, err
		}
		if err := bc.CommitBlock(blk); err != nil {
			return hash.ZeroHash256, err
		}
		ex1Hash, err := ex1.Hash()
		if err != nil {
			return hash.ZeroHash256, err
		}
		return ex1Hash, nil
	}

	var (
		zero     = big.NewInt(0)
		gasLimit = testutil.TestGasLimit * 5
		gasPrice = big.NewInt(testutil.TestGasPriceInt64)
	)

	// Add block 2
	// call set() to set storedData = 0xfe...1f40
	funcSig := hash.Hash256b([]byte("set(uint256)"))
	data := append(funcSig[:4], _setTopic...)
	_setHash, err = addOneBlock(contract, 2, zero, gasLimit, gasPrice, data)
	if err != nil {
		return err
	}

	// Add block 3
	// call shright() to test SHR opcode, storedData => 0x00fe...1f
	funcSig = hash.Hash256b([]byte("shright()"))
	_shrHash, err = addOneBlock(contract, 3, zero, gasLimit, gasPrice, funcSig[:4])
	if err != nil {
		return err
	}

	// Add block 4
	// call shleft() to test SHL opcode, storedData => 0xfe...1f00
	funcSig = hash.Hash256b([]byte("shleft()"))
	_shlHash, err = addOneBlock(contract, 4, zero, gasLimit, gasPrice, funcSig[:4])
	if err != nil {
		return err
	}

	// Add block 5
	// call saright() to test SAR opcode, storedData => 0xfffe...1f
	funcSig = hash.Hash256b([]byte("saright()"))
	_sarHash, err = addOneBlock(contract, 5, zero, gasLimit, gasPrice, funcSig[:4])
	if err != nil {
		return err
	}

	// Add block 6
	// call getCodeHash() to test EXTCODEHASH opcode
	funcSig = hash.Hash256b([]byte("getCodeHash(address)"))
	addr, _ := address.FromString(contract)
	ethaddr := hash.BytesToHash256(addr.Bytes())
	data = append(funcSig[:4], ethaddr[:]...)
	_extHash, err = addOneBlock(contract, 6, zero, gasLimit, gasPrice, data)
	if err != nil {
		return err
	}

	// Add block 7
	// call create2() to test CREATE2 opcode
	funcSig = hash.Hash256b([]byte("create2()"))
	_crt2Hash, err = addOneBlock(contract, 7, zero, gasLimit, gasPrice, funcSig[:4])
	if err != nil {
		return err
	}

	// Add block 8
	// test store out of gas
	var (
		caller     = &state.Account{}
		callerAddr = hash.BytesToHash160(identityset.Address(27).Bytes())
	)
	_, err = sf.State(caller, protocol.LegacyKeyOption(callerAddr))
	if err != nil {
		return err
	}
	_preGrPreStore = new(big.Int).Set(caller.Balance)
	_storeHash, err = addOneBlock(action.EmptyAddress, 8, unit.ConvertIotxToRau(10000), 3000000, big.NewInt(unit.Qev), _codeStoreOutOfGasContract)
	if err != nil {
		return err
	}

	if dao != nil {
		r, err := receiptByActionHash(dao, 8, _storeHash)
		if err != nil {
			return err
		}
		if r.Status != uint64(iotextypes.ReceiptStatus_ErrCodeStoreOutOfGas) {
			return blockchain.ErrBalance
		}
	}

	// Add block 9
	// test store out of gas
	_, err = sf.State(caller, protocol.LegacyKeyOption(callerAddr))
	if err != nil {
		return err
	}
	_preGrPostStore = new(big.Int).Set(caller.Balance)
	_store2Hash, err = addOneBlock(action.EmptyAddress, 9, unit.ConvertIotxToRau(10000), 3000000, big.NewInt(unit.Qev), _codeStoreOutOfGasContract)
	if err != nil {
		return err
	}

	if dao != nil {
		r, err := receiptByActionHash(dao, 9, _store2Hash)
		if err != nil {
			return err
		}
		if r.Status != uint64(iotextypes.ReceiptStatus_ErrCodeStoreOutOfGas) {
			return blockchain.ErrBalance
		}
	}

	_, err = sf.State(caller, protocol.LegacyKeyOption(callerAddr))
	if err != nil {
		return err
	}
	_postGrPostStore = new(big.Int).Set(caller.Balance)
	return nil
}

func addTestingTsfBlocks(cfg config.Config, bc blockchain.Blockchain, dao blockdao.BlockDAO, ap actpool.ActPool) error {
	ctx := genesis.WithGenesisContext(context.Background(), cfg.Genesis)
	addOneTsf := func(recipientAddr string, senderPriKey iotexcrypto.PrivateKey, nonce uint64, amount *big.Int, payload []byte, gasLimit uint64, gasPrice *big.Int) error {
		tx, err := action.SignedTransfer(recipientAddr, senderPriKey, nonce, amount, payload, gasLimit, gasPrice)
		if err != nil {
			return err
		}
		if err := ap.Add(ctx, tx); err != nil {
			return err
		}
		return nil
	}
	addOneExec := func(contractAddr string, executorPriKey iotexcrypto.PrivateKey, nonce uint64, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte) error {
		tx, err := action.SignedExecution(contractAddr, executorPriKey, nonce, amount, gasLimit, gasPrice, data)
		if err != nil {
			return err
		}
		if err := ap.Add(ctx, tx); err != nil {
			return err
		}
		return nil
	}
	// Add block 1
	addr0 := identityset.Address(27).String()
	if err := addOneTsf(addr0, identityset.PrivateKey(0), 1, big.NewInt(90000000), nil, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64)); err != nil {
		return err
	}
	blk, err := bc.MintNewBlock(testutil.TimestampNow())
	if err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}
	ap.Reset()

	priKey0 := identityset.PrivateKey(27)
	addr1 := identityset.Address(28).String()
	priKey1 := identityset.PrivateKey(28)
	addr2 := identityset.Address(29).String()
	priKey2 := identityset.PrivateKey(29)
	addr3 := identityset.Address(30).String()
	priKey3 := identityset.PrivateKey(30)
	addr4 := identityset.Address(31).String()
	priKey4 := identityset.PrivateKey(31)
	addr5 := identityset.Address(32).String()
	priKey5 := identityset.PrivateKey(32)
	addr6 := identityset.Address(33).String()
	// Add block 2
	// test --> A, B, C, D, E, F
	if err := addOneTsf(addr1, priKey0, 1, big.NewInt(20), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64)); err != nil {
		return err
	}
	if err := addOneTsf(addr2, priKey0, 2, big.NewInt(30), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64)); err != nil {
		return err
	}
	if err := addOneTsf(addr3, priKey0, 3, big.NewInt(50), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64)); err != nil {
		return err
	}
	if err := addOneTsf(addr4, priKey0, 4, big.NewInt(70), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64)); err != nil {
		return err
	}
	if err := addOneTsf(addr5, priKey0, 5, big.NewInt(110), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64)); err != nil {
		return err
	}
	if err := addOneTsf(addr6, priKey0, 6, big.NewInt(50<<20), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64)); err != nil {
		return err
	}
	// deploy simple smart contract
	data, _ := hex.DecodeString("608060405234801561001057600080fd5b50610233806100206000396000f300608060405260043610610057576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff1680635bec9e671461005c57806360fe47b114610073578063c2bc2efc146100a0575b600080fd5b34801561006857600080fd5b506100716100f7565b005b34801561007f57600080fd5b5061009e60048036038101908080359060200190929190505050610143565b005b3480156100ac57600080fd5b506100e1600480360381019080803573ffffffffffffffffffffffffffffffffffffffff16906020019092919050505061017a565b6040518082815260200191505060405180910390f35b5b6001156101155760008081548092919060010191905055506100f8565b7f8bfaa460932ccf8751604dd60efa3eafa220ec358fccb32ef703f91c509bc3ea60405160405180910390a1565b80600081905550807fdf7a95aebff315db1b7716215d602ab537373cdb769232aae6055c06e798425b60405160405180910390a250565b60008073ffffffffffffffffffffffffffffffffffffffff168273ffffffffffffffffffffffffffffffffffffffff16141515156101b757600080fd5b6000548273ffffffffffffffffffffffffffffffffffffffff167fbde7a70c2261170a87678200113c8e12f82f63d0a1d1cfa45681cbac328e87e360405160405180910390a360005490509190505600a165627a7a723058203198d0390613dab2dff2fa053c1865e802618d628429b01ab05b8458afc347eb0029")
	ex1, err := action.SignedExecution(action.EmptyAddress, priKey2, 1, big.NewInt(0), 200000, big.NewInt(testutil.TestGasPriceInt64), data)
	if err != nil {
		return err
	}
	if err := ap.Add(ctx, ex1); err != nil {
		return err
	}
	_deployHash, err = ex1.Hash()
	if err != nil {
		return err
	}
	blk, err = bc.MintNewBlock(testutil.TimestampNow())
	if err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}
	ap.Reset()

	// get deployed contract address
	var contract string
	_, gateway := cfg.Plugins[config.GatewayPlugin]
	if gateway && !cfg.Chain.EnableAsyncIndexWrite {
		r, err := receiptByActionHash(dao, 2, _deployHash)
		if err != nil {
			return err
		}
		contract = r.ContractAddress
	}

	// Add block 3
	// Charlie --> A, B, D, E, test
	if err := addOneTsf(addr1, priKey3, 1, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64)); err != nil {
		return err
	}
	if err := addOneTsf(addr2, priKey3, 2, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64)); err != nil {
		return err
	}
	if err := addOneTsf(addr4, priKey3, 3, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64)); err != nil {
		return err
	}
	if err := addOneTsf(addr5, priKey3, 4, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64)); err != nil {
		return err
	}
	if err := addOneTsf(addr0, priKey3, 5, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64)); err != nil {
		return err
	}
	// call set() to set storedData = 0x1f40
	data, _ = hex.DecodeString("60fe47b1")
	data = append(data, _setTopic...)
	ex1, err = action.SignedExecution(contract, priKey2, 2, big.NewInt(0), testutil.TestGasLimit*5, big.NewInt(testutil.TestGasPriceInt64), data)
	if err != nil {
		return err
	}
	_setHash, err = ex1.Hash()
	if err != nil {
		return err
	}
	if err := ap.Add(context.Background(), ex1); err != nil {
		return err
	}
	blk, err = bc.MintNewBlock(testutil.TimestampNow())
	if err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}
	ap.Reset()

	// Add block 4
	// Delta --> B, E, F, test
	if err := addOneTsf(addr2, priKey4, 1, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64)); err != nil {
		return err
	}
	if err := addOneTsf(addr5, priKey4, 2, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64)); err != nil {
		return err
	}
	if err := addOneTsf(addr6, priKey4, 3, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64)); err != nil {
		return err
	}
	if err := addOneTsf(addr0, priKey4, 4, big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64)); err != nil {
		return err
	}
	data, _ = hex.DecodeString("c2bc2efc")
	data = append(data, _getTopic...)
	ex1, err = action.SignedExecution(contract, priKey2, 3, big.NewInt(0), testutil.TestGasLimit*5, big.NewInt(testutil.TestGasPriceInt64), data)
	if err != nil {
		return err
	}
	_sarHash, err = ex1.Hash()
	if err != nil {
		return err
	}
	if err := ap.Add(context.Background(), ex1); err != nil {
		return err
	}
	blk, err = bc.MintNewBlock(testutil.TimestampNow())
	if err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	// Add block 5
	// Delta --> A, B, C, D, F, test
	if err := addOneTsf(addr1, priKey5, 1, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64)); err != nil {
		return err
	}
	if err := addOneTsf(addr2, priKey5, 2, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64)); err != nil {
		return err
	}
	if err := addOneTsf(addr3, priKey5, 3, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64)); err != nil {
		return err
	}
	if err := addOneTsf(addr4, priKey5, 4, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64)); err != nil {
		return err
	}
	if err := addOneTsf(addr6, priKey5, 5, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64)); err != nil {
		return err
	}
	if err := addOneTsf(addr0, priKey5, 6, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64)); err != nil {
		return err
	}
	if err := addOneTsf(addr3, priKey3, 6, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64)); err != nil {
		return err
	}
	if err := addOneTsf(addr1, priKey1, 1, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64)); err != nil {
		return err
	}
	// call set() to set storedData = 0x1f40
	data, _ = hex.DecodeString("60fe47b1")
	data = append(data, _setTopic...)
	if err := addOneExec(contract, priKey2, 4, big.NewInt(0), testutil.TestGasLimit*5, big.NewInt(testutil.TestGasPriceInt64), data); err != nil {
		return err
	}
	data, _ = hex.DecodeString("c2bc2efc")
	data = append(data, _getTopic...)
	if err := addOneExec(contract, priKey2, 5, big.NewInt(0), testutil.TestGasLimit*5, big.NewInt(testutil.TestGasPriceInt64), data); err != nil {
		return err
	}
	blk, err = bc.MintNewBlock(testutil.TimestampNow())
	if err != nil {
		return err
	}
	return bc.CommitBlock(blk)
}

func TestCreateBlockchain(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	cfg := config.Default
	// disable account-based testing
	cfg.Chain.TrieDBPath = ""
	cfg.Genesis = genesis.TestDefault()
	cfg.ActPool.MinGasPriceStr = "0"
	// create chain
	registry := protocol.NewRegistry()
	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(acc.Register(registry))
	rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
	require.NoError(rp.Register(registry))
	factoryCfg := factory.GenerateConfig(cfg.Chain, cfg.Genesis)
	sf, err := factory.NewStateDB(factoryCfg, db.NewMemKVStore(), factory.RegistryStateDBOption(registry))
	require.NoError(err)
	ap, err := actpool.NewActPool(cfg.Genesis, sf, cfg.ActPool)
	require.NoError(err)
	store, err := filedao.NewFileDAOInMemForTest()
	require.NoError(err)
	dao := blockdao.NewBlockDAOWithIndexersAndCache(store, []blockdao.BlockIndexer{sf}, cfg.DB.MaxCacheSize)
	bc := blockchain.NewBlockchain(
		cfg.Chain,
		cfg.Genesis,
		dao,
		factory.NewMinter(sf, ap),
		blockchain.BlockValidatorOption(block.NewValidator(
			sf,
			protocol.NewGenericValidator(sf, accountutil.AccountState),
		)),
	)
	ep := execution.NewProtocol(dao.GetBlockHash, rewarding.DepositGas, fakeGetBlockTime)
	require.NoError(ep.Register(registry))
	rewardingProtocol := rewarding.NewProtocol(cfg.Genesis.Rewarding)
	require.NoError(rewardingProtocol.Register(registry))
	require.NoError(bc.Start(ctx))
	require.NotNil(bc)
	height := bc.TipHeight()
	require.Equal(0, int(height))
	defer func() {
		require.NoError(bc.Stop(ctx))
	}()

	// add 4 sample blocks
	require.NoError(addTestingTsfBlocks(cfg, bc, nil, ap))
	height = bc.TipHeight()
	require.Equal(5, int(height))
}

func TestGetBlockHash(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	cfg := config.Default
	// disable account-based testing
	cfg.Chain.TrieDBPath = ""
	cfg.Genesis = genesis.TestDefault()
	cfg.Genesis.AleutianBlockHeight = 2
	cfg.Genesis.HawaiiBlockHeight = 4
	cfg.Genesis.MidwayBlockHeight = 9
	cfg.ActPool.MinGasPriceStr = "0"
	genesis.SetGenesisTimestamp(cfg.Genesis.Timestamp)
	block.LoadGenesisHash(&cfg.Genesis)
	// create chain
	registry := protocol.NewRegistry()
	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(acc.Register(registry))
	rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
	require.NoError(rp.Register(registry))
	factoryCfg := factory.GenerateConfig(cfg.Chain, cfg.Genesis)
	sf, err := factory.NewStateDB(factoryCfg, db.NewMemKVStore(), factory.RegistryStateDBOption(registry))
	require.NoError(err)
	ap, err := actpool.NewActPool(cfg.Genesis, sf, cfg.ActPool)
	require.NoError(err)
	store, err := filedao.NewFileDAOInMemForTest()
	require.NoError(err)
	dao := blockdao.NewBlockDAOWithIndexersAndCache(store, []blockdao.BlockIndexer{sf}, cfg.DB.MaxCacheSize)
	bc := blockchain.NewBlockchain(
		cfg.Chain,
		cfg.Genesis,
		dao,
		factory.NewMinter(sf, ap),
		blockchain.BlockValidatorOption(block.NewValidator(
			sf,
			protocol.NewGenericValidator(sf, accountutil.AccountState),
		)),
	)
	ep := execution.NewProtocol(dao.GetBlockHash, rewarding.DepositGas, fakeGetBlockTime)
	require.NoError(ep.Register(registry))
	rewardingProtocol := rewarding.NewProtocol(cfg.Genesis.Rewarding)
	require.NoError(rewardingProtocol.Register(registry))
	require.NoError(bc.Start(ctx))
	require.NotNil(bc)
	height := bc.TipHeight()
	require.Equal(0, int(height))
	defer func() {
		require.NoError(bc.Stop(ctx))
	}()

	addTestingGetBlockHash(t, cfg.Genesis, bc, dao, ap)
}

func addTestingGetBlockHash(t *testing.T, g genesis.Genesis, bc blockchain.Blockchain, dao blockdao.BlockDAO, ap actpool.ActPool) {
	require := require.New(t)
	priKey0 := identityset.PrivateKey(27)

	// deploy simple smart contract
	/*
		pragma solidity <6.0 >=0.4.24;

		contract Test {
		    event GetBlockhash(bytes32 indexed hash);

		    function getBlockHash(uint256 blockNumber) public  returns (bytes32) {
		       bytes32 h = blockhash(blockNumber);
		        emit GetBlockhash(h);
		        return h;
		    }
		}
	*/
	ctx := genesis.WithGenesisContext(context.Background(), g)
	data, _ := hex.DecodeString("6080604052348015600f57600080fd5b5060de8061001e6000396000f3fe6080604052348015600f57600080fd5b506004361060285760003560e01c8063ee82ac5e14602d575b600080fd5b605660048036036020811015604157600080fd5b8101908080359060200190929190505050606c565b6040518082815260200191505060405180910390f35b60008082409050807f2d93f7749862d33969fb261757410b48065a1bc86a56da5c47820bd063e2338260405160405180910390a28091505091905056fea265627a7a723158200a258cd08ea99ee11aa68c78b6d2bf7ea912615a1e64a81b90a2abca2dd59cfa64736f6c634300050c0032")

	ex1, err := action.SignedExecution(action.EmptyAddress, priKey0, 1, big.NewInt(0), 500000, big.NewInt(testutil.TestGasPriceInt64), data)
	require.NoError(err)
	require.NoError(ap.Add(ctx, ex1))
	_deployHash, err = ex1.Hash()
	require.NoError(err)
	blk, err := bc.MintNewBlock(testutil.TimestampNow())
	require.NoError(err)
	require.NoError(bc.CommitBlock(blk))

	ap.Reset()
	blockTime := time.Unix(1546329600, 0)
	// get deployed contract address
	var contract string
	if dao != nil {
		r, err := receiptByActionHash(dao, 1, _deployHash)
		require.NoError(err)
		contract = r.ContractAddress
	}
	addOneBlock := func(contract string, nonce uint64, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte) (hash.Hash256, error) {
		ex1, err := action.SignedExecution(contract, priKey0, nonce, amount, gasLimit, gasPrice, data)
		if err != nil {
			return hash.ZeroHash256, err
		}
		blockTime = blockTime.Add(time.Second)
		if err := ap.Add(ctx, ex1); err != nil {
			return hash.ZeroHash256, err
		}
		blk, err = bc.MintNewBlock(blockTime)
		if err != nil {
			return hash.ZeroHash256, err
		}
		if err := bc.CommitBlock(blk); err != nil {
			return hash.ZeroHash256, err
		}
		ex1Hash, err := ex1.Hash()
		if err != nil {
			return hash.ZeroHash256, err
		}
		return ex1Hash, nil
	}

	getBlockHashCallData := func(x int64) []byte {
		funcSig := hash.Hash256b([]byte("getBlockHash(uint256)"))
		// convert block number to uint256 (32-bytes)
		blockNumber := hash.BytesToHash256(big.NewInt(x).Bytes())
		return append(funcSig[:4], blockNumber[:]...)
	}

	var (
		zero     = big.NewInt(0)
		nonce    = uint64(2)
		gasLimit = testutil.TestGasLimit * 5
		gasPrice = big.NewInt(testutil.TestGasPriceInt64)
		bcHash   hash.Hash256
	)
	tests := []struct {
		commitHeight  uint64
		getHashHeight uint64
	}{
		{2, 0},
		{3, 5},
		{4, 1},
		{5, 3},
		{6, 0},
		{7, 6},
		{8, 9},
		{9, 3},
		{10, 9},
		{11, 1},
		{12, 4},
		{13, 0},
		{14, 100},
		{15, 15},
	}
	for _, test := range tests {
		h, err := addOneBlock(contract, nonce, zero, gasLimit, gasPrice, getBlockHashCallData(int64(test.getHashHeight)))
		require.NoError(err)
		r, err := receiptByActionHash(dao, test.commitHeight, h)
		require.NoError(err)
		if test.getHashHeight >= test.commitHeight {
			bcHash = hash.ZeroHash256
		} else if test.commitHeight < g.HawaiiBlockHeight {
			// before hawaii it mistakenly return zero hash
			// see https://github.com/iotexproject/iotex-core/v2/commit/2585b444214f9009b6356fbaf59c992e8728fc01
			bcHash = hash.ZeroHash256
		} else {
			var targetHeight uint64
			if test.commitHeight < g.MidwayBlockHeight {
				targetHeight = test.commitHeight - (test.getHashHeight + 1)
			} else {
				targetHeight = test.getHashHeight
			}
			bcHash, err = dao.GetBlockHash(targetHeight)
			require.NoError(err)
		}
		require.Equal(r.Logs()[0].Topics[1], bcHash)
		nonce++
	}
}

func TestBlockchain_MintNewBlock(t *testing.T) {
	cfg := config.Default
	cfg.Genesis = genesis.TestDefault()
	cfg.Genesis.BlockGasLimit = uint64(100000)
	cfg.ActPool.MinGasPriceStr = "0"
	ctx := genesis.WithGenesisContext(context.Background(), cfg.Genesis)
	registry := protocol.NewRegistry()
	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(t, acc.Register(registry))
	rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
	require.NoError(t, rp.Register(registry))
	factoryCfg := factory.GenerateConfig(cfg.Chain, cfg.Genesis)
	sf, err := factory.NewStateDB(factoryCfg, db.NewMemKVStore(), factory.RegistryStateDBOption(registry))
	require.NoError(t, err)
	ap, err := actpool.NewActPool(cfg.Genesis, sf, cfg.ActPool)
	require.NoError(t, err)
	store, err := filedao.NewFileDAOInMemForTest()
	require.NoError(t, err)
	dao := blockdao.NewBlockDAOWithIndexersAndCache(store, []blockdao.BlockIndexer{sf}, cfg.DB.MaxCacheSize)
	bc := blockchain.NewBlockchain(
		cfg.Chain,
		cfg.Genesis,
		dao,
		factory.NewMinter(sf, ap),
		blockchain.BlockValidatorOption(block.NewValidator(
			sf,
			protocol.NewGenericValidator(sf, accountutil.AccountState),
		)),
	)
	ep := execution.NewProtocol(dao.GetBlockHash, rewarding.DepositGas, fakeGetBlockTime)
	require.NoError(t, ep.Register(registry))
	rewardingProtocol := rewarding.NewProtocol(cfg.Genesis.Rewarding)
	require.NoError(t, rewardingProtocol.Register(registry))
	require.NoError(t, bc.Start(ctx))
	defer func() {
		require.NoError(t, bc.Stop(ctx))
	}()

	tsf := action.NewTransfer(
		big.NewInt(100000000),
		identityset.Address(27).String(),
		[]byte{})
	data, _ := hex.DecodeString("608060405234801561001057600080fd5b5060df8061001f6000396000f3006080604052600436106049576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff16806360fe47b114604e5780636d4ce63c146078575b600080fd5b348015605957600080fd5b5060766004803603810190808035906020019092919050505060a0565b005b348015608357600080fd5b50608a60aa565b6040518082815260200191505060405180910390f35b8060008190555050565b600080549050905600a165627a7a7230582002faabbefbbda99b20217cf33cb8ab8100caf1542bf1f48117d72e2c59139aea0029")
	execution := action.NewExecution(action.EmptyAddress, big.NewInt(0), data)

	bd := &action.EnvelopeBuilder{}
	elp1 := bd.SetAction(tsf).
		SetNonce(1).
		SetGasLimit(100000).
		SetGasPrice(big.NewInt(10)).Build()
	selp1, err := action.Sign(elp1, identityset.PrivateKey(0))
	require.NoError(t, err)
	require.NoError(t, ap.Add(ctx, selp1))
	// This execution should not be included in block because block is out of gas
	elp2 := bd.SetAction(execution).
		SetNonce(2).
		SetGasLimit(100000).
		SetGasPrice(big.NewInt(10)).Build()
	selp2, err := action.Sign(elp2, identityset.PrivateKey(0))
	require.NoError(t, err)
	require.NoError(t, ap.Add(ctx, selp2))

	blk, err := bc.MintNewBlock(testutil.TimestampNow())
	require.NoError(t, err)
	require.Equal(t, 2, len(blk.Actions))
	require.Equal(t, 2, len(blk.Receipts))
	var gasConsumed uint64
	for _, receipt := range blk.Receipts {
		gasConsumed += receipt.GasConsumed
	}
	require.True(t, gasConsumed <= cfg.Genesis.BlockGasLimit)
}

func TestBlockchain_MintNewBlock_PopAccount(t *testing.T) {
	cfg := config.Default
	cfg.Genesis = genesis.TestDefault()
	cfg.ActPool.MinGasPriceStr = "0"
	ctx := genesis.WithGenesisContext(context.Background(), cfg.Genesis)
	registry := protocol.NewRegistry()
	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(t, acc.Register(registry))
	factoryCfg := factory.GenerateConfig(cfg.Chain, cfg.Genesis)
	sf, err := factory.NewStateDB(factoryCfg, db.NewMemKVStore(), factory.RegistryStateDBOption(registry))
	require.NoError(t, err)
	ap, err := actpool.NewActPool(cfg.Genesis, sf, cfg.ActPool)
	require.NoError(t, err)
	store, err := filedao.NewFileDAOInMemForTest()
	require.NoError(t, err)
	dao := blockdao.NewBlockDAOWithIndexersAndCache(store, []blockdao.BlockIndexer{sf}, cfg.DB.MaxCacheSize)
	bc := blockchain.NewBlockchain(
		cfg.Chain,
		cfg.Genesis,
		dao,
		factory.NewMinter(sf, ap),
		blockchain.BlockValidatorOption(block.NewValidator(
			sf,
			protocol.NewGenericValidator(sf, accountutil.AccountState),
		)),
	)
	rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
	require.NoError(t, rp.Register(registry))
	ep := execution.NewProtocol(dao.GetBlockHash, rewarding.DepositGas, fakeGetBlockTime)
	require.NoError(t, ep.Register(registry))
	rewardingProtocol := rewarding.NewProtocol(cfg.Genesis.Rewarding)
	require.NoError(t, rewardingProtocol.Register(registry))
	require.NoError(t, bc.Start(ctx))
	defer func() {
		require.NoError(t, bc.Stop(ctx))
	}()

	priKey0 := identityset.PrivateKey(27)
	addr1 := identityset.Address(28).String()
	priKey3 := identityset.PrivateKey(30)
	require.NoError(t, addTestingTsfBlocks(cfg, bc, nil, ap))

	// test third block
	bytes := []byte{}
	for i := 0; i < 1000; i++ {
		bytes = append(bytes, 1)
	}
	for i := uint64(0); i < 300; i++ {
		tsf, err := action.SignedTransfer(addr1, priKey0, i+7, big.NewInt(2), bytes,
			19000, big.NewInt(testutil.TestGasPriceInt64))
		require.NoError(t, err)
		require.NoError(t, ap.Add(ctx, tsf))
	}
	transfer1, err := action.SignedTransfer(addr1, priKey3, 7, big.NewInt(2),
		[]byte{}, 10000, big.NewInt(testutil.TestGasPriceInt64))
	require.NoError(t, err)
	require.NoError(t, ap.Add(ctx, transfer1))

	blk, err := bc.MintNewBlock(testutil.TimestampNow())
	require.NoError(t, err)
	require.NotNil(t, blk)
	require.Equal(t, 183, len(blk.Actions))
	whetherInclude := false
	for _, action := range blk.Actions {
		transfer1Hash, err := transfer1.Hash()
		require.NoError(t, err)
		actionHash, err := action.Hash()
		require.NoError(t, err)
		if transfer1Hash == actionHash {
			whetherInclude = true
			break
		}
	}
	require.True(t, whetherInclude)
}

type MockSubscriber struct {
	counter int32
}

func (ms *MockSubscriber) ReceiveBlock(blk *block.Block) error {
	tsfs, _ := classifyActions(blk.Actions)
	atomic.AddInt32(&ms.counter, int32(len(tsfs)))
	return nil
}

func (ms *MockSubscriber) Counter() int {
	return int(atomic.LoadInt32(&ms.counter))
}

func createChain(cfg config.Config, inMem bool) (blockchain.Blockchain, factory.Factory, blockdao.BlockDAO, actpool.ActPool, error) {
	registry := protocol.NewRegistry()
	// Create a blockchain from scratch
	factoryCfg := factory.GenerateConfig(cfg.Chain, cfg.Genesis)
	var (
		sf  factory.Factory
		dao blockdao.BlockDAO
		err error
	)
	if inMem {
		sf, err = factory.NewStateDB(factoryCfg, db.NewMemKVStore(), factory.RegistryStateDBOption(registry))
	} else {
		var db2 db.KVStore
		db2, err = db.CreateKVStore(cfg.DB, cfg.Chain.TrieDBPath)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		sf, err = factory.NewStateDB(factoryCfg, db2, factory.RegistryStateDBOption(registry))
	}
	if err != nil {
		return nil, nil, nil, nil, err
	}
	ap, err := actpool.NewActPool(cfg.Genesis, sf, cfg.ActPool)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	ap.AddActionEnvelopeValidators(
		protocol.NewGenericValidator(sf, accountutil.AccountState),
	)
	acc := account.NewProtocol(rewarding.DepositGas)
	if err = acc.Register(registry); err != nil {
		return nil, nil, nil, nil, err
	}
	rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
	if err = rp.Register(registry); err != nil {
		return nil, nil, nil, nil, err
	}
	// create indexer
	cfg.DB.DbPath = cfg.Chain.IndexDBPath
	indexer, err := blockindex.NewIndexer(db.NewBoltDB(cfg.DB), cfg.Genesis.Hash())
	if err != nil {
		return nil, nil, nil, nil, err
	}
	var store blockdao.BlockStore
	// create BlockDAO
	if inMem {
		store, err = filedao.NewFileDAOInMemForTest()
		if err != nil {
			return nil, nil, nil, nil, err
		}
	} else {
		cfg.DB.DbPath = cfg.Chain.ChainDBPath
		store, err = filedao.NewFileDAO(cfg.DB, block.NewDeserializer(cfg.Chain.EVMNetworkID))
	}
	if err != nil {
		return nil, nil, nil, nil, err
	}
	dao = blockdao.NewBlockDAOWithIndexersAndCache(
		store,
		[]blockdao.BlockIndexer{sf, indexer},
		cfg.DB.MaxCacheSize,
	)
	if dao == nil {
		return nil, nil, nil, nil, err
	}
	rewardingProtocol := rewarding.NewProtocol(cfg.Genesis.Rewarding)
	if err = rewardingProtocol.Register(registry); err != nil {
		return nil, nil, nil, nil, err
	}
	bc := blockchain.NewBlockchain(
		cfg.Chain,
		cfg.Genesis,
		dao,
		factory.NewMinter(sf, ap),
		blockchain.BlockValidatorOption(block.NewValidator(
			sf,
			protocol.NewGenericValidator(sf, accountutil.AccountState),
		)),
	)
	btc, err := blockutil.NewBlockTimeCalculator(func(uint64) time.Duration { return time.Second },
		bc.TipHeight, func(height uint64) (time.Time, error) {
			blk, err := dao.GetBlockByHeight(height)
			if err != nil {
				return time.Time{}, err
			}
			return blk.Timestamp(), nil
		})
	if err != nil {
		return nil, nil, nil, nil, err
	}
	ep := execution.NewProtocol(dao.GetBlockHash, rewarding.DepositGas, btc.CalculateBlockTime)
	if err = ep.Register(registry); err != nil {
		return nil, nil, nil, nil, err
	}
	return bc, sf, dao, ap, nil
}

func TestBlockchainHardForkFeatures(t *testing.T) {
	require := require.New(t)

	cfg := config.Default
	testIndexPath, err := testutil.PathOfTempFile("index")
	require.NoError(err)

	defer func() {
		testutil.CleanupPath(testIndexPath)
		// clear the gateway
		delete(cfg.Plugins, config.GatewayPlugin)
	}()

	minGas := big.NewInt(unit.Qev)
	cfg.Chain.IndexDBPath = testIndexPath
	cfg.Chain.ProducerPrivKey = "a000000000000000000000000000000000000000000000000000000000000000"
	cfg.Genesis = genesis.TestDefault()
	cfg.Plugins[config.GatewayPlugin] = true
	cfg.Chain.EnableAsyncIndexWrite = false
	cfg.ActPool.MinGasPriceStr = minGas.String()
	cfg.Genesis.PacificBlockHeight = 2
	cfg.Genesis.AleutianBlockHeight = 2
	cfg.Genesis.BeringBlockHeight = 2
	cfg.Genesis.CookBlockHeight = 2
	cfg.Genesis.DaytonaBlockHeight = 2
	cfg.Genesis.DardanellesBlockHeight = 2
	cfg.Genesis.EasterBlockHeight = 2
	cfg.Genesis.FbkMigrationBlockHeight = 2
	cfg.Genesis.FairbankBlockHeight = 2
	cfg.Genesis.GreenlandBlockHeight = 2
	cfg.Genesis.HawaiiBlockHeight = 2
	cfg.Genesis.IcelandBlockHeight = 2
	cfg.Genesis.JutlandBlockHeight = 2
	cfg.Genesis.KamchatkaBlockHeight = 2
	cfg.Genesis.LordHoweBlockHeight = 2
	cfg.Genesis.MidwayBlockHeight = 2
	cfg.Genesis.NewfoundlandBlockHeight = 2
	cfg.Genesis.OkhotskBlockHeight = 2
	cfg.Genesis.PalauBlockHeight = 2
	cfg.Genesis.QuebecBlockHeight = 2
	cfg.Genesis.RedseaBlockHeight = 2
	cfg.Genesis.SumatraBlockHeight = 2
	cfg.Genesis.TsunamiBlockHeight = 3
	cfg.Genesis.UpernavikBlockHeight = 4
	cfg.Genesis.VanuatuBlockHeight = 4
	cfg.Genesis.InitBalanceMap[identityset.Address(27).String()] = unit.ConvertIotxToRau(10000000000).String()

	ctx := genesis.WithGenesisContext(context.Background(), cfg.Genesis)
	bc, sf, dao, ap, err := createChain(cfg, true)
	require.NoError(err)
	sk, err := iotexcrypto.HexStringToPrivateKey(cfg.Chain.ProducerPrivKey)
	require.NoError(err)
	producer := sk.PublicKey().Address()
	ctrl := gomock.NewController(t)
	pp := mock_poll.NewMockProtocol(ctrl)
	pp.EXPECT().CreateGenesisStates(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	pp.EXPECT().Candidates(gomock.Any(), gomock.Any()).Return([]*state.Candidate{
		{
			Address:       producer.String(),
			RewardAddress: producer.String(),
		},
	}, nil).AnyTimes()
	pp.EXPECT().Register(gomock.Any()).DoAndReturn(func(reg *protocol.Registry) error {
		return reg.Register("poll", pp)
	}).AnyTimes()
	pp.EXPECT().Validate(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	require.NoError(sf.Register(pp))
	require.NoError(bc.Start(ctx))
	defer func() {
		require.NoError(bc.Stop(ctx))
	}()

	// Add block 1
	nonce, err := ap.GetPendingNonce(identityset.Address(27).String())
	require.NoError(err)
	require.EqualValues(1, nonce)
	nonce, err = ap.GetPendingNonce(identityset.Address(25).String())
	require.NoError(err)
	require.EqualValues(1, nonce)
	priKey0 := identityset.PrivateKey(27)
	ex1, err := action.SignedExecution(action.EmptyAddress, priKey0, 1, new(big.Int), 500000, minGas, _constantinopleOpCodeContract)
	require.NoError(err)
	require.NoError(ap.Add(ctx, ex1))
	tsf1, err := action.SignedTransfer(identityset.Address(25).String(), priKey0, 2, big.NewInt(10000), nil, 500000, minGas)
	require.NoError(err)
	require.NoError(ap.Add(ctx, tsf1))
	tsf2, err := action.SignedTransfer(identityset.Address(24).String(), priKey0, 3, big.NewInt(10000), nil, 500000, minGas)
	require.NoError(err)
	require.NoError(ap.Add(ctx, tsf2))
	tsf3, err := action.SignedTransfer(identityset.Address(23).String(), priKey0, 4, big.NewInt(30000), nil, 500000, minGas)
	require.NoError(err)
	require.NoError(ap.Add(ctx, tsf3))
	deterministic, err := address.FromHex("3fab184622dc19b6109349b94811493bf2a45362")
	require.NoError(err)
	tsf4, err := action.SignedTransfer(deterministic.String(), priKey0, 5, big.NewInt(10000000000000000), nil, 500000, minGas)
	require.NoError(err)
	require.NoError(ap.Add(ctx, tsf4))
	blockTime := time.Unix(1546329600, 0)
	blk, err := bc.MintNewBlock(blockTime)
	require.NoError(err)
	require.EqualValues(1, blk.Height())
	require.Equal(6, len(blk.Body.Actions))
	require.NoError(bc.CommitBlock(blk))

	// get deployed contract address
	h, err := ex1.Hash()
	require.NoError(err)
	r, err := receiptByActionHash(dao, 1, h)
	require.NoError(err)
	testContract := r.ContractAddress

	// verify 4 recipients remain legacy fresh accounts
	for _, v := range []struct {
		a address.Address
		b string
	}{
		{identityset.Address(23), "100000000000000000000030000"},
		{identityset.Address(24), "100000000000000000000010000"},
		{identityset.Address(25), "100000000000000000000010000"},
		{deterministic, "10000000000000000"},
	} {
		a, err := accountutil.AccountState(ctx, sf, v.a)
		require.NoError(err)
		require.True(a.IsLegacyFreshAccount())
		require.EqualValues(1, a.PendingNonce())
		require.Equal(v.b, a.Balance.String())
		// actpool returns nonce considering legacy fresh account
		nonce, err = ap.GetPendingNonce(v.a.String())
		require.NoError(err)
		require.Zero(nonce)
	}

	// Add block 2 -- test the UseZeroNonceForFreshAccount flag
	t1 := action.NewTransfer(big.NewInt(100), identityset.Address(27).String(), nil)
	elp := (&action.EnvelopeBuilder{}).SetNonce(0).
		SetChainID(cfg.Chain.ID).
		SetGasPrice(minGas).
		SetGasLimit(500000).
		SetAction(t1).Build()
	tsf1, err = action.Sign(elp, identityset.PrivateKey(25))
	require.NoError(err)
	require.NoError(ap.Add(ctx, tsf1))
	t2 := action.NewTransfer(big.NewInt(200), identityset.Address(27).String(), nil)
	elp = (&action.EnvelopeBuilder{}).SetNonce(1).
		SetChainID(cfg.Chain.ID).
		SetGasPrice(minGas).
		SetGasLimit(500000).
		SetAction(t2).Build()
	tsf2, err = action.Sign(elp, identityset.PrivateKey(25))
	require.NoError(err)
	require.NoError(ap.Add(ctx, tsf2))
	// call set() to set storedData = 0xfe...1f40
	funcSig := hash.Hash256b([]byte("set(uint256)"))
	e1 := action.NewExecution(testContract, new(big.Int), append(funcSig[:4], _setTopic...))
	elp = (&action.EnvelopeBuilder{}).SetNonce(0).
		SetChainID(cfg.Chain.ID).
		SetGasPrice(minGas).
		SetGasLimit(500000).
		SetAction(e1).Build()
	ex1, err = action.Sign(elp, identityset.PrivateKey(24))
	require.NoError(err)
	require.NoError(ap.Add(ctx, ex1))
	// deterministic deployment transaction
	tx, err := action.DecodeEtherTx("0xf8a58085174876e800830186a08080b853604580600e600039806000f350fe7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe03601600081602082378035828234f58015156039578182fd5b8082525050506014600cf31ba02222222222222222222222222222222222222222222222222222222222222222a02222222222222222222222222222222222222222222222222222222222222222")
	require.NoError(err)
	require.False(tx.Protected())
	require.Nil(tx.To())
	require.Equal("100000000000", tx.GasPrice().String())
	encoding, sig, pubkey, err := action.ExtractTypeSigPubkey(tx)
	require.NoError(err)
	require.Equal(iotextypes.Encoding_ETHEREUM_UNPROTECTED, encoding)
	// convert tx to envelope
	elp, err = (&action.EnvelopeBuilder{}).SetChainID(cfg.Chain.ID).BuildExecution(tx)
	require.NoError(err)
	ex2, err := (&action.Deserializer{}).SetEvmNetworkID(cfg.Chain.EVMNetworkID).
		ActionToSealedEnvelope(&iotextypes.Action{
			Core:         elp.Proto(),
			SenderPubKey: pubkey.Bytes(),
			Signature:    sig,
			Encoding:     encoding,
		})
	require.NoError(err)
	require.True(address.Equal(ex2.SenderAddress(), deterministic))
	require.True(cfg.Genesis.IsDeployerWhitelisted(ex2.SenderAddress()))
	require.NoError(ap.Add(ctx, ex2))
	blockTime = blockTime.Add(time.Second)
	blk1, err := bc.MintNewBlock(blockTime)
	require.NoError(err)
	require.EqualValues(2, blk1.Height())
	require.Equal(5, len(blk1.Body.Actions))
	require.NoError(bc.CommitBlock(blk1))

	// 3 legacy fresh accounts are converted to zero-nonce account
	for _, v := range []struct {
		a     address.Address
		nonce uint64
		b     string
	}{
		{identityset.Address(24), 1, "99999999962880000000010000"},
		{identityset.Address(25), 2, "99999999980000000000009700"},
		{deterministic, 1, "6786100000000000"},
	} {
		a, err := accountutil.AccountState(ctx, sf, v.a)
		require.NoError(err)
		require.EqualValues(1, a.AccountType())
		require.Equal(v.nonce, a.PendingNonce())
		require.Equal(v.b, a.Balance.String())
	}

	// verify contract execution
	h, err = ex1.Hash()
	require.NoError(err)
	r, err = receiptByActionHash(dao, 2, h)
	require.NoError(err)
	require.EqualValues(iotextypes.ReceiptStatus_Success, r.Status)
	require.EqualValues(2, r.BlockHeight)
	require.Equal(h, r.ActionHash)
	require.EqualValues(37120, r.GasConsumed)
	require.Empty(r.ContractAddress)
	logs := r.Logs()
	require.Equal(1, len(logs))
	require.Equal(_setTopic, logs[0].Topics[1][:])
	require.True(blk1.Header.LogsBloomfilter().Exist(_setTopic))

	// verify deterministic deployment transaction
	deterministicTxHash, err := ex2.Hash()
	require.NoError(err)
	require.Equal("eddf9e61fb9d8f5111840daef55e5fde0041f5702856532cdbb5a02998033d26", hex.EncodeToString(deterministicTxHash[:]))
	r, err = receiptByActionHash(dao, 2, deterministicTxHash)
	require.NoError(err)
	require.EqualValues(iotextypes.ReceiptStatus_Success, r.Status)
	require.EqualValues(2, r.BlockHeight)
	require.Equal(deterministicTxHash, r.ActionHash)
	require.EqualValues(32139, r.GasConsumed)
	require.Equal("io1fevmgjz8kdu40pvgjgx20ralymqtf9tv3mdu7f", r.ContractAddress)
	tl, err := dao.TransactionLogs(2)
	require.NoError(err)
	require.Equal(4, len(tl.Logs))

	// Add block 3 -- test the RefactorFreshAccountConversion flag
	t1 = action.NewTransfer(big.NewInt(100), identityset.Address(25).String(), nil)
	elp = (&action.EnvelopeBuilder{}).SetNonce(0).
		SetChainID(cfg.Chain.ID).
		SetGasPrice(minGas).
		SetGasLimit(500000).
		SetAction(t1).Build()
	tsf1, err = action.Sign(elp, identityset.PrivateKey(23))
	require.NoError(err)
	require.NoError(ap.Add(ctx, tsf1))
	t2 = action.NewTransfer(big.NewInt(200), identityset.Address(24).String(), nil)
	elp = (&action.EnvelopeBuilder{}).SetNonce(1).
		SetChainID(cfg.Chain.ID).
		SetGasPrice(minGas).
		SetGasLimit(500000).
		SetAction(t2).Build()
	tsf2, err = action.Sign(elp, identityset.PrivateKey(23))
	require.NoError(err)
	require.NoError(ap.Add(ctx, tsf2))
	blockTime = blockTime.Add(time.Second)
	blk2, err := bc.MintNewBlock(blockTime)
	require.NoError(err)
	require.EqualValues(3, blk2.Height())
	require.Equal(3, len(blk2.Body.Actions))
	require.Zero(blk2.Header.GasUsed())
	require.Nil(blk2.Header.BaseFee())
	require.NoError(bc.CommitBlock(blk2))
	ap.Reset()

	// 4 legacy fresh accounts are converted to zero-nonce account
	for _, v := range []struct {
		a     address.Address
		nonce uint64
		b     string
	}{
		{identityset.Address(23), 2, "99999999980000000000029700"},
		{identityset.Address(24), 1, "99999999962880000000010200"},
		{identityset.Address(25), 2, "99999999980000000000009800"},
		{deterministic, 1, "6786100000000000"},
	} {
		a, err := accountutil.AccountState(ctx, sf, v.a)
		require.NoError(err)
		require.EqualValues(1, a.AccountType())
		require.Equal(v.nonce, a.PendingNonce())
		require.Equal(v.b, a.Balance.String())
	}

	// Add block 4 -- test the UseTxContainer, AddClaimRewardAddress, and EIP-6780 selfdestruct
	var (
		txs          [2]*types.Transaction
		contractHash hash.Hash256
	)
	txs[0] = types.NewTransaction(1, common.BytesToAddress(identityset.Address(23).Bytes()),
		big.NewInt(100), 500000, minGas, nil)
	testAddr, _ := address.FromString(testContract)
	txs[1] = types.NewTransaction(2, common.BytesToAddress(testAddr.Bytes()),
		new(big.Int), 500000, minGas, append(funcSig[:4], _sarTopic...))
	signer := types.NewEIP155Signer(big.NewInt(int64(cfg.Chain.EVMNetworkID)))
	sender := identityset.PrivateKey(24)
	for i := 0; i < 2; i++ {
		signedTx := MustNoErrorV(types.SignTx(txs[i], signer, sender.EcdsaPrivateKey().(*ecdsa.PrivateKey)))
		raw := MustNoErrorV(signedTx.MarshalBinary())
		// API receive/decode the tx
		rawString := hex.EncodeToString(raw)
		tx := MustNoErrorV(action.DecodeEtherTx(rawString))
		require.True(tx.Protected())
		require.EqualValues(cfg.Chain.EVMNetworkID, tx.ChainId().Uint64())
		encoding, sig, pubkey, err := action.ExtractTypeSigPubkey(tx)
		require.NoError(err)
		require.Equal(iotextypes.Encoding_ETHEREUM_EIP155, encoding)
		// use container to send tx
		req := iotextypes.Action{
			Core:         MustNoErrorV(action.EthRawToContainer(cfg.Chain.ID, rawString)),
			SenderPubKey: pubkey.Bytes(),
			Signature:    sig,
			Encoding:     iotextypes.Encoding_TX_CONTAINER,
		}
		// decode the tx
		selp := MustNoErrorV((&action.Deserializer{}).SetEvmNetworkID(cfg.Chain.EVMNetworkID).ActionToSealedEnvelope(&req))
		_, ok := selp.Envelope.(action.TxContainer)
		require.True(ok)
		if i == 1 {
			contractHash = MustNoErrorV(selp.Hash())
		}
		require.EqualValues(iotextypes.Encoding_TX_CONTAINER, selp.Encoding())
		require.NoError(ap.Add(ctx, selp))
		require.Equal(i+1, len(ap.GetUnconfirmedActs(sender.PublicKey().Address().String())))
		require.Equal(1, len(ap.GetUnconfirmedActs(MustNoErrorV(address.FromBytes(txs[i].To()[:])).String())))
	}
	claim := action.NewClaimFromRewardingFund(big.NewInt(200000000000), producer, nil)
	elp = (&action.EnvelopeBuilder{}).SetNonce(6).
		SetChainID(cfg.Chain.ID).
		SetGasPrice(minGas).
		SetGasLimit(100000).
		SetAction(claim).Build()
	tsf2, err = action.Sign(elp, priKey0)
	require.NoError(err)
	require.NoError(ap.Add(ctx, tsf2))
	elp = (&action.EnvelopeBuilder{}).SetNonce(7).
		SetChainID(cfg.Chain.ID).
		SetGasPrice(minGas).
		SetGasLimit(500000).
		SetAction(action.NewExecution(action.EmptyAddress, new(big.Int), _selfdestructContract)).Build()
	ex1, err = action.Sign(elp, priKey0)
	require.NoError(err)
	require.NoError(ap.Add(ctx, ex1))
	elp = (&action.EnvelopeBuilder{}).SetNonce(8).
		SetChainID(cfg.Chain.ID).
		SetGasPrice(minGas).
		SetGasLimit(500000).
		SetAction(action.NewExecution(action.EmptyAddress, new(big.Int), _selfdestructOnCreationContract)).Build()
	ex2, err = action.Sign(elp, priKey0)
	require.NoError(err)
	require.NoError(ap.Add(ctx, ex2))
	selfdestructContract := "io12fltnfupejreyl8fmd9jq6rcfextg5ra9zjwuz"
	elp = (&action.EnvelopeBuilder{}).SetNonce(9).
		SetChainID(cfg.Chain.ID).
		SetGasPrice(minGas).
		SetGasLimit(500000).
		SetAction(action.NewExecution(selfdestructContract, new(big.Int), MustNoErrorV(hex.DecodeString("0c08bf88")))).Build()
	ex3, err := action.Sign(elp, priKey0)
	require.NoError(err)
	require.NoError(ap.Add(ctx, ex3))

	blockTime = blockTime.Add(time.Second)
	blk3, err := bc.MintNewBlock(blockTime)
	require.NoError(err)
	require.EqualValues(4, blk3.Height())
	require.EqualValues(377122, blk3.Header.GasUsed())
	require.EqualValues(action.InitialBaseFee, blk3.Header.BaseFee().Uint64())
	require.Equal(7, len(blk3.Body.Actions))
	require.NoError(bc.CommitBlock(blk3))

	// verify contract execution
	r, err = receiptByActionHash(dao, 4, contractHash)
	require.NoError(err)
	require.EqualValues(iotextypes.ReceiptStatus_Success, r.Status)
	require.EqualValues(4, r.BlockHeight)
	require.Equal(contractHash, r.ActionHash)
	require.EqualValues(20020, r.GasConsumed)
	require.Empty(r.ContractAddress)
	logs = r.Logs()
	require.Equal(1, len(logs))
	require.Equal(_sarTopic, logs[0].Topics[1][:])
	require.True(blk3.Header.LogsBloomfilter().Exist(_sarTopic))

	// verify claim reward
	a, err := accountutil.AccountState(ctx, sf, producer)
	require.NoError(err)
	require.EqualValues(1, a.AccountType())
	require.Equal("200000000000", a.Balance.String())

	// verify EIP-6780 selfdestruct contract address
	for _, v := range []struct {
		h      hash.Hash256
		txType string
		gas    uint64
	}{
		{MustNoErrorV(ex1.Hash()), "selfdestruct", 269559},
		{MustNoErrorV(ex2.Hash()), "selfdestruct-oncreation", 49724},
		{MustNoErrorV(ex3.Hash()), "selfdestruct-afterwards", 17819},
	} {
		r = MustNoErrorV(receiptByActionHash(dao, 4, v.h))
		require.EqualValues(1, r.Status)
		require.EqualValues(v.gas, r.GasConsumed)
		if v.txType == "selfdestruct" {
			require.Equal(selfdestructContract, r.ContractAddress)
			a = MustNoErrorV(accountutil.AccountState(ctx, sf, MustNoErrorV(address.FromString(r.ContractAddress))))
			require.True(a.IsContract())
			require.False(a.IsNewbieAccount())
		} else if v.txType == "selfdestruct-oncreation" {
			a = MustNoErrorV(accountutil.AccountState(ctx, sf, MustNoErrorV(address.FromString(r.ContractAddress))))
			require.False(a.IsContract())
			require.True(a.IsNewbieAccount())
		}
	}

	// commit 4 blocks to a new chain
	testTriePath2, err := testutil.PathOfTempFile("trie")
	require.NoError(err)
	testDBPath2, err := testutil.PathOfTempFile("db")
	require.NoError(err)
	testIndexPath2, err := testutil.PathOfTempFile("index")
	require.NoError(err)

	defer func() {
		testutil.CleanupPath(testTriePath2)
		testutil.CleanupPath(testDBPath2)
		testutil.CleanupPath(testIndexPath2)
		// clear the gateway
		delete(cfg.Plugins, config.GatewayPlugin)
	}()

	cfg.Chain.TrieDBPath = testTriePath2
	cfg.Chain.ChainDBPath = testDBPath2
	cfg.Chain.IndexDBPath = testIndexPath2
	bc2, sf2, dao2, _, err := createChain(cfg, false)
	require.NoError(err)
	require.NoError(sf2.Register(pp))
	require.NoError(bc2.Start(ctx))
	defer func() {
		require.NoError(bc2.Stop(ctx))
	}()
	require.NoError(bc2.ValidateBlock(blk))
	require.NoError(bc2.CommitBlock(blk))
	require.NoError(bc2.ValidateBlock(blk1))
	require.NoError(bc2.CommitBlock(blk1))
	require.NoError(bc2.ValidateBlock(blk2))
	require.NoError(bc2.CommitBlock(blk2))
	require.NoError(bc2.ValidateBlock(blk3))
	require.NoError(bc2.CommitBlock(blk3))
	require.EqualValues(4, bc2.TipHeight())

	// 4 legacy fresh accounts are converted to zero-nonce account
	for _, v := range []struct {
		a     address.Address
		nonce uint64
		b     string
	}{
		{identityset.Address(23), 2, "99999999980000000000029800"},
		{identityset.Address(24), 3, "99999999932860000000010100"},
		{identityset.Address(25), 2, "99999999980000000000009800"},
		{deterministic, 1, "6786100000000000"},
	} {
		a, err := accountutil.AccountState(ctx, sf2, v.a)
		require.NoError(err)
		require.EqualValues(1, a.AccountType())
		require.EqualValues(v.nonce, a.PendingNonce())
		require.Equal(v.b, a.Balance.String())
	}

	// verify deterministic deployment transaction
	r, err = receiptByActionHash(dao2, 2, deterministicTxHash)
	require.NoError(err)
	require.EqualValues(iotextypes.ReceiptStatus_Success, r.Status)
	require.EqualValues(2, r.BlockHeight)
	require.Equal(deterministicTxHash, r.ActionHash)
	require.EqualValues(32139, r.GasConsumed)
	require.Equal("io1fevmgjz8kdu40pvgjgx20ralymqtf9tv3mdu7f", r.ContractAddress)
	tl, err = dao2.TransactionLogs(2)
	require.NoError(err)
	require.Equal(4, len(tl.Logs))

	// verify claim reward
	a, err = accountutil.AccountState(ctx, sf2, producer)
	require.NoError(err)
	require.EqualValues(1, a.AccountType())
	require.Equal("200000000000", a.Balance.String())
}

func TestConstantinople(t *testing.T) {
	testValidateBlockchain := func(cfg config.Config, t *testing.T) {
		require := require.New(t)
		ctx := context.Background()

		registry := protocol.NewRegistry()
		// Create a blockchain from scratch
		factoryCfg := factory.GenerateConfig(cfg.Chain, cfg.Genesis)
		db2, err := db.CreateKVStore(cfg.DB, cfg.Chain.TrieDBPath)
		require.NoError(err)
		sf, err := factory.NewStateDB(factoryCfg, db2, factory.RegistryStateDBOption(registry))
		require.NoError(err)
		ap, err := actpool.NewActPool(cfg.Genesis, sf, cfg.ActPool)
		require.NoError(err)
		acc := account.NewProtocol(rewarding.DepositGas)
		require.NoError(acc.Register(registry))
		rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
		require.NoError(rp.Register(registry))
		// create indexer
		cfg.DB.DbPath = cfg.Chain.IndexDBPath
		indexer, err := blockindex.NewIndexer(db.NewBoltDB(cfg.DB), cfg.Genesis.Hash())
		require.NoError(err)
		// create BlockDAO
		cfg.DB.DbPath = cfg.Chain.ChainDBPath
		store, err := filedao.NewFileDAO(cfg.DB, block.NewDeserializer(cfg.Chain.EVMNetworkID))
		require.NoError(err)
		dao := blockdao.NewBlockDAOWithIndexersAndCache(store, []blockdao.BlockIndexer{sf, indexer}, cfg.DB.MaxCacheSize)
		require.NotNil(dao)
		bc := blockchain.NewBlockchain(
			cfg.Chain,
			cfg.Genesis,
			dao,
			factory.NewMinter(sf, ap),
			blockchain.BlockValidatorOption(block.NewValidator(
				sf,
				protocol.NewGenericValidator(sf, accountutil.AccountState),
			)),
		)
		ep := execution.NewProtocol(dao.GetBlockHash, rewarding.DepositGas, fakeGetBlockTime)
		require.NoError(ep.Register(registry))
		rewardingProtocol := rewarding.NewProtocol(cfg.Genesis.Rewarding)
		require.NoError(rewardingProtocol.Register(registry))
		require.NoError(bc.Start(ctx))
		defer func() {
			require.NoError(bc.Stop(ctx))
		}()

		require.NoError(addTestingConstantinopleBlocks(bc, dao, sf, ap))

		// reason to use hard-coded hash value here:
		// this test TestConstantinople() is added when we upgrade our EVM to enable Constantinople
		// at that time, the test is run with both EVM version (Byzantium vs. Constantinople), and it generates the
		// same exact block hash, so these values stood as gatekeeper for backward-compatibility
		hashTopic := []struct {
			height  uint64
			h       hash.Hash256
			blkHash string
			topic   []byte
		}{
			{
				1,
				_deployHash,
				"d0913e38fb63b8ec7fe5ccdee098f5a3e05770f5c9f7e20b1aebc833cfc4a1e8",
				nil,
			},
			{
				2,
				_setHash,
				"a968af4cd63da732ddcae27f3ba2e0c2be34b0bba47eaa4c5fe5d9eb483a8a53",
				_setTopic,
			},
			{
				3,
				_shrHash,
				"f7728c6edaeee88c8c23390a78c711074fb05a3b6c67446abb3f9a666cd3f474",
				_shrTopic,
			},
			{
				4,
				_shlHash,
				"7ace333e5b394d729497890f90e879180551f4f4bf8813f273d247eee3f6b1cd",
				_shlTopic,
			},
			{
				5,
				_sarHash,
				"96e703f1afd4c7e0df8b91eee57db730a0c5d78cc87fd050596f4eab4f33704e",
				_sarTopic,
			},
			{
				6,
				_extHash,
				"30d8205e93b1eae9c26e3f144b7f0d5c2c809d07d2f287be3577566b4655e091",
				_extTopic,
			},
			{
				7,
				_crt2Hash,
				"01696409becb6b2585e84c3bd69decbb79a2d38fe14fb1a74cea02dd2e41b4d4",
				_crt2Topic,
			},
		}

		// test getReceipt
		for _, v := range hashTopic {
			ai, err := indexer.GetActionIndex(v.h[:])
			require.NoError(err)
			require.Equal(v.height, ai.BlockHeight())
			r, err := receiptByActionHash(dao, v.height, v.h)
			require.NoError(err)
			require.NotNil(r)
			require.Equal(uint64(1), r.Status)
			require.Equal(v.h, r.ActionHash)
			require.Equal(v.height, r.BlockHeight)
			if v.height == 1 {
				require.Equal("io1va03q4lcr608dr3nltwm64sfcz05czjuycsqgn", r.ContractAddress)
			} else {
				require.Empty(r.ContractAddress)
			}
			blk, err := dao.GetBlockByHeight(v.height)
			require.NoError(err)
			a, _, err := blk.ActionByHash(v.h)
			require.NoError(err)
			require.NotNil(a)
			aHash, err := a.Hash()
			require.NoError(err)
			require.Equal(v.h, aHash)

			blkHash, err := dao.GetBlockHash(v.height)
			require.NoError(err)
			require.Equal(v.blkHash, hex.EncodeToString(blkHash[:]))

			if v.topic != nil {
				funcSig := hash.Hash256b([]byte("Set(uint256)"))
				blk, err := dao.GetBlockByHeight(v.height)
				require.NoError(err)
				f := blk.Header.LogsBloomfilter()
				require.NotNil(f)
				require.True(f.Exist(funcSig[:]))
				require.True(f.Exist(v.topic))
			}
		}

		storeOutGasTests := []struct {
			height      uint64
			actHash     hash.Hash256
			status      iotextypes.ReceiptStatus
			preBalance  *big.Int
			postBalance *big.Int
		}{
			{
				8, _storeHash, iotextypes.ReceiptStatus_ErrCodeStoreOutOfGas, _preGrPreStore, _preGrPostStore,
			},
			{
				9, _store2Hash, iotextypes.ReceiptStatus_ErrCodeStoreOutOfGas, _preGrPostStore, _postGrPostStore,
			},
		}
		caller := identityset.Address(27)
		for _, v := range storeOutGasTests {
			r, err := receiptByActionHash(dao, v.height, v.actHash)
			require.NoError(err)
			require.EqualValues(v.status, r.Status)

			// verify transaction log
			bLog, err := dao.TransactionLogs(v.height)
			require.NoError(err)
			tLog := bLog.Logs[0]
			// first transaction log is gas fee
			tx := tLog.Transactions[0]
			require.Equal(tx.Sender, caller.String())
			require.Equal(tx.Recipient, address.RewardingPoolAddr)
			require.Equal(iotextypes.TransactionLogType_GAS_FEE, tx.Type)
			gasFee, ok := new(big.Int).SetString(tx.Amount, 10)
			require.True(ok)
			postBalance := new(big.Int).Sub(v.preBalance, gasFee)

			if !cfg.Genesis.IsGreenland(v.height) {
				// pre-Greenland contains a tx with status = ReceiptStatus_ErrCodeStoreOutOfGas
				// due to a bug the transfer is not reverted
				require.Equal(2, len(tLog.Transactions))
				// 2nd log is in-contract-transfer
				tx = tLog.Transactions[1]
				require.Equal(tx.Sender, caller.String())
				require.Equal(iotextypes.TransactionLogType_IN_CONTRACT_TRANSFER, tx.Type)
				tsfAmount, ok := new(big.Int).SetString(tx.Amount, 10)
				require.True(ok)
				postBalance.Sub(postBalance, tsfAmount)
				// post = pre - gasFee - in_contract_transfer
				require.Equal(v.postBalance, postBalance)
			} else {
				// post-Greenland fixed that bug, the transfer is reverted so it only contains the gas fee
				require.Equal(1, len(tLog.Transactions))
				// post = pre - gasFee (transfer is reverted)
				require.Equal(v.postBalance, postBalance)
			}
		}

		// test getActions
		addr27 := hash.BytesToHash160(caller.Bytes())
		total, err := indexer.GetActionCountByAddress(addr27)
		require.NoError(err)
		require.EqualValues(len(hashTopic)+len(storeOutGasTests), total)
		actions, err := indexer.GetActionsByAddress(addr27, 0, total)
		require.NoError(err)
		require.EqualValues(total, len(actions))
		for i := range hashTopic {
			require.Equal(hashTopic[i].h[:], actions[i])
		}
		for i := range storeOutGasTests {
			require.Equal(storeOutGasTests[i].actHash[:], actions[i+len(hashTopic)])
		}
	}

	require := require.New(t)
	cfg := config.Default
	testTriePath, err := testutil.PathOfTempFile("trie")
	require.NoError(err)
	testDBPath, err := testutil.PathOfTempFile("db")
	require.NoError(err)
	testIndexPath, err := testutil.PathOfTempFile("index")
	require.NoError(err)

	defer func() {
		testutil.CleanupPath(testTriePath)
		testutil.CleanupPath(testDBPath)
		testutil.CleanupPath(testIndexPath)
		// clear the gateway
		delete(cfg.Plugins, config.GatewayPlugin)
	}()

	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Chain.IndexDBPath = testIndexPath
	cfg.Chain.ProducerPrivKey = "a000000000000000000000000000000000000000000000000000000000000000"
	cfg.Genesis = genesis.TestDefault()
	cfg.Plugins[config.GatewayPlugin] = true
	cfg.Chain.EnableAsyncIndexWrite = false
	cfg.ActPool.MinGasPriceStr = "0"
	cfg.Genesis.AleutianBlockHeight = 2
	cfg.Genesis.BeringBlockHeight = 8
	cfg.Genesis.GreenlandBlockHeight = 9
	cfg.Genesis.InitBalanceMap[identityset.Address(27).String()] = unit.ConvertIotxToRau(10000000000).String()

	t.Run("test Constantinople contract", func(t *testing.T) {
		testValidateBlockchain(cfg, t)
	})
}

func TestLoadBlockchainfromDB(t *testing.T) {
	testValidateBlockchain := func(cfg config.Config, t *testing.T) {
		require := require.New(t)
		ctx := genesis.WithGenesisContext(context.Background(), cfg.Genesis)

		registry := protocol.NewRegistry()
		// Create a blockchain from scratch
		factoryCfg := factory.GenerateConfig(cfg.Chain, cfg.Genesis)
		db2, err := db.CreateKVStore(cfg.DB, cfg.Chain.TrieDBPath)
		require.NoError(err)
		sf, err := factory.NewStateDB(factoryCfg, db2, factory.RegistryStateDBOption(registry))
		require.NoError(err)
		ap, err := actpool.NewActPool(cfg.Genesis, sf, cfg.ActPool)
		require.NoError(err)
		acc := account.NewProtocol(rewarding.DepositGas)
		require.NoError(acc.Register(registry))
		rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
		require.NoError(rp.Register(registry))
		var indexer blockindex.Indexer
		indexers := []blockdao.BlockIndexer{sf}
		if _, gateway := cfg.Plugins[config.GatewayPlugin]; gateway && !cfg.Chain.EnableAsyncIndexWrite {
			// create indexer
			cfg.DB.DbPath = cfg.Chain.IndexDBPath
			indexer, err = blockindex.NewIndexer(db.NewBoltDB(cfg.DB), cfg.Genesis.Hash())
			require.NoError(err)
			indexers = append(indexers, indexer)
		}
		cfg.Genesis.InitBalanceMap[identityset.Address(27).String()] = unit.ConvertIotxToRau(10000000000).String()
		// create BlockDAO
		cfg.DB.DbPath = cfg.Chain.ChainDBPath
		store, err := filedao.NewFileDAO(cfg.DB, block.NewDeserializer(cfg.Chain.EVMNetworkID))
		require.NoError(err)
		dao := blockdao.NewBlockDAOWithIndexersAndCache(store, indexers, cfg.DB.MaxCacheSize)
		require.NotNil(dao)
		bc := blockchain.NewBlockchain(
			cfg.Chain,
			cfg.Genesis,
			dao,
			factory.NewMinter(sf, ap),
			blockchain.BlockValidatorOption(block.NewValidator(
				sf,
				protocol.NewGenericValidator(sf, accountutil.AccountState),
			)),
		)
		ep := execution.NewProtocol(dao.GetBlockHash, rewarding.DepositGas, fakeGetBlockTime)
		require.NoError(ep.Register(registry))
		require.NoError(bc.Start(ctx))

		ms := &MockSubscriber{counter: 0}
		require.NoError(bc.AddSubscriber(ms))
		require.Equal(0, ms.Counter())

		height := bc.TipHeight()
		fmt.Printf("Open blockchain pass, height = %d\n", height)
		require.NoError(addTestingTsfBlocks(cfg, bc, dao, ap))
		//make sure pubsub is completed
		err = testutil.WaitUntil(200*time.Millisecond, 3*time.Second, func() (bool, error) {
			return ms.Counter() == 24, nil
		})
		require.NoError(err)
		require.NoError(bc.Stop(ctx))

		// Load a blockchain from DB
		bc = blockchain.NewBlockchain(
			cfg.Chain,
			cfg.Genesis,
			dao,
			factory.NewMinter(sf, ap),
			blockchain.BlockValidatorOption(block.NewValidator(
				sf,
				protocol.NewGenericValidator(sf, accountutil.AccountState),
			)),
		)
		require.NoError(bc.Start(ctx))
		defer func() {
			require.NoError(bc.Stop(ctx))
		}()

		// verify block header hash
		for i := uint64(1); i <= 5; i++ {
			hash, err := dao.GetBlockHash(i)
			require.NoError(err)
			height, err = dao.GetBlockHeight(hash)
			require.NoError(err)
			require.Equal(i, height)
			header, err := bc.BlockHeaderByHeight(height)
			require.NoError(err)
			require.Equal(height, header.Height())

			// bloomfilter only exists after aleutian height
			require.Equal(height >= cfg.Genesis.AleutianBlockHeight, header.LogsBloomfilter() != nil)
		}

		empblk, err := dao.GetBlock(hash.ZeroHash256)
		require.Nil(empblk)
		require.Error(err)

		header, err := bc.BlockHeaderByHeight(60000)
		require.Nil(header)
		require.Error(err)

		// add wrong blocks
		h := bc.TipHeight()
		blkhash := bc.TipHash()
		header, err = bc.BlockHeaderByHeight(h)
		require.NoError(err)
		require.Equal(blkhash, header.HashBlock())
		fmt.Printf("Current tip = %d hash = %x\n", h, blkhash)

		// add block with wrong height
		selp, err := action.SignedTransfer(identityset.Address(29).String(), identityset.PrivateKey(27), 1, big.NewInt(50), nil, cfg.Genesis.ActionGasLimit, big.NewInt(0))
		require.NoError(err)

		nblk, err := block.NewTestingBuilder().
			SetHeight(h + 2).
			SetPrevBlockHash(blkhash).
			SetTimeStamp(testutil.TimestampNow()).
			AddActions(selp).SignAndBuild(identityset.PrivateKey(29))
		require.NoError(err)

		require.Error(bc.ValidateBlock(&nblk))
		fmt.Printf("Cannot validate block %d: %v\n", header.Height(), err)

		// add block with zero prev hash
		selp2, err := action.SignedTransfer(identityset.Address(29).String(), identityset.PrivateKey(27), 1, big.NewInt(50), nil, cfg.Genesis.ActionGasLimit, big.NewInt(0))
		require.NoError(err)

		nblk, err = block.NewTestingBuilder().
			SetHeight(h + 1).
			SetPrevBlockHash(hash.ZeroHash256).
			SetTimeStamp(testutil.TimestampNow()).
			AddActions(selp2).SignAndBuild(identityset.PrivateKey(29))
		require.NoError(err)
		err = bc.ValidateBlock(&nblk)
		require.Error(err)
		fmt.Printf("Cannot validate block %d: %v\n", header.Height(), err)

		// add existing block again will have no effect
		blk, err := dao.GetBlockByHeight(3)
		require.NotNil(blk)
		require.NoError(err)
		require.NoError(bc.CommitBlock(blk))
		fmt.Printf("Cannot add block 3 again: %v\n", err)

		// invalid address returns error
		_, err = address.FromString("")
		require.Contains(err.Error(), "address length = 0, expecting 41")

		// valid but unused address should return empty account
		addr, err := address.FromString("io1066kus4vlyvk0ljql39fzwqw0k22h7j8wmef3n")
		require.NoError(err)
		act, err := accountutil.AccountState(ctx, sf, addr)
		require.NoError(err)
		require.Equal(uint64(1), act.PendingNonce())
		require.Equal(big.NewInt(0), act.Balance)

		_, gateway := cfg.Plugins[config.GatewayPlugin]
		if gateway && !cfg.Chain.EnableAsyncIndexWrite {
			// verify deployed contract
			ai, err := indexer.GetActionIndex(_deployHash[:])
			require.NoError(err)
			r, err := receiptByActionHash(dao, ai.BlockHeight(), _deployHash)
			require.NoError(err)
			require.NotNil(r)
			require.Equal(uint64(1), r.Status)
			require.Equal(uint64(2), r.BlockHeight)

			// 2 topics in block 3 calling set()
			funcSig := hash.Hash256b([]byte("Set(uint256)"))
			blk, err := dao.GetBlockByHeight(3)
			require.NoError(err)
			f := blk.Header.LogsBloomfilter()
			require.NotNil(f)
			require.True(f.Exist(funcSig[:]))
			require.True(f.Exist(_setTopic))
			r, err = receiptByActionHash(dao, 3, _setHash)
			require.NoError(err)
			require.EqualValues(1, r.Status)
			require.EqualValues(3, r.BlockHeight)
			require.Empty(r.ContractAddress)

			// 3 topics in block 4 calling get()
			funcSig = hash.Hash256b([]byte("Get(address,uint256)"))
			blk, err = dao.GetBlockByHeight(4)
			require.NoError(err)
			f = blk.Header.LogsBloomfilter()
			require.NotNil(f)
			require.True(f.Exist(funcSig[:]))
			require.True(f.Exist(_setTopic))
			require.True(f.Exist(_getTopic))
			r, err = receiptByActionHash(dao, 4, _sarHash)
			require.NoError(err)
			require.EqualValues(1, r.Status)
			require.EqualValues(4, r.BlockHeight)
			require.Empty(r.ContractAddress)

			// txIndex/logIndex corrected in block 5
			blk, err = dao.GetBlockByHeight(5)
			require.NoError(err)
			verifyTxLogIndex(require, dao, blk, 10, 2)

			// verify genesis block index
			bi, err := indexer.GetBlockIndex(0)
			require.NoError(err)
			require.Equal(cfg.Genesis.Hash(), hash.BytesToHash256(bi.Hash()))
			require.EqualValues(0, bi.NumAction())
			require.Equal(big.NewInt(0), bi.TsfAmount())

			for h := uint64(1); h <= 5; h++ {
				// verify getting number of actions
				blk, err = dao.GetBlockByHeight(h)
				require.NoError(err)
				blkIndex, err := indexer.GetBlockIndex(h)
				require.NoError(err)
				require.EqualValues(blkIndex.NumAction(), len(blk.Actions))

				// verify getting transfer amount
				tsfs, _ := classifyActions(blk.Actions)
				tsfa := big.NewInt(0)
				for _, tsf := range tsfs {
					tsfa.Add(tsfa, tsf.Amount())
				}
				require.Equal(blkIndex.TsfAmount(), tsfa)
			}
		}
	}

	require := require.New(t)
	testTriePath, err := testutil.PathOfTempFile("trie")
	require.NoError(err)
	testDBPath, err := testutil.PathOfTempFile("db")
	require.NoError(err)
	testIndexPath, err := testutil.PathOfTempFile("index")
	require.NoError(err)

	defer func() {
		testutil.CleanupPath(testTriePath)
		testutil.CleanupPath(testDBPath)
		testutil.CleanupPath(testIndexPath)
	}()

	cfg := config.Default
	cfg.Genesis = genesis.TestDefault()
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Chain.IndexDBPath = testIndexPath
	cfg.ActPool.MinGasPriceStr = "0"
	genesis.SetGenesisTimestamp(cfg.Genesis.Timestamp)
	block.LoadGenesisHash(&cfg.Genesis)

	t.Run("load blockchain from DB w/o explorer", func(t *testing.T) {
		testValidateBlockchain(cfg, t)
	})

	testTriePath2, err := testutil.PathOfTempFile("trie")
	require.NoError(err)
	testDBPath2, err := testutil.PathOfTempFile("db")
	require.NoError(err)
	testIndexPath2, err := testutil.PathOfTempFile("index")
	require.NoError(err)

	defer func() {
		testutil.CleanupPath(testTriePath2)
		testutil.CleanupPath(testDBPath2)
		testutil.CleanupPath(testIndexPath2)
		// clear the gateway
		delete(cfg.Plugins, config.GatewayPlugin)
	}()

	cfg.Plugins[config.GatewayPlugin] = true
	cfg.Chain.TrieDBPath = testTriePath2
	cfg.Chain.ChainDBPath = testDBPath2
	cfg.Chain.IndexDBPath = testIndexPath2
	// test using sm2 signature
	cfg.Chain.SignatureScheme = []string{blockchain.SigP256sm2}
	cfg.Chain.ProducerPrivKey = "308193020100301306072a8648ce3d020106082a811ccf5501822d0479307702010104202d57ec7da578b98dad465997748ed02af0c69092ad809598073e5a2356c20492a00a06082a811ccf5501822da14403420004223356f0c6f40822ade24d47b0cd10e9285402cbc8a5028a8eec9efba44b8dfe1a7e8bc44953e557b32ec17039fb8018a58d48c8ffa54933fac8030c9a169bf6"
	cfg.Chain.EnableAsyncIndexWrite = false
	cfg.Genesis.AleutianBlockHeight = 3
	cfg.Genesis.MidwayBlockHeight = 5

	t.Run("load blockchain from DB", func(t *testing.T) {
		testValidateBlockchain(cfg, t)
	})
}

// verify the block contains all tx/log indices up to txIndex and logIndex
func verifyTxLogIndex(r *require.Assertions, dao blockdao.BlockDAO, blk *block.Block, txIndex int, logIndex uint32) {
	r.Equal(txIndex, len(blk.Actions))
	receipts, err := dao.GetReceipts(blk.Height())
	r.NoError(err)
	r.Equal(txIndex, len(receipts))

	logs := make(map[uint32]bool)
	for i := uint32(0); i < logIndex; i++ {
		logs[i] = true
	}
	for i, v := range receipts {
		r.EqualValues(1, v.Status)
		r.EqualValues(i, v.TxIndex)
		h, err := blk.Actions[i].Hash()
		r.NoError(err)
		r.Equal(h, v.ActionHash)
		// verify log index
		for _, l := range v.Logs() {
			r.Equal(h, l.ActionHash)
			r.EqualValues(i, l.TxIndex)
			r.True(logs[l.Index])
			delete(logs, l.Index)
		}
	}
	r.Zero(len(logs))
}

func TestBlockchainInitialCandidate(t *testing.T) {
	require := require.New(t)

	testTriePath, err := testutil.PathOfTempFile("trie")
	require.NoError(err)
	testDBPath, err := testutil.PathOfTempFile("db")
	require.NoError(err)
	testIndexPath, err := testutil.PathOfTempFile("index")
	require.NoError(err)

	cfg := config.Default
	cfg.Genesis = genesis.TestDefault()
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Chain.IndexDBPath = testIndexPath
	cfg.Consensus.Scheme = config.RollDPoSScheme
	registry := protocol.NewRegistry()
	factoryCfg := factory.GenerateConfig(cfg.Chain, cfg.Genesis)
	db2, err := db.CreateKVStore(cfg.DB, cfg.Chain.TrieDBPath)
	require.NoError(err)
	sf, err := factory.NewStateDB(factoryCfg, db2, factory.RegistryStateDBOption(registry))
	require.NoError(err)
	ap, err := actpool.NewActPool(cfg.Genesis, sf, cfg.ActPool)
	require.NoError(err)
	accountProtocol := account.NewProtocol(rewarding.DepositGas)
	require.NoError(accountProtocol.Register(registry))
	dbcfg := cfg.DB
	dbcfg.DbPath = cfg.Chain.ChainDBPath
	store, err := filedao.NewFileDAO(dbcfg, block.NewDeserializer(cfg.Chain.EVMNetworkID))
	require.NoError(err)
	dao := blockdao.NewBlockDAOWithIndexersAndCache(store, []blockdao.BlockIndexer{sf}, dbcfg.MaxCacheSize)
	bc := blockchain.NewBlockchain(
		cfg.Chain,
		cfg.Genesis,
		dao,
		factory.NewMinter(sf, ap),
		blockchain.BlockValidatorOption(sf),
	)
	rolldposProtocol := rolldpos.NewProtocol(
		cfg.Genesis.NumCandidateDelegates,
		cfg.Genesis.NumDelegates,
		cfg.Genesis.NumSubEpochs,
	)
	require.NoError(rolldposProtocol.Register(registry))
	rewardingProtocol := rewarding.NewProtocol(cfg.Genesis.Rewarding)
	require.NoError(rewardingProtocol.Register(registry))
	pollProtocol := poll.NewLifeLongDelegatesProtocol(cfg.Genesis.Delegates)
	require.NoError(pollProtocol.Register(registry))

	require.NoError(bc.Start(context.Background()))
	defer func() {
		require.NoError(bc.Stop(context.Background()))
		testutil.CleanupPath(testTriePath)
		testutil.CleanupPath(testDBPath)
		testutil.CleanupPath(testIndexPath)
	}()
	candidate, _, err := candidatesutil.CandidatesFromDB(sf, 1, true, false)
	require.NoError(err)
	require.Equal(24, len(candidate))
}

func TestBlockchain_AccountState(t *testing.T) {
	require := require.New(t)

	cfg := config.Default
	cfg.Genesis = genesis.TestDefault()
	ctx := genesis.WithGenesisContext(context.Background(), cfg.Genesis)
	registry := protocol.NewRegistry()
	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(acc.Register(registry))
	factoryCfg := factory.GenerateConfig(cfg.Chain, cfg.Genesis)
	sf, err := factory.NewStateDB(factoryCfg, db.NewMemKVStore(), factory.RegistryStateDBOption(registry))
	require.NoError(err)
	ap, err := actpool.NewActPool(cfg.Genesis, sf, cfg.ActPool)
	require.NoError(err)
	store, err := filedao.NewFileDAOInMemForTest()
	require.NoError(err)
	dao := blockdao.NewBlockDAOWithIndexersAndCache(store, []blockdao.BlockIndexer{sf}, cfg.DB.MaxCacheSize)
	bc := blockchain.NewBlockchain(cfg.Chain, cfg.Genesis, dao, factory.NewMinter(sf, ap))
	require.NoError(bc.Start(ctx))
	require.NotNil(bc)
	defer func() {
		require.NoError(bc.Stop(ctx))
	}()
	s, err := accountutil.AccountState(ctx, sf, identityset.Address(0))
	require.NoError(err)
	require.Equal(uint64(1), s.PendingNonce())
	require.Equal(unit.ConvertIotxToRau(100000000), s.Balance)
	require.Zero(s.Root)
	require.Nil(s.CodeHash)
}

func TestNewAccountAction(t *testing.T) {
	require := require.New(t)

	cfg := config.Default
	cfg.Genesis = genesis.TestDefault()
	cfg.Genesis.OkhotskBlockHeight = 1
	ctx := genesis.WithGenesisContext(context.Background(), cfg.Genesis)
	registry := protocol.NewRegistry()
	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(acc.Register(registry))
	factoryCfg := factory.GenerateConfig(cfg.Chain, cfg.Genesis)
	sf, err := factory.NewStateDB(factoryCfg, db.NewMemKVStore(), factory.RegistryStateDBOption(registry))
	require.NoError(err)
	ap, err := actpool.NewActPool(cfg.Genesis, sf, cfg.ActPool)
	require.NoError(err)
	store, err := filedao.NewFileDAOInMemForTest()
	require.NoError(err)
	dao := blockdao.NewBlockDAOWithIndexersAndCache(store, []blockdao.BlockIndexer{sf}, cfg.DB.MaxCacheSize)
	bc := blockchain.NewBlockchain(cfg.Chain, cfg.Genesis, dao, factory.NewMinter(sf, ap))
	require.NoError(bc.Start(ctx))
	require.NotNil(bc)
	defer func() {
		require.NoError(bc.Stop(ctx))
	}()

	// create a new address, transfer 4 IOTX
	newSk, err := iotexcrypto.HexStringToPrivateKey("55499c1b09f687488af9e4ee9e2bd53c7c8c3ddc69d4d9345a04b13030cffabe")
	require.NoError(err)
	newAddr := newSk.PublicKey().Address()
	tx, err := action.SignedTransfer(newAddr.String(), identityset.PrivateKey(0), 1, big.NewInt(4*unit.Iotx), nil, testutil.TestGasLimit, testutil.TestGasPrice)
	require.NoError(err)
	require.NoError(ap.Add(ctx, tx))
	blk, err := bc.MintNewBlock(testutil.TimestampNow())
	require.NoError(err)
	require.NoError(bc.CommitBlock(blk))
	ap.Reset()

	// initiate transfer from new address
	tx, err = action.SignedTransfer(identityset.Address(0).String(), newSk, 0, big.NewInt(unit.Iotx), nil, testutil.TestGasLimit, testutil.TestGasPrice)
	require.NoError(err)
	require.NoError(ap.Add(ctx, tx))
	tx1, err := action.SignedTransfer(identityset.Address(1).String(), newSk, 1, big.NewInt(unit.Iotx), nil, testutil.TestGasLimit, testutil.TestGasPrice)
	require.NoError(err)
	require.NoError(ap.Add(ctx, tx1))
	blk1, err := bc.MintNewBlock(testutil.TimestampNow())
	require.NoError(err)
	require.NoError(bc.CommitBlock(blk1))
	ap.Reset()

	// commit 2 blocks into a new chain
	for _, validateNonce := range []bool{false, true} {
		if validateNonce {
			cfg.Genesis.PalauBlockHeight = 2
		} else {
			cfg.Genesis.PalauBlockHeight = 20
		}
		ctx = genesis.WithGenesisContext(context.Background(), cfg.Genesis)
		factoryCfg = factory.GenerateConfig(cfg.Chain, cfg.Genesis)
		sf1, err := factory.NewStateDB(factoryCfg, db.NewMemKVStore(), factory.RegistryStateDBOption(registry))
		require.NoError(err)
		store, err := filedao.NewFileDAOInMemForTest()
		require.NoError(err)
		dao1 := blockdao.NewBlockDAOWithIndexersAndCache(store, []blockdao.BlockIndexer{sf1}, cfg.DB.MaxCacheSize)
		bc1 := blockchain.NewBlockchain(cfg.Chain, cfg.Genesis, dao1, factory.NewMinter(sf1, ap))
		require.NoError(bc1.Start(ctx))
		require.NotNil(bc1)
		defer func() {
			require.NoError(bc1.Stop(ctx))
		}()
		require.NoError(bc1.CommitBlock(blk))
		err = bc1.CommitBlock(blk1)
		if validateNonce {
			require.NoError(err)
		} else {
			require.Equal(action.ErrNonceTooHigh, errors.Cause(err))
		}

		// verify new addr
		s, err := accountutil.AccountState(ctx, sf1, newAddr)
		require.NoError(err)
		if validateNonce {
			require.EqualValues(2, s.PendingNonce())
			require.Equal(big.NewInt(2*unit.Iotx), s.Balance)
		} else {
			require.Zero(s.PendingNonce())
			require.Equal(big.NewInt(4*unit.Iotx), s.Balance)
		}
		require.Zero(s.Root)
		require.Nil(s.CodeHash)
	}
}

func TestBlocks(t *testing.T) {
	// This test is used for committing block verify benchmark purpose
	t.Skip()
	require := require.New(t)
	cfg := config.Default

	testTriePath, err := testutil.PathOfTempFile("trie")
	require.NoError(err)
	testDBPath, err := testutil.PathOfTempFile("db")
	require.NoError(err)
	testIndexPath, err := testutil.PathOfTempFile("index")
	require.NoError(err)

	a := identityset.Address(28).String()
	priKeyA := identityset.PrivateKey(28)
	c := identityset.Address(29).String()

	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Chain.IndexDBPath = testIndexPath
	cfg.Genesis = genesis.TestDefault()
	cfg.Genesis.InitBalanceMap[identityset.Address(27).String()] = unit.ConvertIotxToRau(10000000000).String()
	cfg.Genesis.InitBalanceMap[a] = "100000"
	cfg.Genesis.InitBalanceMap[c] = "100000"

	registry := protocol.NewRegistry()
	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(acc.Register(registry))
	factoryCfg := factory.GenerateConfig(cfg.Chain, cfg.Genesis)
	sf, err := factory.NewStateDB(factoryCfg, db.NewMemKVStore(), factory.RegistryStateDBOption(registry))
	require.NoError(err)
	ap, err := actpool.NewActPool(cfg.Genesis, sf, cfg.ActPool)
	require.NoError(err)
	dbcfg := cfg.DB
	dbcfg.DbPath = cfg.Chain.ChainDBPath
	store, err := filedao.NewFileDAO(dbcfg, block.NewDeserializer(cfg.Chain.EVMNetworkID))
	require.NoError(err)
	dao := blockdao.NewBlockDAOWithIndexersAndCache(store, []blockdao.BlockIndexer{sf}, dbcfg.MaxCacheSize)

	// Create a blockchain from scratch
	bc := blockchain.NewBlockchain(cfg.Chain, cfg.Genesis, dao, factory.NewMinter(sf, ap))
	require.NoError(bc.Start(context.Background()))
	defer func() {
		require.NoError(bc.Stop(context.Background()))
		testutil.CleanupPath(testTriePath)
		testutil.CleanupPath(testDBPath)
		testutil.CleanupPath(testIndexPath)
	}()

	gasLimit := testutil.TestGasLimit
	ctx := protocol.WithBlockCtx(context.Background(),
		protocol.BlockCtx{
			Producer: identityset.Address(27),
			GasLimit: gasLimit,
		})
	ctx = genesis.WithGenesisContext(ctx, cfg.Genesis)

	for i := 0; i < 10; i++ {
		actionMap := make(map[string][]*action.SealedEnvelope)
		actionMap[a] = []*action.SealedEnvelope{}
		for i := 0; i < 1000; i++ {
			tsf, err := action.SignedTransfer(c, priKeyA, 1, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
			require.NoError(err)
			require.NoError(ap.Add(context.Background(), tsf))
		}
		blk, _ := bc.MintNewBlock(testutil.TimestampNow())
		require.NoError(bc.CommitBlock(blk))
	}
}

func TestActions(t *testing.T) {
	// This test is used for block verify benchmark purpose
	t.Skip()
	require := require.New(t)
	cfg := config.Default

	cfg.Genesis = genesis.TestDefault()
	registry := protocol.NewRegistry()
	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(acc.Register(registry))

	ctx := genesis.WithGenesisContext(
		protocol.WithRegistry(context.Background(), registry),
		cfg.Genesis,
	)

	testTriePath, err := testutil.PathOfTempFile("trie")
	require.NoError(err)
	testDBPath, err := testutil.PathOfTempFile("db")
	require.NoError(err)
	testIndexPath, err := testutil.PathOfTempFile("index")
	require.NoError(err)

	a := identityset.Address(28).String()
	priKeyA := identityset.PrivateKey(28)
	c := identityset.Address(29).String()

	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Chain.IndexDBPath = testIndexPath
	cfg.Genesis.InitBalanceMap[identityset.Address(27).String()] = unit.ConvertIotxToRau(10000000000).String()
	cfg.Genesis.InitBalanceMap[a] = "100000"
	cfg.Genesis.InitBalanceMap[c] = "100000"
	factoryCfg := factory.GenerateConfig(cfg.Chain, cfg.Genesis)
	sf, err := factory.NewStateDB(factoryCfg, db.NewMemKVStore(), factory.RegistryStateDBOption(registry))
	require.NoError(err)
	ap, err := actpool.NewActPool(cfg.Genesis, sf, cfg.ActPool)
	require.NoError(err)
	dbcfg := cfg.DB
	dbcfg.DbPath = cfg.Chain.ChainDBPath
	store, err := filedao.NewFileDAO(dbcfg, block.NewDeserializer(cfg.Chain.EVMNetworkID))
	require.NoError(err)
	dao := blockdao.NewBlockDAOWithIndexersAndCache(store, []blockdao.BlockIndexer{sf}, dbcfg.MaxCacheSize)
	// Create a blockchain from scratch
	bc := blockchain.NewBlockchain(
		cfg.Chain,
		cfg.Genesis,
		dao,
		factory.NewMinter(sf, ap),
		blockchain.BlockValidatorOption(block.NewValidator(
			sf,
			protocol.NewGenericValidator(sf, accountutil.AccountState),
		)),
	)
	require.NoError(bc.Start(context.Background()))
	defer func() {
		require.NoError(bc.Stop(context.Background()))
		testutil.CleanupPath(testTriePath)
		testutil.CleanupPath(testDBPath)
		testutil.CleanupPath(testIndexPath)
	}()

	gasLimit := testutil.TestGasLimit
	ctx = protocol.WithBlockCtx(context.Background(),
		protocol.BlockCtx{
			Producer: identityset.Address(27),
			GasLimit: gasLimit,
		})
	ctx = genesis.WithGenesisContext(ctx, cfg.Genesis)

	for i := 0; i < 5000; i++ {
		tsf, err := action.SignedTransfer(c, priKeyA, 1, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
		require.NoError(err)
		require.NoError(ap.Add(context.Background(), tsf))

		tsf2, err := action.SignedTransfer(a, priKeyA, 1, big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
		require.NoError(err)
		require.NoError(ap.Add(context.Background(), tsf2))
	}
	blk, _ := bc.MintNewBlock(testutil.TimestampNow())
	ctx = protocol.WithBlockchainCtx(
		ctx,
		protocol.BlockchainCtx{
			Tip: protocol.TipInfo{
				Height: 0,
				Hash:   blk.PrevHash(),
			},
		},
	)
	require.NoError(bc.ValidateBlock(blk))
}

func TestBlockchain_AddRemoveSubscriber(t *testing.T) {
	req := require.New(t)
	cfg := config.Default
	cfg.Genesis = genesis.TestDefault()
	cfg.Genesis.BlockGasLimit = uint64(100000)
	cfg.Genesis.EnableGravityChainVoting = false
	// create chain
	registry := protocol.NewRegistry()
	factoryCfg := factory.GenerateConfig(cfg.Chain, cfg.Genesis)
	sf, err := factory.NewStateDB(factoryCfg, db.NewMemKVStore(), factory.RegistryStateDBOption(registry))
	req.NoError(err)
	ap, err := actpool.NewActPool(cfg.Genesis, sf, cfg.ActPool)
	req.NoError(err)
	store, err := filedao.NewFileDAOInMemForTest()
	req.NoError(err)
	dao := blockdao.NewBlockDAOWithIndexersAndCache(store, []blockdao.BlockIndexer{sf}, cfg.DB.MaxCacheSize)
	bc := blockchain.NewBlockchain(cfg.Chain, cfg.Genesis, dao, factory.NewMinter(sf, ap))
	// mock
	ctrl := gomock.NewController(t)
	mb := mock_blockcreationsubscriber.NewMockBlockCreationSubscriber(ctrl)
	req.Error(bc.RemoveSubscriber(mb))
	req.NoError(bc.AddSubscriber(mb))
	req.EqualError(bc.AddSubscriber(nil), "subscriber could not be nil")
	req.NoError(bc.RemoveSubscriber(mb))
	req.EqualError(bc.RemoveSubscriber(nil), "cannot find subscription")
}

func TestHistoryForAccount(t *testing.T) {
	require := require.New(t)
	bc, sf, _, _, ap := newChain(t)
	a := identityset.Address(28)
	priKeyA := identityset.PrivateKey(28)
	b := identityset.Address(29)
	ctx := genesis.WithGenesisContext(context.Background(), bc.Genesis())

	// check the original balance a and b before transfer
	AccountA, err := accountutil.AccountState(ctx, sf, a)
	require.NoError(err)
	AccountB, err := accountutil.AccountState(ctx, sf, b)
	require.NoError(err)
	require.Equal(big.NewInt(100), AccountA.Balance)
	require.Equal(big.NewInt(100), AccountB.Balance)

	// make a transfer from a to b
	actionMap := make(map[string][]*action.SealedEnvelope)
	actionMap[a.String()] = []*action.SealedEnvelope{}
	tsf, err := action.SignedTransfer(b.String(), priKeyA, 1, big.NewInt(10), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	require.NoError(err)
	require.NoError(ap.Add(context.Background(), tsf))
	blk, err := bc.MintNewBlock(testutil.TimestampNow())
	require.NoError(err)
	require.NoError(bc.ValidateBlock(blk))
	require.NoError(bc.CommitBlock(blk))

	// check balances after transfer
	AccountA, err = accountutil.AccountState(ctx, sf, a)
	require.NoError(err)
	AccountB, err = accountutil.AccountState(ctx, sf, b)
	require.NoError(err)
	require.Equal(big.NewInt(90), AccountA.Balance)
	require.Equal(big.NewInt(110), AccountB.Balance)

	// check history account's balance
	_, err = sf.WorkingSetAtHeight(ctx, 0)
	require.NoError(err)
}

func TestHistoryForContract(t *testing.T) {
	require := require.New(t)
	bc, sf, _, dao, ap := newChain(t)
	ctx := genesis.WithGenesisContext(context.Background(), bc.Genesis())
	genesisAccount := identityset.Address(27).String()
	// deploy and get contract address
	contract := deployXrc20(bc, dao, ap, t)

	contractAddr, err := address.FromString(contract)
	require.NoError(err)
	account, err := accountutil.AccountState(ctx, sf, contractAddr)
	require.NoError(err)
	// check the original balance
	balance := BalanceOfContract(contract, genesisAccount, sf, t, account.Root)
	expect, ok := new(big.Int).SetString("2000000000000000000000000000", 10)
	require.True(ok)
	require.Equal(expect, balance)
	// make a transfer for contract
	makeTransfer(contract, bc, ap, t)
	account, err = accountutil.AccountState(ctx, sf, contractAddr)
	require.NoError(err)
	// check the balance after transfer
	balance = BalanceOfContract(contract, genesisAccount, sf, t, account.Root)
	expect, ok = new(big.Int).SetString("1999999999999999999999999999", 10)
	require.True(ok)
	require.Equal(expect, balance)

	// check the original balance again
	_, err = sf.WorkingSetAtHeight(ctx, bc.TipHeight()-1)
	require.NoError(err)
}

func deployXrc20(bc blockchain.Blockchain, dao blockdao.BlockDAO, ap actpool.ActPool, t *testing.T) string {
	require := require.New(t)
	genesisPriKey := identityset.PrivateKey(27)
	// deploy a xrc20 contract with balance 2000000000000000000000000000
	data, err := hex.DecodeString("60806040526002805460ff1916601217905534801561001d57600080fd5b506040516107cd3803806107cd83398101604090815281516020808401518385015160025460ff16600a0a84026003819055336000908152600485529586205590850180519395909491019261007592850190610092565b508051610089906001906020840190610092565b5050505061012d565b828054600181600116156101000203166002900490600052602060002090601f016020900481019282601f106100d357805160ff1916838001178555610100565b82800160010185558215610100579182015b828111156101005782518255916020019190600101906100e5565b5061010c929150610110565b5090565b61012a91905b8082111561010c5760008155600101610116565b90565b6106918061013c6000396000f3006080604052600436106100ae5763ffffffff7c010000000000000000000000000000000000000000000000000000000060003504166306fdde0381146100b3578063095ea7b31461013d57806318160ddd1461017557806323b872dd1461019c578063313ce567146101c657806342966c68146101f1578063670d14b21461020957806370a082311461022a57806395d89b411461024b578063a9059cbb14610260578063dd62ed3e14610286575b600080fd5b3480156100bf57600080fd5b506100c86102ad565b6040805160208082528351818301528351919283929083019185019080838360005b838110156101025781810151838201526020016100ea565b50505050905090810190601f16801561012f5780820380516001836020036101000a031916815260200191505b509250505060405180910390f35b34801561014957600080fd5b50610161600160a060020a036004351660243561033b565b604080519115158252519081900360200190f35b34801561018157600080fd5b5061018a610368565b60408051918252519081900360200190f35b3480156101a857600080fd5b50610161600160a060020a036004358116906024351660443561036e565b3480156101d257600080fd5b506101db6103dd565b6040805160ff9092168252519081900360200190f35b3480156101fd57600080fd5b506101616004356103e6565b34801561021557600080fd5b506100c8600160a060020a036004351661045e565b34801561023657600080fd5b5061018a600160a060020a03600435166104c6565b34801561025757600080fd5b506100c86104d8565b34801561026c57600080fd5b50610284600160a060020a0360043516602435610532565b005b34801561029257600080fd5b5061018a600160a060020a0360043581169060243516610541565b6000805460408051602060026001851615610100026000190190941693909304601f810184900484028201840190925281815292918301828280156103335780601f1061030857610100808354040283529160200191610333565b820191906000526020600020905b81548152906001019060200180831161031657829003601f168201915b505050505081565b336000908152600560209081526040808320600160a060020a039590951683529390529190912055600190565b60035481565b600160a060020a038316600090815260056020908152604080832033845290915281205482111561039e57600080fd5b600160a060020a03841660009081526005602090815260408083203384529091529020805483900390556103d384848461055e565b5060019392505050565b60025460ff1681565b3360009081526004602052604081205482111561040257600080fd5b3360008181526004602090815260409182902080548690039055600380548690039055815185815291517fcc16f5dbb4873280815c1ee09dbd06736cffcc184412cf7a71a0fdb75d397ca59281900390910190a2506001919050565b60066020908152600091825260409182902080548351601f6002600019610100600186161502019093169290920491820184900484028101840190945280845290918301828280156103335780601f1061030857610100808354040283529160200191610333565b60046020526000908152604090205481565b60018054604080516020600284861615610100026000190190941693909304601f810184900484028201840190925281815292918301828280156103335780601f1061030857610100808354040283529160200191610333565b61053d33838361055e565b5050565b600560209081526000928352604080842090915290825290205481565b6000600160a060020a038316151561057557600080fd5b600160a060020a03841660009081526004602052604090205482111561059a57600080fd5b600160a060020a038316600090815260046020526040902054828101116105c057600080fd5b50600160a060020a038083166000818152600460209081526040808320805495891680855282852080548981039091559486905281548801909155815187815291519390950194927fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef929181900390910190a3600160a060020a0380841660009081526004602052604080822054928716825290205401811461065f57fe5b505050505600a165627a7a723058207c03ad12a18902cfe387e684509d310abd583d862c11e3ee80c116af8b49ec5c00290000000000000000000000000000000000000000000000000000000077359400000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000000a00000000000000000000000000000000000000000000000000000000000000004696f7478000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000004696f747800000000000000000000000000000000000000000000000000000000")
	require.NoError(err)
	execution := action.NewExecution(action.EmptyAddress, big.NewInt(0), data)
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetAction(execution).
		SetNonce(3).
		SetGasLimit(1000000).
		SetGasPrice(big.NewInt(testutil.TestGasPriceInt64)).Build()
	selp, err := action.Sign(elp, genesisPriKey)
	require.NoError(err)

	require.NoError(ap.Add(context.Background(), selp))

	blk, err := bc.MintNewBlock(testutil.TimestampNow())
	require.NoError(err)
	require.NoError(bc.CommitBlock(blk))
	selpHash, err := selp.Hash()
	require.NoError(err)
	r, err := receiptByActionHash(dao, blk.Height(), selpHash)
	require.NoError(err)
	return r.ContractAddress
}

func BalanceOfContract(contract, genesisAccount string, sr protocol.StateReader, t *testing.T, root hash.Hash256) *big.Int {
	require := require.New(t)
	addr, err := address.FromString(contract)
	require.NoError(err)
	addrHash := hash.BytesToHash160(addr.Bytes())
	dbForTrie := protocol.NewKVStoreForTrieWithStateReader(evm.ContractKVNameSpace, sr)
	options := []mptrie.Option{
		mptrie.KVStoreOption(dbForTrie),
		mptrie.KeyLengthOption(len(hash.Hash256{})),
		mptrie.HashFuncOption(func(data []byte) []byte {
			h := hash.Hash256b(append(addrHash[:], data...))
			return h[:]
		}),
	}
	options = append(options, mptrie.RootHashOption(root[:]))
	tr, err := mptrie.New(options...)
	require.NoError(err)
	require.NoError(tr.Start(context.Background()))
	defer tr.Stop(context.Background())
	// get producer's xrc20 balance
	addr, err = address.FromString(genesisAccount)
	require.NoError(err)
	addrHash = hash.BytesToHash160(addr.Bytes())
	checkData := "000000000000000000000000" + hex.EncodeToString(addrHash[:]) + "0000000000000000000000000000000000000000000000000000000000000004"
	hb, err := hex.DecodeString(checkData)
	require.NoError(err)
	out2 := crypto.Keccak256(hb)
	ret, err := tr.Get(out2[:])
	require.NoError(err)
	return big.NewInt(0).SetBytes(ret)
}

func newChain(t *testing.T) (blockchain.Blockchain, factory.Factory, db.KVStore, blockdao.BlockDAO, actpool.ActPool) {
	require := require.New(t)
	cfg := config.Default

	testTriePath, err := testutil.PathOfTempFile("trie")
	require.NoError(err)
	testDBPath, err := testutil.PathOfTempFile("db")
	require.NoError(err)
	testIndexPath, err := testutil.PathOfTempFile("index")
	require.NoError(err)
	defer func() {
		testutil.CleanupPath(testTriePath)
		testutil.CleanupPath(testDBPath)
		testutil.CleanupPath(testIndexPath)
	}()

	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Chain.IndexDBPath = testIndexPath
	cfg.Chain.EnableArchiveMode = true
	cfg.Consensus.Scheme = config.RollDPoSScheme
	cfg.Genesis.BlockGasLimit = uint64(1000000)
	cfg.ActPool.MinGasPriceStr = "0"
	cfg.Genesis = genesis.TestDefault()
	registry := protocol.NewRegistry()
	var sf factory.Factory
	kv := db.NewMemKVStore()
	factoryCfg := factory.GenerateConfig(cfg.Chain, cfg.Genesis)
	sf, err = factory.NewStateDB(factoryCfg, kv, factory.RegistryStateDBOption(registry))
	require.NoError(err)

	ap, err := actpool.NewActPool(cfg.Genesis, sf, cfg.ActPool)
	require.NoError(err)
	acc := account.NewProtocol(rewarding.DepositGas)
	require.NoError(acc.Register(registry))
	rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
	require.NoError(rp.Register(registry))
	var indexer blockindex.Indexer
	indexers := []blockdao.BlockIndexer{sf}
	if _, gateway := cfg.Plugins[config.GatewayPlugin]; gateway && !cfg.Chain.EnableAsyncIndexWrite {
		// create indexer
		cfg.DB.DbPath = cfg.Chain.IndexDBPath
		indexer, err = blockindex.NewIndexer(db.NewBoltDB(cfg.DB), cfg.Genesis.Hash())
		require.NoError(err)
		indexers = append(indexers, indexer)
	}
	cfg.Genesis.InitBalanceMap[identityset.Address(27).String()] = unit.ConvertIotxToRau(10000000000).String()
	// create BlockDAO
	cfg.DB.DbPath = cfg.Chain.ChainDBPath
	store, err := filedao.NewFileDAO(cfg.DB, block.NewDeserializer(cfg.Chain.EVMNetworkID))
	require.NoError(err)
	dao := blockdao.NewBlockDAOWithIndexersAndCache(store, indexers, cfg.DB.MaxCacheSize)
	require.NotNil(dao)
	bc := blockchain.NewBlockchain(
		cfg.Chain,
		cfg.Genesis,
		dao,
		factory.NewMinter(sf, ap),
		blockchain.BlockValidatorOption(block.NewValidator(
			sf,
			protocol.NewGenericValidator(sf, accountutil.AccountState),
		)),
	)
	require.NotNil(bc)
	ep := execution.NewProtocol(dao.GetBlockHash, rewarding.DepositGas, fakeGetBlockTime)
	require.NoError(ep.Register(registry))
	require.NoError(bc.Start(context.Background()))

	genesisPriKey := identityset.PrivateKey(27)
	a := identityset.Address(28).String()
	b := identityset.Address(29).String()
	// make a transfer from genesisAccount to a and b,because stateTX cannot store data in height 0
	tsf, err := action.SignedTransfer(a, genesisPriKey, 1, big.NewInt(100), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	require.NoError(err)
	tsf2, err := action.SignedTransfer(b, genesisPriKey, 2, big.NewInt(100), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	require.NoError(err)
	require.NoError(ap.Add(context.Background(), tsf))
	require.NoError(ap.Add(context.Background(), tsf2))
	blk, err := bc.MintNewBlock(testutil.TimestampNow())
	require.NoError(err)
	require.NoError(bc.CommitBlock(blk))
	return bc, sf, kv, dao, ap
}

func makeTransfer(contract string, bc blockchain.Blockchain, ap actpool.ActPool, t *testing.T) *block.Block {
	require := require.New(t)
	genesisPriKey := identityset.PrivateKey(27)
	// make a transfer for contract,transfer 1 to io16eur00s9gdvak4ujhpuk9a45x24n60jgecgxzz
	bytecode, err := hex.DecodeString("a9059cbb0000000000000000000000004867c4bada9553216bf296c4c64e9ff0749206490000000000000000000000000000000000000000000000000000000000000001")
	require.NoError(err)
	execution := action.NewExecution(contract, big.NewInt(0), bytecode)
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetAction(execution).
		SetNonce(4).
		SetGasLimit(1000000).
		SetGasPrice(big.NewInt(testutil.TestGasPriceInt64)).Build()
	selp, err := action.Sign(elp, genesisPriKey)
	require.NoError(err)
	require.NoError(ap.Add(context.Background(), selp))
	blk, err := bc.MintNewBlock(testutil.TimestampNow())
	require.NoError(err)
	require.NoError(bc.CommitBlock(blk))
	return blk
}

// classifyActions classfies actions
func classifyActions(actions []*action.SealedEnvelope) ([]*action.Transfer, []*action.Execution) {
	tsfs := make([]*action.Transfer, 0)
	exes := make([]*action.Execution, 0)
	for _, elp := range actions {
		act := elp.Action()
		switch act := act.(type) {
		case *action.Transfer:
			tsfs = append(tsfs, act)
		case *action.Execution:
			exes = append(exes, act)
		}
	}
	return tsfs, exes
}

func receiptByActionHash(dao blockdao.BlockDAO, height uint64, h hash.Hash256) (*action.Receipt, error) {
	receipts, err := dao.GetReceipts(height)
	if err != nil {
		return nil, err
	}
	for _, receipt := range receipts {
		if receipt.ActionHash == h {
			return receipt, nil
		}
	}
	return nil, errors.Errorf("failed to find receipt for %x", h)
}

// TODO: add func TestValidateBlock()
