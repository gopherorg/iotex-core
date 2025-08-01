// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package genesis

import (
	"math"
	"math/big"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"go.uber.org/config"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/pkg/unit"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

var (
	// Default contains the default genesis config
	Default = defaultConfig()

	_genesisTs     int64
	_loadGenesisTs sync.Once
)

func defaultConfig() Genesis {
	return Genesis{
		Blockchain: Blockchain{
			Timestamp:                 1553558500,
			BlockGasLimit:             20000000,
			TsunamiBlockGasLimit:      50000000,
			WakeBlockGasLimit:         25000000,
			ActionGasLimit:            5000000,
			BlockInterval:             10 * time.Second,
			NumSubEpochs:              15,
			DardanellesNumSubEpochs:   30,
			WakeNumSubEpochs:          60,
			NumDelegates:              24,
			NumCandidateDelegates:     36,
			TimeBasedRotation:         true,
			MinBlocksForBlobRetention: 345600,
			PacificBlockHeight:        432001,
			AleutianBlockHeight:       864001,
			BeringBlockHeight:         1512001,
			CookBlockHeight:           1641601,
			DardanellesBlockHeight:    1816201,
			DaytonaBlockHeight:        3238921,
			EasterBlockHeight:         4478761,
			FbkMigrationBlockHeight:   5157001,
			FairbankBlockHeight:       5165641,
			GreenlandBlockHeight:      6544441,
			HawaiiBlockHeight:         11267641,
			IcelandBlockHeight:        12289321,
			JutlandBlockHeight:        13685401,
			KamchatkaBlockHeight:      13816441,
			LordHoweBlockHeight:       13979161,
			MidwayBlockHeight:         16509241,
			NewfoundlandBlockHeight:   17662681,
			OkhotskBlockHeight:        21542761,
			PalauBlockHeight:          22991401,
			QuebecBlockHeight:         24838201,
			RedseaBlockHeight:         26704441,
			SumatraBlockHeight:        28516681,
			TsunamiBlockHeight:        29275561,
			UpernavikBlockHeight:      31174201,
			VanuatuBlockHeight:        33730921,
			WakeBlockHeight:           36893881,
			ToBeEnabledBlockHeight:    math.MaxUint64,
		},
		Account: Account{
			InitBalanceMap:          map[string]string{},
			ReplayDeployerWhitelist: []string{"0x3fab184622dc19b6109349b94811493bf2a45362"},
		},
		Poll: Poll{
			PollMode:                         "nativeMix",
			EnableGravityChainVoting:         true,
			GravityChainCeilingHeight:        10199000,
			ProbationEpochPeriod:             6,
			ProbationIntensityRate:           90,
			UnproductiveDelegateMaxCacheSize: 20,
			SystemStakingContractAddress:     "io1drde9f483guaetl3w3w6n6y7yv80f8fael7qme", // https://iotexscout.io/tx/8b899515d180d631abe8596b091380b0f42117122415393fa459c74c2bc5b6af
			SystemStakingContractHeight:      24486464,
			SystemStakingContractV2Address:   "io13mjjr5shj4mte39axwsqjp8fdggk0qzjhatprp", // https://iotexscan.io/tx/b838b7a7c95e511fd8b256c5cbafde0547a72215d682eb60668d1b475a1beb70
			SystemStakingContractV2Height:    30934838,
			SystemStakingContractV3Address:   "io1vkcvq4ywarvfj4u9zwlqedfsttalq55jmtmqcu", // https://iotexscan.io/tx/0261599524be26cd0a5bdfffc4df1316b244306b4d31488bf60d3f6cbfa6722e
			SystemStakingContractV3Height:    36726575,
			NativeStakingContractAddress:     "io1xpq62aw85uqzrccg9y5hnryv8ld2nkpycc3gza",
			VoteThreshold:                    "100000000000000000000",
			StakingContractAddress:           "0x87c9dbff0016af23f5b1ab9b8e072124ab729193",
			SelfStakingThreshold:             "1200000000000000000000000",
			ScoreThreshold:                   "2000000000000000000000000",
			RegisterContractAddress:          "0x95724986563028deb58f15c5fac19fa09304f32d",
			GravityChainStartHeight:          7614500,
			GravityChainHeightInterval:       100,
			Delegates:                        []Delegate{},
		},
		Rewarding: Rewarding{
			InitBalanceStr:             unit.ConvertIotxToRau(200000000).String(),
			BlockRewardStr:             unit.ConvertIotxToRau(16).String(),
			DardanellesBlockRewardStr:  unit.ConvertIotxToRau(8).String(),
			EpochRewardStr:             unit.ConvertIotxToRau(12500).String(),
			AleutianEpochRewardStr:     unit.ConvertIotxToRau(18750).String(),
			NumDelegatesForEpochReward: 100,
			ExemptAddrStrsFromEpochReward: []string{
				"io15fqav3tugm96ge7anckx0k4gukz5m4mqf0jpv3",
				"io1x9kjkr0qv2fa7j4t2as8lrj223xxsqt4tl7xp7",
				"io1ar5l5s268rtgzshltnqv88mua06ucm58dx678y",
				"io1xsx5n94kg2zv64r7tm8vyz9mh86amfak9ka9xx",
				"io1vtm2zgn830pn6auc2cvnchgwdaefa9gr4z0s86",
				"io159fv8mu9d5djk8u2t0flgw4yqmt6fg98uqjka8",
				"io1c3r4th3zrk4uhv83a9gr4gvn3y6pzaj6mc84ea",
				"io14vmhs9c75r2ptxdaqrtk0dz7skct30pxmt69d9",
				"io1gf08snppu2a2wfd50pjas2j6q2kcxjzqph3pep",
				"io1u5ff879gg2dw9vfpxr2tsmuaz07e2rea6gvl7s",
				"io1du4eq4f88n4wyc026l3gamjwetlgsg4jz7j884",
				"io12yxdwewry70gr9fs6fphyfaky9c7gurmzk8f4f",
				"io1lx53nfgq2dnzrz5ecz2ecs7vvl6qll0mkn970w",
				"io1u5xy0ecnrjrdkzyctfqh37lgr5pcfzphgqrdwt",
				"io1aj8arp07xw6s9rgh42af5xf98csyuehnnwlk52",
				"io18gdmv5g0xhkuj2cdyvp8076uwhl7h3gesmzh8u",
				"io1td5fvamm3qf22r5h93gay6ggqdh9z0edeqx63u",
				"io1qs785af9k9xf3xgd6vut7um9zcthtrvsn2xap2",
				"io127ftn4ry6wgxdrw4hcd6gdwqlq70ujk98dvtw5",
				"io1wv5m0xyermvr2n0wjx2cjsqwyk863drdl5qfyn",
				"io1v0q5g2f8z6l3v25krl677chdx7g5pwt9kvqfpc",
				"io1xsdegzr2hdj5sv5ad4nr0yfgpsd98e40u6svem",
				"io1fks575kklxafq4fwjccmz5d3pmq5ynxk5h6h0v",
				"io15npzu93ug8r3zdeysppnyrcdu2xssz0lcam9l9",
				"io1gh7xfrsnj6p5uqgjpk9xq6jg9na28aewgp7a9v",
				"io1nyjs526mnqcsx4twa7nptkg08eclsw5c2dywp4",
				"io1jafqlvntcxgyp6e0uxctt3tljzc3vyv5hg4ukh",
				"io1z7mjef7w528nasnsafan0rp6yuvkvq405l6r8j",
				"io1cup9k8hl8fp40vrj29ex8djc346780dk223end",
				"io1scs89jur7qklzh5vfrmha3c40u8yajjx6kvzg9",
				"io10kyvvzu08pjeylymq4umknjal25ea3ptfknrpf",
				"io18mvepyxkcd5jkyplfqn27ydkpsendrey3xe2l8",
				"io1nz40npqa3yvek4zdasmqaetl2j4h6urejfkera",
				"io1m7p9yrejngxyvxhvn7p9g9uwlvd7uuamg8wcjd",
				"io1cwwm08dwv9phh3wt5vsdhu9gcypw9q2sc7pl9s",
				"io14aj46jjmtt83vts9syhrs9st80czumg40cjasl",
			},
			FoundationBonusStr:             unit.ConvertIotxToRau(80).String(),
			NumDelegatesForFoundationBonus: 36,
			FoundationBonusLastEpoch:       8760,
			FoundationBonusP2StartEpoch:    9698,
			FoundationBonusP2EndEpoch:      18458,
			ProductivityThreshold:          85,
			WakeBlockRewardStr:             "4000000000000000000",
		},
		Staking: Staking{
			VoteWeightCalConsts: VoteWeightCalConsts{
				DurationLg: 1.2,
				AutoStake:  1,
				SelfStake:  1.06,
			},
			RegistrationConsts: RegistrationConsts{
				Fee:          unit.ConvertIotxToRau(100).String(),
				MinSelfStake: unit.ConvertIotxToRau(1200000).String(),
			},
			WithdrawWaitingPeriod:            3 * 24 * time.Hour,
			MinStakeAmount:                   unit.ConvertIotxToRau(100).String(),
			BootstrapCandidates:              []BootstrapCandidate{},
			EndorsementWithdrawWaitingBlocks: 24 * 60 * 60 / 5,
		},
	}
}

func defaultInitBalanceMap() map[string]string {
	return map[string]string{
		"io1uqhmnttmv0pg8prugxxn7d8ex9angrvfjfthxa": "9800000000000000000000000000",
		"io1v3gkc49d5vwtdfdka2ekjl3h468egun8e43r7z": "100000000000000000000000000",
		"io1vrl48nsdm8jaujccd9cx4ve23cskr0ys6urx92": "100000000000000000000000000",
		"io1llupp3n8q5x8usnr5w08j6hc6hn55x64l46rr7": "100000000000000000000000000",
		"io1ns7y0pxmklk8ceattty6n7makpw76u770u5avy": "100000000000000000000000000",
		"io1xuavja5dwde8pvy4yms06yyncad4yavghjhwra": "100000000000000000000000000",
		"io1cdqx6p5rquudxuewflfndpcl0l8t5aezen9slr": "100000000000000000000000000",
		"io1hh97f273nhxcq8ajzcpujtt7p9pqyndfmavn9r": "100000000000000000000000000",
		"io1yhvu38epz5vmkjaclp45a7t08r27slmcc0zjzh": "100000000000000000000000000",
		"io1cl6rl2ev5dfa988qmgzg2x4hfazmp9vn2g66ng": "100000000000000000000000000",
		"io1skmqp33qme8knyw0fzgt9takwrc2nvz4sevk5c": "100000000000000000000000000",
		"io1fxzh50pa6qc6x5cprgmgw4qrp5vw97zk5pxt3q": "100000000000000000000000000",
		"io1jh0ekmccywfkmj7e8qsuzsupnlk3w5337hjjg2": "100000000000000000000000000",
		"io1juvx5g063eu4ts832nukp4vgcwk2gnc5cu9ayd": "100000000000000000000000000",
		"io19d0p3ah4g8ww9d7kcxfq87yxe7fnr8rpth5shj": "100000000000000000000000000",
		"io1ed52svvdun2qv8sf2m0xnynuxfaulv6jlww7ur": "100000000000000000000000000",
		"io158hyzrmf4a8xll7gfc8xnwlv70jgp44tzy5nvd": "100000000000000000000000000",
		"io19kshh892255x4h5ularvr3q3al2v8cgl80fqrt": "100000000000000000000000000",
		"io1ph0u2psnd7muq5xv9623rmxdsxc4uapxhzpg02": "100000000000000000000000000",
		"io1znka733xefxjjw2wqddegplwtefun0mfdmz7dw": "100000000000000000000000000",
		"io13sj9mzpewn25ymheukte4v39hvjdtrfp00mlyv": "100000000000000000000000000",
		"io14gnqxf9dpkn05g337rl7eyt2nxasphf5m6n0rd": "100000000000000000000000000",
		"io1l3wc0smczyay8xq747e2hw63mzg3ctp6uf8wsg": "100000000000000000000000000",
		"io1q4tdrahguffdu4e9j9aj4f38p2nee0r9vlhx7s": "100000000000000000000000000",
		"io1k9y4a9juk45zaqwvjmhtz6yjc68twqds4qcvzv": "100000000000000000000000000",
		"io15flratm0nhh5xpxz2lznrrpmnwteyd86hxdtj0": "100000000000000000000000000",
		"io1eq4ehs6xx6zj9gcsax7h3qydwlxut9xcfcjras": "100000000000000000000000000",
		"io10a298zmzvrt4guq79a9f4x7qedj59y7ery84he": "100000000000000000000000000",
	}
}

// TestDefault is the default genesis config for testing
func TestDefault() Genesis {
	cfg := defaultConfig()
	cfg.InitBalanceMap = map[string]string{}
	cfg.NumSubEpochs = 2
	cfg.Timestamp = 1546329600
	cfg.TimeBasedRotation = false
	cfg.PacificBlockHeight = 0
	cfg.NumSubEpochs = 2
	cfg.EnableGravityChainVoting = false
	for i := 0; i < identityset.Size(); i++ {
		addr := identityset.Address(i).String()
		value := unit.ConvertIotxToRau(100000000).String()
		cfg.InitBalanceMap[addr] = value
		if uint64(i) < cfg.NumDelegates {
			cfg.Delegates = append(cfg.Delegates, Delegate{
				OperatorAddrStr: addr,
				RewardAddrStr:   addr,
				VotesStr:        value,
			})
		}
	}

	return cfg
}

type (
	// Genesis is the root level of genesis config. Genesis config is the network-wide blockchain config. All the nodes
	// participating into the same network should use EXACTLY SAME genesis config.
	Genesis struct {
		Blockchain `yaml:"blockchain"`
		Account    `yaml:"account"`
		Poll       `yaml:"poll"`
		Rewarding  `yaml:"rewarding"`
		Staking    `yaml:"staking"`
	}
	// Blockchain contains blockchain level configs
	Blockchain struct {
		// Timestamp is the timestamp of the genesis block
		Timestamp int64
		// BlockGasLimit is the total gas limit could be consumed in a block
		BlockGasLimit uint64 `yaml:"blockGasLimit"`
		// TsunamiBlockGasLimit is the block gas limit starting Tsunami height (raised to 50M by default)
		TsunamiBlockGasLimit uint64 `yaml:"tsunamiBlockGasLimit"`
		// WakeBlockGasLimit is the block gas limit starting Wake height (reduced to 30M by default)
		WakeBlockGasLimit uint64 `yaml:"wakeBlockGasLimit"`
		// ActionGasLimit is the per action gas limit cap
		ActionGasLimit uint64 `yaml:"actionGasLimit"`
		// BlockInterval is the interval between two blocks
		BlockInterval time.Duration `yaml:"blockInterval"`
		// NumSubEpochs is the number of sub epochs in one epoch of block production
		NumSubEpochs uint64 `yaml:"numSubEpochs"`
		// DardanellesNumSubEpochs is the number of sub epochs starts from dardanelles height in one epoch of block production
		DardanellesNumSubEpochs uint64 `yaml:"dardanellesNumSubEpochs"`
		// WakeNumSubEpochs is the number of sub epochs starts from wake height in one epoch of block production
		WakeNumSubEpochs uint64 `yaml:"wakeNumSubEpochs"`
		// NumDelegates is the number of delegates that participate into one epoch of block production
		NumDelegates uint64 `yaml:"numDelegates"`
		// NumCandidateDelegates is the number of candidate delegates, who may be selected as a delegate via roll dpos
		NumCandidateDelegates uint64 `yaml:"numCandidateDelegates"`
		// TimeBasedRotation is the flag to enable rotating delegates' time slots on a block height
		TimeBasedRotation bool `yaml:"timeBasedRotation"`
		// MinBlocksForBlobRetention is the minimum number of blocks for blob retention
		MinBlocksForBlobRetention uint64 `yaml:"minBlocksForBlobRetention"`
		// PacificBlockHeight is the start height of using the logic of Pacific version
		// TODO: PacificBlockHeight is not added into protobuf definition for backward compatibility
		PacificBlockHeight uint64 `yaml:"pacificHeight"`
		// AleutianBlockHeight is the start height of adding bloom filter of all events into block header
		AleutianBlockHeight uint64 `yaml:"aleutianHeight"`
		// BeringBlockHeight is the start height of evm upgrade
		BeringBlockHeight uint64 `yaml:"beringHeight"`
		// CookBlockHeight is the start height of native staking
		CookBlockHeight uint64 `yaml:"cookHeight"`
		// DardanellesBlockHeight is the start height of 5s block internal
		DardanellesBlockHeight uint64 `yaml:"dardanellesHeight"`
		// DaytonaBlockHeight is the height to fix low gas for read native staking contract
		DaytonaBlockHeight uint64 `yaml:"daytonaBlockHeight"`
		// EasterBlockHeight is the start height of probation for slashing
		EasterBlockHeight uint64 `yaml:"easterHeight"`
		// FbkMigrationBlockHeight is the start height for fairbank migration
		FbkMigrationBlockHeight uint64 `yaml:"fbkMigrationHeight"`
		// FairbankBlockHeight is the start height to switch to native staking V2
		FairbankBlockHeight uint64 `yaml:"fairbankHeight"`
		// GreenlandBlockHeight is the start height of storing latest 720 block meta and rewarding/staking bucket pool
		GreenlandBlockHeight uint64 `yaml:"greenlandHeight"`
		// HawaiiBlockHeight is the start height to
		// 1. fix GetBlockHash in EVM
		// 2. add revert message to log
		// 3. fix change to same candidate in staking protocol
		// 4. fix sorted map in StateDBAdapter
		// 5. use pending nonce in EVM
		HawaiiBlockHeight uint64 `yaml:"hawaiiHeight"`
		// IcelandBlockHeight is the start height to support chainID opcode and EVM Istanbul
		IcelandBlockHeight uint64 `yaml:"icelandHeight"`
		// JutlandBlockHeight is the start height to
		// 1. report more EVM error codes
		// 2. enable the opCall fix
		JutlandBlockHeight uint64 `yaml:"jutlandHeight"`
		// KamchatkaBlockHeight is the start height to
		// 1. fix EVM snapshot order
		// 2. extend foundation bonus
		KamchatkaBlockHeight uint64 `yaml:"kamchatkaHeight"`
		// LordHoweBlockHeight is the start height to
		// 1. recover the smart contracts affected by snapshot order
		// 2. clear snapshots in Revert()
		LordHoweBlockHeight uint64 `yaml:"lordHoweHeight"`
		// MidwayBlockHeight is the start height to
		// 1. allow correct and default ChainID
		// 2. fix GetHashFunc in EVM
		// 3. correct tx/log index for transaction receipt and EVM log
		// 4. revert logs upon tx reversion in EVM
		MidwayBlockHeight uint64 `yaml:"midwayHeight"`
		// NewfoundlandBlockHeight is the start height to
		// 1. use correct chainID
		// 2. check legacy address
		// 3. enable web3 staking transaction
		NewfoundlandBlockHeight uint64 `yaml:"newfoundlandHeight"`
		// OkhotskBlockHeight is the start height to
		// 1. enable London EVM
		// 2. create zero-nonce account
		// 3. fix gas and nonce update
		// 4. fix unproductive delegates in staking protocol
		OkhotskBlockHeight uint64 `yaml:"okhotskHeight"`
		// PalauBlockHeight is the start height to
		// 1. enable rewarding action via web3
		// 2. broadcast node info into the p2p network
		PalauBlockHeight uint64 `yaml:"palauHeight"`
		// QuebecBlockHeight is the start height to
		// 1. enforce using correct chainID only
		// 2. enable IIP-13 liquidity staking
		// 3. valiate system action layout
		QuebecBlockHeight uint64 `yaml:"quebecHeight"`
		// RedseaBlockHeight is the start height to
		// 1. upgrade go-ethereum to Bellatrix release
		// 2. correct weighted votes for contract staking bucket
		RedseaBlockHeight uint64 `yaml:"redseaHeight"`
		// SumatraBlockHeight is the start height to enable Shanghai EVM
		SumatraBlockHeight uint64 `yaml:"sumatraHeight"`
		// TsunamiBlockHeight is the start height to
		// 1. enable delegate endorsement
		// 2. raise block gas limit to 50M
		TsunamiBlockHeight uint64 `yaml:"tsunamiHeight"`
		// UpernavikBlockHeight is the start height to
		// 1. enable new NFT staking contract
		// 2. migrate native staking bucket to NFT staking
		// 3. delegate ownership transfer
		// 4. send EVM transaction in general container format
		// 5. generate transaction log for SelfDestruct() call in EVM
		// 6. add address in claim reward action
		UpernavikBlockHeight uint64 `yaml:"upernavikHeight"`
		// VanuatuBlockHeight is the start height to
		// 1. enable Cancun EVM
		// 2. enable dynamic fee tx
		VanuatuBlockHeight uint64 `yaml:"vanuatuHeight"`
		// WakeBlockHeight is the start height to
		// 1. enable 3s block interval
		WakeBlockHeight uint64 `yaml:"wakeHeight"`
		// ToBeEnabledBlockHeight is a fake height that acts as a gating factor for WIP features
		// upon next release, change IsToBeEnabled() to IsNextHeight() for features to be released
		ToBeEnabledBlockHeight uint64 `yaml:"toBeEnabledHeight"`
	}
	// Account contains the configs for account protocol
	Account struct {
		// InitBalanceMap is the address and initial balance mapping before the first block.
		InitBalanceMap map[string]string `yaml:"initBalances"`
		// ReplayDeployerWhitelist is the whitelist address for unprotected (pre-EIP155) transaction
		ReplayDeployerWhitelist []string `yaml:"replayDeployerWhitelist"`
	}
	// Poll contains the configs for poll protocol
	Poll struct {
		// PollMode is different based on chain type or poll input data source
		PollMode string `yaml:"pollMode"`
		// EnableGravityChainVoting is a flag whether read voting from gravity chain
		EnableGravityChainVoting bool `yaml:"enableGravityChainVoting"`
		// GravityChainStartHeight is the height in gravity chain where the init poll result stored
		GravityChainStartHeight uint64 `yaml:"gravityChainStartHeight"`
		// GravityChainCeilingHeight is the height in gravity chain where the poll is no longer needed
		GravityChainCeilingHeight uint64 `yaml:"gravityChainCeilingHeight"`
		// GravityChainHeightInterval the height interval on gravity chain to pull delegate information
		GravityChainHeightInterval uint64 `yaml:"gravityChainHeightInterval"`
		// RegisterContractAddress is the address of register contract
		RegisterContractAddress string `yaml:"registerContractAddress"`
		// StakingContractAddress is the address of staking contract
		StakingContractAddress string `yaml:"stakingContractAddress"`
		// NativeStakingContractAddress is the address of native staking contract
		NativeStakingContractAddress string `yaml:"nativeStakingContractAddress"`
		// NativeStakingContractCode is the code of native staking contract
		NativeStakingContractCode string `yaml:"nativeStakingContractCode"`
		// ConsortiumCommitteeContractCode is the code of consortiumCommittee contract
		ConsortiumCommitteeContractCode string `yaml:"consortiumCommitteeContractCode"`
		// VoteThreshold is the vote threshold amount in decimal string format
		VoteThreshold string `yaml:"voteThreshold"`
		// ScoreThreshold is the score threshold amount in decimal string format
		ScoreThreshold string `yaml:"scoreThreshold"`
		// SelfStakingThreshold is self-staking vote threshold amount in decimal string format
		SelfStakingThreshold string `yaml:"selfStakingThreshold"`
		// Delegates is a list of delegates with votes
		Delegates []Delegate `yaml:"delegates"`
		// ProbationEpochPeriod is a duration of probation after delegate's productivity is lower than threshold
		ProbationEpochPeriod uint64 `yaml:"probationEpochPeriod"`
		// ProbationIntensityRate is a intensity rate of probation range from [0, 100], where 100 is hard-probation
		ProbationIntensityRate uint32 `yaml:"probationIntensityRate"`
		// UnproductiveDelegateMaxCacheSize is a max cache size of upd which is stored into state DB (probationEpochPeriod <= UnproductiveDelegateMaxCacheSize)
		UnproductiveDelegateMaxCacheSize uint64 `yaml:"unproductiveDelegateMaxCacheSize"`
		// SystemStakingContractAddress is the address of system staking contract
		SystemStakingContractAddress string `yaml:"systemStakingContractAddress"`
		// SystemStakingContractHeight is the height of system staking contract
		SystemStakingContractHeight uint64 `yaml:"systemStakingContractHeight"`
		// deprecated
		SystemSGDContractAddress string `yaml:"systemSGDContractAddress"`
		// deprecated
		SystemSGDContractHeight uint64 `yaml:"systemSGDContractHeight"`
		// SystemStakingContractV2Address is the address of system staking contract
		SystemStakingContractV2Address string `yaml:"systemStakingContractV2Address"`
		// SystemStakingContractV2Height is the height of system staking contract
		SystemStakingContractV2Height uint64 `yaml:"systemStakingContractV2Height"`
		// SystemStakingContractV3Address is the address of system staking contract
		SystemStakingContractV3Address string `yaml:"systemStakingContractV3Address"`
		// SystemStakingContractV3Height is the height of system staking contract
		SystemStakingContractV3Height uint64 `yaml:"systemStakingContractV3Height"`
	}
	// Delegate defines a delegate with address and votes
	Delegate struct {
		// OperatorAddrStr is the address who will operate the node
		OperatorAddrStr string `yaml:"operatorAddr"`
		// RewardAddrStr is the address who will get the reward when operator produces blocks
		RewardAddrStr string `yaml:"rewardAddr"`
		// VotesStr is the score for the operator to rank and weight for rewardee to split epoch reward
		VotesStr string `yaml:"votes"`
	}
	// Rewarding contains the configs for rewarding protocol
	Rewarding struct {
		// InitBalanceStr is the initial balance of the rewarding protocol in decimal string format
		InitBalanceStr string `yaml:"initBalance"`
		// BlockReward is the block reward amount in decimal string format
		BlockRewardStr string `yaml:"blockReward"`
		// DardanellesBlockReward is the block reward amount starts from dardanelles height in decimal string format
		DardanellesBlockRewardStr string `yaml:"dardanellesBlockReward"`
		// EpochReward is the epoch reward amount in decimal string format
		EpochRewardStr string `yaml:"epochReward"`
		// AleutianEpochRewardStr is the epoch reward amount in decimal string format after aleutian fork
		AleutianEpochRewardStr string `yaml:"aleutianEpochReward"`
		// NumDelegatesForEpochReward is the number of top candidates that will share a epoch reward
		NumDelegatesForEpochReward uint64 `yaml:"numDelegatesForEpochReward"`
		// ExemptAddrStrsFromEpochReward is the list of addresses in encoded string format that exempt from epoch reward
		ExemptAddrStrsFromEpochReward []string `yaml:"exemptAddrsFromEpochReward"`
		// FoundationBonusStr is the bootstrap bonus in decimal string format
		FoundationBonusStr string `yaml:"foundationBonus"`
		// NumDelegatesForFoundationBonus is the number of top candidate that will get the bootstrap bonus
		NumDelegatesForFoundationBonus uint64 `yaml:"numDelegatesForFoundationBonus"`
		// FoundationBonusLastEpoch is the last epoch number that bootstrap bonus will be granted
		FoundationBonusLastEpoch uint64 `yaml:"foundationBonusLastEpoch"`
		// FoundationBonusP2StartEpoch is the start epoch number for part 2 foundation bonus
		FoundationBonusP2StartEpoch uint64 `yaml:"foundationBonusP2StartEpoch"`
		// FoundationBonusP2EndEpoch is the end epoch number for part 2 foundation bonus
		FoundationBonusP2EndEpoch uint64 `yaml:"foundationBonusP2EndEpoch"`
		// ProductivityThreshold is the percentage number that a delegate's productivity needs to reach not to get probation
		ProductivityThreshold uint64 `yaml:"productivityThreshold"`
		// WakeBlockReward is the block reward amount starts from wake height in decimal string format
		WakeBlockRewardStr string `yaml:"wakeBlockRewardStr"`
	}
	// Staking contains the configs for staking protocol
	Staking struct {
		VoteWeightCalConsts              VoteWeightCalConsts  `yaml:"voteWeightCalConsts"`
		RegistrationConsts               RegistrationConsts   `yaml:"registrationConsts"`
		WithdrawWaitingPeriod            time.Duration        `yaml:"withdrawWaitingPeriod"`
		MinStakeAmount                   string               `yaml:"minStakeAmount"`
		BootstrapCandidates              []BootstrapCandidate `yaml:"bootstrapCandidates"`
		EndorsementWithdrawWaitingBlocks uint64               `yaml:"endorsementWithdrawWaitingBlocks"`
	}

	// VoteWeightCalConsts contains the configs for calculating vote weight
	VoteWeightCalConsts struct {
		DurationLg float64 `yaml:"durationLg"`
		AutoStake  float64 `yaml:"autoStake"`
		SelfStake  float64 `yaml:"selfStake"`
	}

	// RegistrationConsts contains the configs for candidate registration
	RegistrationConsts struct {
		Fee          string `yaml:"fee"`
		MinSelfStake string `yaml:"minSelfStake"`
	}

	// BootstrapCandidate is the candidate data need to be provided to bootstrap candidate.
	BootstrapCandidate struct {
		OwnerAddress      string `yaml:"ownerAddress"`
		OperatorAddress   string `yaml:"operatorAddress"`
		RewardAddress     string `yaml:"rewardAddress"`
		Name              string `yaml:"name"`
		SelfStakingTokens string `yaml:"selfStakingTokens"`
	}
)

// New constructs a genesis config. It loads the default values, and could be overwritten by values defined in the yaml
// config files
func New(genesisPath string) (Genesis, error) {
	def := defaultConfig()

	opts := make([]config.YAMLOption, 0)
	opts = append(opts, config.Static(def))
	if genesisPath != "" {
		opts = append(opts, config.File(genesisPath))
	}
	yaml, err := config.NewYAML(opts...)
	if err != nil {
		return Genesis{}, errors.Wrap(err, "error when constructing a genesis in yaml")
	}

	var genesis Genesis
	if err := yaml.Get(config.Root).Populate(&genesis); err != nil {
		return Genesis{}, errors.Wrap(err, "failed to unmarshal yaml genesis to struct")
	}
	if len(genesis.InitBalanceMap) == 0 {
		genesis.InitBalanceMap = defaultInitBalanceMap()
	}
	return genesis, nil
}

// SetGenesisTimestamp sets the genesis timestamp
func SetGenesisTimestamp(ts int64) {
	_loadGenesisTs.Do(func() {
		_genesisTs = ts
	})
}

// Timestamp returns the genesis timestamp
func Timestamp() int64 {
	return atomic.LoadInt64(&_genesisTs)
}

// Hash is the hash of genesis config
func (g *Genesis) Hash() hash.Hash256 {
	gbProto := iotextypes.GenesisBlockchain{
		Timestamp:             g.Timestamp,
		BlockGasLimit:         g.BlockGasLimit,
		ActionGasLimit:        g.ActionGasLimit,
		BlockInterval:         g.BlockInterval.Nanoseconds(),
		NumSubEpochs:          g.NumSubEpochs,
		NumDelegates:          g.NumDelegates,
		NumCandidateDelegates: g.NumCandidateDelegates,
		TimeBasedRotation:     g.TimeBasedRotation,
	}

	initBalanceAddrs := make([]string, 0)
	for initBalanceAddr := range g.InitBalanceMap {
		initBalanceAddrs = append(initBalanceAddrs, initBalanceAddr)
	}
	sort.Strings(initBalanceAddrs)
	initBalances := make([]string, 0)
	for _, initBalanceAddr := range initBalanceAddrs {
		initBalances = append(initBalances, g.InitBalanceMap[initBalanceAddr])
	}
	aProto := iotextypes.GenesisAccount{
		InitBalanceAddrs: initBalanceAddrs,
		InitBalances:     initBalances,
	}

	dProtos := make([]*iotextypes.GenesisDelegate, 0)
	for _, d := range g.Delegates {
		dProto := iotextypes.GenesisDelegate{
			OperatorAddr: d.OperatorAddrStr,
			RewardAddr:   d.RewardAddrStr,
			Votes:        d.VotesStr,
		}
		dProtos = append(dProtos, &dProto)
	}
	pProto := iotextypes.GenesisPoll{
		EnableGravityChainVoting: g.EnableGravityChainVoting,
		GravityChainStartHeight:  g.GravityChainStartHeight,
		RegisterContractAddress:  g.RegisterContractAddress,
		StakingContractAddress:   g.StakingContractAddress,
		VoteThreshold:            g.VoteThreshold,
		ScoreThreshold:           g.ScoreThreshold,
		SelfStakingThreshold:     g.SelfStakingThreshold,
		Delegates:                dProtos,
	}

	rProto := iotextypes.GenesisRewarding{
		InitBalance:                    g.InitBalanceStr,
		BlockReward:                    g.BlockRewardStr,
		EpochReward:                    g.EpochRewardStr,
		NumDelegatesForEpochReward:     g.NumDelegatesForEpochReward,
		FoundationBonus:                g.FoundationBonusStr,
		NumDelegatesForFoundationBonus: g.NumDelegatesForFoundationBonus,
		FoundationBonusLastEpoch:       g.FoundationBonusLastEpoch,
		ProductivityThreshold:          g.ProductivityThreshold,
	}

	gProto := iotextypes.Genesis{
		Blockchain: &gbProto,
		Account:    &aProto,
		Poll:       &pProto,
		Rewarding:  &rProto,
	}
	b, err := proto.Marshal(&gProto)
	if err != nil {
		log.L().Panic("Error when marshaling genesis proto", zap.Error(err))
	}
	return hash.Hash256b(b)
}

func (g *Blockchain) isPost(targetHeight, height uint64) bool {
	return height >= targetHeight
}

// IsPacific checks whether height is equal to or larger than pacific height
func (g *Blockchain) IsPacific(height uint64) bool {
	return g.isPost(g.PacificBlockHeight, height)
}

// IsAleutian checks whether height is equal to or larger than aleutian height
func (g *Blockchain) IsAleutian(height uint64) bool {
	return g.isPost(g.AleutianBlockHeight, height)
}

// IsBering checks whether height is equal to or larger than bering height
func (g *Blockchain) IsBering(height uint64) bool {
	return g.isPost(g.BeringBlockHeight, height)
}

// IsCook checks whether height is equal to or larger than cook height
func (g *Blockchain) IsCook(height uint64) bool {
	return g.isPost(g.CookBlockHeight, height)
}

// IsDardanelles checks whether height is equal to or larger than dardanelles height
func (g *Blockchain) IsDardanelles(height uint64) bool {
	return g.isPost(g.DardanellesBlockHeight, height)
}

// IsDaytona checks whether height is equal to or larger than daytona height
func (g *Blockchain) IsDaytona(height uint64) bool {
	return g.isPost(g.DaytonaBlockHeight, height)
}

// IsEaster checks whether height is equal to or larger than easter height
func (g *Blockchain) IsEaster(height uint64) bool {
	return g.isPost(g.EasterBlockHeight, height)
}

// IsFairbank checks whether height is equal to or larger than fairbank height
func (g *Blockchain) IsFairbank(height uint64) bool {
	return g.isPost(g.FairbankBlockHeight, height)
}

// IsFbkMigration checks whether height is equal to or larger than fbk migration height
func (g *Blockchain) IsFbkMigration(height uint64) bool {
	return g.isPost(g.FbkMigrationBlockHeight, height)
}

// IsGreenland checks whether height is equal to or larger than greenland height
func (g *Blockchain) IsGreenland(height uint64) bool {
	return g.isPost(g.GreenlandBlockHeight, height)
}

// IsHawaii checks whether height is equal to or larger than hawaii height
func (g *Blockchain) IsHawaii(height uint64) bool {
	return g.isPost(g.HawaiiBlockHeight, height)
}

// IsIceland checks whether height is equal to or larger than iceland height
func (g *Blockchain) IsIceland(height uint64) bool {
	return g.isPost(g.IcelandBlockHeight, height)
}

// IsJutland checks whether height is equal to or larger than jutland height
func (g *Blockchain) IsJutland(height uint64) bool {
	return g.isPost(g.JutlandBlockHeight, height)
}

// IsKamchatka checks whether height is equal to or larger than kamchatka height
func (g *Blockchain) IsKamchatka(height uint64) bool {
	return g.isPost(g.KamchatkaBlockHeight, height)
}

// IsLordHowe checks whether height is equal to or larger than lordHowe height
func (g *Blockchain) IsLordHowe(height uint64) bool {
	return g.isPost(g.LordHoweBlockHeight, height)
}

// IsMidway checks whether height is equal to or larger than midway height
func (g *Blockchain) IsMidway(height uint64) bool {
	return g.isPost(g.MidwayBlockHeight, height)
}

// IsNewfoundland checks whether height is equal to or larger than newfoundland height
func (g *Blockchain) IsNewfoundland(height uint64) bool {
	return g.isPost(g.NewfoundlandBlockHeight, height)
}

// IsOkhotsk checks whether height is equal to or larger than okhotsk height
func (g *Blockchain) IsOkhotsk(height uint64) bool {
	return g.isPost(g.OkhotskBlockHeight, height)
}

// IsPalau checks whether height is equal to or larger than palau height
func (g *Blockchain) IsPalau(height uint64) bool {
	return g.isPost(g.PalauBlockHeight, height)
}

// IsQuebec checks whether height is equal to or larger than quebec height
func (g *Blockchain) IsQuebec(height uint64) bool {
	return g.isPost(g.QuebecBlockHeight, height)
}

// IsRedsea checks whether height is equal to or larger than redsea height
func (g *Blockchain) IsRedsea(height uint64) bool {
	return g.isPost(g.RedseaBlockHeight, height)
}

// IsSumatra checks whether height is equal to or larger than sumatra height
func (g *Blockchain) IsSumatra(height uint64) bool {
	return g.isPost(g.SumatraBlockHeight, height)
}

// IsTsunami checks whether height is equal to or larger than tsunami height
func (g *Blockchain) IsTsunami(height uint64) bool {
	return g.isPost(g.TsunamiBlockHeight, height)
}

// IsUpernavik checks whether height is equal to or larger than upernavik height
func (g *Blockchain) IsUpernavik(height uint64) bool {
	return g.isPost(g.UpernavikBlockHeight, height)
}

// IsVanuatu checks whether height is equal to or larger than vanuatu height
func (g *Blockchain) IsVanuatu(height uint64) bool {
	return g.isPost(g.VanuatuBlockHeight, height)
}

// IsWake checks whether height is equal to or larger than wake height
func (g *Blockchain) IsWake(height uint64) bool {
	return g.isPost(g.WakeBlockHeight, height)
}

// IsToBeEnabled checks whether height is equal to or larger than toBeEnabled height
func (g *Blockchain) IsToBeEnabled(height uint64) bool {
	return g.isPost(g.ToBeEnabledBlockHeight, height)
}

// BlockGasLimitByHeight returns the block gas limit by height
func (g *Blockchain) BlockGasLimitByHeight(height uint64) uint64 {
	if g.isPost(g.WakeBlockHeight, height) {
		return g.WakeBlockGasLimit
	}
	if g.isPost(g.TsunamiBlockHeight, height) {
		// block gas limit raised to 50M after Tsunami block height
		return g.TsunamiBlockGasLimit
	}
	return g.BlockGasLimit
}

// IsDeployerWhitelisted returns if the replay deployer is whitelisted
func (a *Account) IsDeployerWhitelisted(deployer address.Address) bool {
	for _, v := range a.ReplayDeployerWhitelist {
		if v[:3] == "io1" {
			if addr, err := address.FromString(v); err == nil {
				if address.Equal(deployer, addr) {
					return true
				}
			}
		} else if common.IsHexAddress(v) {
			if addr, err := address.FromHex(v); err == nil {
				if address.Equal(deployer, addr) {
					return true
				}
			}
		}
	}
	return false
}

// InitBalances returns the address that have initial balances and the corresponding amounts. The i-th amount is the
// i-th address' balance.
func (a *Account) InitBalances() ([]address.Address, []*big.Int) {
	// Make the list always be ordered
	addrStrs := make([]string, 0)
	for addrStr := range a.InitBalanceMap {
		addrStrs = append(addrStrs, addrStr)
	}
	sort.Strings(addrStrs)
	addrs := make([]address.Address, 0)
	amounts := make([]*big.Int, 0)
	for _, addrStr := range addrStrs {
		addr, err := address.FromString(addrStr)
		if err != nil {
			log.L().Panic("Error when decoding the account protocol init balance address from string.", zap.Error(err))
		}
		addrs = append(addrs, addr)
		amount, ok := new(big.Int).SetString(a.InitBalanceMap[addrStr], 10)
		if !ok {
			log.S().Panicf("Error when casting init balance string %s into big int", a.InitBalanceMap[addrStr])
		}
		amounts = append(amounts, amount)
	}
	return addrs, amounts
}

// OperatorAddr is the address of operator
func (d *Delegate) OperatorAddr() address.Address {
	addr, err := address.FromString(d.OperatorAddrStr)
	if err != nil {
		log.L().Panic("Error when decoding the poll protocol operator address from string.", zap.Error(err))
	}
	return addr
}

// RewardAddr is the address of rewardee, which is allowed to be nil
func (d *Delegate) RewardAddr() address.Address {
	if d.RewardAddrStr == "" {
		return nil
	}
	addr, err := address.FromString(d.RewardAddrStr)
	if err != nil {
		log.L().Panic("Error when decoding the poll protocol rewardee address from string.", zap.Error(err))
	}
	return addr
}

// Votes returns the votes
func (d *Delegate) Votes() *big.Int {
	val, ok := new(big.Int).SetString(d.VotesStr, 10)
	if !ok {
		log.S().Panicf("Error when casting votes string %s into big int", d.VotesStr)
	}
	return val
}

// InitBalance returns the init balance of the rewarding fund
func (r *Rewarding) InitBalance() *big.Int {
	val, ok := new(big.Int).SetString(r.InitBalanceStr, 10)
	if !ok {
		log.S().Panicf("Error when casting init balance string %s into big int", r.InitBalanceStr)
	}
	return val
}

// BlockReward returns the block reward amount
func (r *Rewarding) BlockReward() *big.Int {
	val, ok := new(big.Int).SetString(r.BlockRewardStr, 10)
	if !ok {
		log.S().Panicf("Error when casting block reward string %s into big int", r.BlockRewardStr)
	}
	return val
}

// EpochReward returns the epoch reward amount
func (r *Rewarding) EpochReward() *big.Int {
	val, ok := new(big.Int).SetString(r.EpochRewardStr, 10)
	if !ok {
		log.S().Panicf("Error when casting epoch reward string %s into big int", r.EpochRewardStr)
	}
	return val
}

// AleutianEpochReward returns the epoch reward amount after Aleutian fork
func (r *Rewarding) AleutianEpochReward() *big.Int {
	val, ok := new(big.Int).SetString(r.AleutianEpochRewardStr, 10)
	if !ok {
		log.S().Panicf("Error when casting epoch reward string %s into big int", r.AleutianEpochRewardStr)
	}
	return val
}

// DardanellesBlockReward returns the block reward amount after dardanelles fork
func (r *Rewarding) DardanellesBlockReward() *big.Int {
	val, ok := new(big.Int).SetString(r.DardanellesBlockRewardStr, 10)
	if !ok {
		log.S().Panicf("Error when casting block reward string %s into big int", r.DardanellesBlockRewardStr)
	}
	return val
}

// WakeBlockReward returns the block reward amount after wake fork
func (r *Rewarding) WakeBlockReward() *big.Int {
	val, ok := new(big.Int).SetString(r.WakeBlockRewardStr, 10)
	if !ok {
		log.S().Panicf("Error when casting block reward string %s into big int", r.WakeBlockRewardStr)
	}
	return val
}

// ExemptAddrsFromEpochReward returns the list of addresses that exempt from epoch reward
func (r *Rewarding) ExemptAddrsFromEpochReward() []address.Address {
	addrs := make([]address.Address, 0)
	for _, addrStr := range r.ExemptAddrStrsFromEpochReward {
		addr, err := address.FromString(addrStr)
		if err != nil {
			log.L().Panic("Error when decoding the rewarding protocol exempt address from string.", zap.Error(err))
		}
		addrs = append(addrs, addr)
	}
	return addrs
}

// FoundationBonus returns the bootstrap bonus amount rewarded per epoch
func (r *Rewarding) FoundationBonus() *big.Int {
	val, ok := new(big.Int).SetString(r.FoundationBonusStr, 10)
	if !ok {
		log.S().Panicf("Error when casting bootstrap bonus string %s into big int", r.EpochRewardStr)
	}
	return val
}
