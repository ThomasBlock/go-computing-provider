package conf

import (
	"context"
	_ "embed"
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/swanchain/go-computing-provider/build"
	"github.com/swanchain/go-computing-provider/internal/contract"
	"log"
	"net"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var config *ComputeNode

type Pricing bool

func (p *Pricing) UnmarshalTOML(data interface{}) error {
	switch v := data.(type) {
	case bool:
		*p = Pricing(v)
	case string:
		*p = strings.ToLower(v) == "true" || v == ""
	default:
		*p = true
	}
	return nil
}

// ComputeNode is a compute node config
type ComputeNode struct {
	API      API
	UBI      UBI
	LOG      LOG
	HUB      HUB
	MCS      MCS
	Registry Registry
	RPC      RPC
	CONTRACT CONTRACT `toml:"CONTRACT,omitempty"`
}

type API struct {
	Port            int
	MultiAddress    string
	Domain          string
	NodeName        string
	WalletWhiteList string
	WalletBlackList string
	Pricing         Pricing  `toml:"pricing"`
	AutoDeleteImage bool     `toml:"AutoDeleteImage"`
	PortRange       []string `toml:"PortRange"`
}
type UBI struct {
	UbiEnginePk     string
	EnableSequencer bool
	AutoChainProof  bool
	SequencerUrl    string
	EdgeUrl         string
	VerifySign      bool
}

type LOG struct {
	CrtFile string
	KeyFile string
}

type HUB struct {
	AccessToken      string
	BalanceThreshold float64
	OrchestratorPk   string
	VerifySign       bool
}

type MCS struct {
	ApiKey     string
	BucketName string
	Network    string
}

type Registry struct {
	ServerAddress string
	UserName      string
	Password      string
}

type RPC struct {
	SwanChainRpc string `toml:"SWAN_CHAIN_RPC"`
}

type CONTRACT struct {
	UpgradeName       string
	SwanToken         string `toml:"SWAN_CONTRACT"`
	CpAccountRegister string `toml:"REGISTER_CP_CONTRACT"`

	JobCollateral     string `toml:"SWAN_COLLATERAL_CONTRACT"`
	JobManager        string `toml:"SWAN_JOB_CONTRACT"`
	JobManagerCreated uint64 `toml:"JobManagerCreated"`

	TaskRegister           string `toml:"REGISTER_TASK_CONTRACT"`
	ZkCollateral           string `toml:"ZK_COLLATERAL_CONTRACT"`
	Sequencer              string `toml:"SEQUENCER_CONTRACT"`
	EdgeTaskPayment        string `toml:"EDGE_TASK_PAYMENT"`
	EdgeTaskPaymentCreated int64  `toml:"EdgeTaskPaymentCreated"`

	JobCollateralUbiZero string `toml:"SWAN_COLLATERAL_UBI_ZERO_CONTRACT"`
	ZkCollateralUbiZero  string `toml:"ZK_COLLATERAL_UBI_ZERO_CONTRACT"`
}

func GetRpcByNetWorkName() (string, error) {
	if len(strings.TrimSpace(GetConfig().RPC.SwanChainRpc)) == 0 {
		return "", fmt.Errorf("You need to set SWAN_CHAIN_RPC in the configuration file")
	}
	return GetConfig().RPC.SwanChainRpc, nil
}

func InitConfig(cpRepoPath string, standalone bool) error {
	configFile := filepath.Join(cpRepoPath, "config.toml")

	if _, err := os.Stat(configFile); err != nil {
		return fmt.Errorf("not found %s repo, "+
			"please use `computing-provider init` to initialize the repo ", cpRepoPath)
	}

	metaData, err := toml.DecodeFile(configFile, &config)
	if err != nil {
		return fmt.Errorf("failed load config file, path: %s, error: %w", configFile, err)
	}

	if !metaData.IsDefined("Pricing") {
		config.API.Pricing = true
	}

	multiAddressSplit := strings.Split(config.API.MultiAddress, "/")
	if len(multiAddressSplit) < 4 {
		log.Fatalf("MultiAddress %s is invalid\n", multiAddressSplit[2])
	}

	if standalone {
		if !requiredFieldsAreGivenForSeparate(metaData) {
			log.Fatal("Required fields not given")
		}
	} else {
		if !requiredFieldsAreGiven(metaData) {
			log.Fatal("Required fields not given")
		}
		var domain string
		if strings.HasPrefix(config.API.Domain, ".") {
			domain = config.API.Domain[1:]
		} else {
			domain = config.API.Domain
		}

		if !isValidDomain(domain) {
			log.Fatalf("domain \"%s\" is invalid\n", domain)
		}
	}

	getConfigByHeight()
	return nil
}

func getConfigByHeight() {
	networkConfig := build.LoadParam()
	for _, nc := range networkConfig {
		ncCopy := nc
		if ncCopy.Network == build.NetWorkTag {
			var blockNumber uint64
			var err error
			var rpc string
			if config.RPC.SwanChainRpc != "" {
				rpc = config.RPC.SwanChainRpc
			} else {
				rpc = ncCopy.Config.ChainRpc
			}
			for {
				blockNumber, err = getChainHeight(rpc)
				if err != nil {
					fmt.Printf("failed to get chain height, error: %v", err)
					time.Sleep(2 * time.Second)
					continue
				}
				break
			}

			if blockNumber < ncCopy.BoundaryHeight {
				config.CONTRACT.SwanToken = ncCopy.Config.LegacyContract.SwanTokenContract
				config.CONTRACT.JobCollateral = ncCopy.Config.LegacyContract.OrchestratorCollateralContract
				config.CONTRACT.JobManager = ncCopy.Config.LegacyContract.JobManagerContract
				config.CONTRACT.JobManagerCreated = ncCopy.Config.LegacyContract.JobManagerContractCreated
				config.CONTRACT.CpAccountRegister = ncCopy.Config.LegacyContract.RegisterCpContract
				config.CONTRACT.ZkCollateral = ncCopy.Config.LegacyContract.ZkCollateralContract
				config.CONTRACT.Sequencer = ncCopy.Config.LegacyContract.SequencerContract
				config.CONTRACT.TaskRegister = ncCopy.Config.LegacyContract.RegisterTaskContract
				config.CONTRACT.EdgeTaskPayment = ncCopy.Config.LegacyContract.EdgeTaskPayment
				config.CONTRACT.EdgeTaskPaymentCreated = ncCopy.Config.LegacyContract.EdgeTaskPaymentCreated
			} else {
				config.CONTRACT.UpgradeName = ncCopy.UpgradeName
				config.CONTRACT.SwanToken = ncCopy.Config.UpgradeContract.SwanTokenContract
				config.CONTRACT.JobCollateral = ncCopy.Config.UpgradeContract.OrchestratorCollateralContract
				config.CONTRACT.JobManager = ncCopy.Config.UpgradeContract.JobManagerContract
				config.CONTRACT.JobManagerCreated = ncCopy.Config.UpgradeContract.JobManagerContractCreated
				config.CONTRACT.CpAccountRegister = ncCopy.Config.UpgradeContract.RegisterCpContract
				config.CONTRACT.ZkCollateral = ncCopy.Config.UpgradeContract.ZkCollateralContract
				config.CONTRACT.Sequencer = ncCopy.Config.UpgradeContract.SequencerContract
				config.CONTRACT.TaskRegister = ncCopy.Config.UpgradeContract.RegisterTaskContract
				config.CONTRACT.EdgeTaskPayment = ncCopy.Config.UpgradeContract.EdgeTaskPayment
				config.CONTRACT.EdgeTaskPaymentCreated = ncCopy.Config.UpgradeContract.EdgeTaskPaymentCreated
			}
			config.UBI.SequencerUrl = ncCopy.Config.SequencerUrl
			config.UBI.EdgeUrl = ncCopy.Config.EdgeUrl
			config.CONTRACT.ZkCollateralUbiZero = ncCopy.Config.ZkCollateralUbiZeroContract
			config.CONTRACT.JobCollateralUbiZero = ncCopy.Config.OrchestratorCollateralUbiZeroContract
		}
	}
}

func getChainHeight(rpc string) (uint64, error) {
	client, err := contract.GetEthClient(rpc)
	if err != nil {
		return 0, fmt.Errorf("failed to dial rpc, error: %v", err)
	}
	defer client.Close()
	return client.BlockNumber(context.Background())
}

func isValidDomain(domain string) bool {
	domainRegex := `^(?i:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,}$`
	re := regexp.MustCompile(domainRegex)
	return re.MatchString(domain)
}

func GetConfig() *ComputeNode {
	getConfigByHeight()
	return config
}

func requiredFieldsAreGiven(metaData toml.MetaData) bool {
	requiredFields := [][]string{
		{"API"},
		{"LOG"},
		{"UBI"},
		{"HUB"},
		{"MCS"},
		{"Registry"},
		{"RPC"},

		{"API", "MultiAddress"},
		{"API", "Domain"},
		{"API", "NodeName"},

		{"LOG", "CrtFile"},
		{"LOG", "KeyFile"},

		{"UBI", "UbiEnginePk"},
		{"UBI", "EnableSequencer"},
		{"UBI", "AutoChainProof"},
		{"UBI", "SequencerUrl"},

		{"HUB", "OrchestratorPk"},
		{"HUB", "BalanceThreshold"},
		{"HUB", "VerifySign"},

		{"MCS", "ApiKey"},
		{"MCS", "BucketName"},
		{"MCS", "Network"},

		{"RPC", "SWAN_CHAIN_RPC"},
	}

	for _, v := range requiredFields {
		if !metaData.IsDefined(v...) {
			log.Fatal("Required fields ", v)
		}
	}

	return true
}

func requiredFieldsAreGivenForSeparate(metaData toml.MetaData) bool {
	requiredFields := [][]string{
		{"API"},
		{"UBI"},
		{"RPC"},

		{"API", "MultiAddress"},
		{"API", "NodeName"},

		{"UBI", "UbiEnginePk"},
		{"UBI", "EnableSequencer"},
		{"UBI", "AutoChainProof"},
		{"UBI", "SequencerUrl"},

		{"RPC", "SWAN_CHAIN_RPC"},
	}

	for _, v := range requiredFields {
		if !metaData.IsDefined(v...) {
			log.Fatal("Required fields ", v)
		}
	}

	return true
}

func GenerateAndUpdateConfigFile(cpRepoPath string, multiAddress, nodeName string, port int) error {
	fmt.Println("Checking if repo exists")

	if Exists(cpRepoPath) {
		return fmt.Errorf("repo at '%s' is already initialized", cpRepoPath)
	}

	var configTmpl ComputeNode

	configFilePath := path.Join(cpRepoPath, "config.toml")
	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		defaultComputeNode := generateDefaultConfig()
		networkConfig := build.LoadParam()
		for _, nc := range networkConfig {
			ncCopy := nc
			if ncCopy.Network == build.NetWorkTag {
				defaultComputeNode.UBI.UbiEnginePk = ncCopy.Config.ZkEnginePk
				defaultComputeNode.HUB.OrchestratorPk = ncCopy.Config.OrchestratorPk
				defaultComputeNode.RPC.SwanChainRpc = ncCopy.Config.ChainRpc
				defaultComputeNode.UBI.SequencerUrl = ncCopy.Config.SequencerUrl
				defaultComputeNode.UBI.EdgeUrl = ncCopy.Config.EdgeUrl
			}
		}

		configFile, err := os.Create(configFilePath)
		if err != nil {
			return fmt.Errorf("create %s file failed, error: %v", configFilePath, err)
		}
		if err = toml.NewEncoder(configFile).Encode(defaultComputeNode); err != nil {
			return fmt.Errorf("write data to %s file failed, error: %v", configFilePath, err)
		}

		configTmpl = defaultComputeNode
	} else {
		if _, err = toml.DecodeFile(configFilePath, &configTmpl); err != nil {
			return err
		}
	}

	os.Remove(configFilePath)
	configFile, err := os.Create(configFilePath)
	if err != nil {
		return err
	}

	if len(multiAddress) != 0 && !strings.EqualFold(multiAddress, strings.TrimSpace(configTmpl.API.MultiAddress)) {
		configTmpl.API.MultiAddress = multiAddress
	}

	if len(strings.TrimSpace(nodeName)) != 0 {
		configTmpl.API.NodeName = nodeName
	} else {
		hostname, err := os.Hostname()
		if err != nil {
			return fmt.Errorf("get hostname failed, error: %v", err)
		}
		configTmpl.API.NodeName = hostname
	}

	if port != 0 {
		configTmpl.API.Port = port
	}

	if err = toml.NewEncoder(configFile).Encode(configTmpl); err != nil {
		return err
	}

	file, err := os.Create(path.Join(cpRepoPath, "provider.db"))
	if err != nil {
		return err
	}
	defer file.Close()

	if err = os.MkdirAll(path.Join(cpRepoPath, "keystore"), 0755); err != nil {
		return fmt.Errorf("failed to create keystore, error: %v", err)
	}

	fmt.Printf("Initialized CP repo at '%s'. \n", cpRepoPath)
	return nil
}

func generateDefaultConfig() ComputeNode {
	return ComputeNode{
		API: API{
			Port:            8085,
			MultiAddress:    "/ip4/<PUBLIC_IP>/tcp/<PORT>",
			Domain:          "",
			NodeName:        "<YOUR_CP_Node_Name>",
			WalletWhiteList: "",
			WalletBlackList: "",
			Pricing:         true,
		},
		UBI: UBI{
			UbiEnginePk:     "",
			EnableSequencer: true,
			AutoChainProof:  true,
			VerifySign:      true,
		},
		LOG: LOG{
			CrtFile: "",
			KeyFile: "",
		},
		HUB: HUB{
			BalanceThreshold: 0.1,
			OrchestratorPk:   "",
			VerifySign:       true,
		},
		MCS: MCS{
			ApiKey:     "",
			BucketName: "",
			Network:    "polygon.mainnet",
		},
		Registry: Registry{
			ServerAddress: "",
			UserName:      "",
			Password:      "",
		},
		RPC: RPC{
			SwanChainRpc: "",
		},
		CONTRACT: CONTRACT{
			SwanToken:              "",
			JobCollateral:          "",
			JobCollateralUbiZero:   "",
			JobManager:             "",
			JobManagerCreated:      0,
			CpAccountRegister:      "",
			TaskRegister:           "",
			ZkCollateral:           "",
			ZkCollateralUbiZero:    "",
			Sequencer:              "",
			EdgeTaskPayment:        "",
			EdgeTaskPaymentCreated: 0,
		},
	}
}

func Exists(cpPath string) bool {
	_, err := os.Stat(filepath.Join(cpPath, "keystore"))
	KeyStoreNoExist := os.IsNotExist(err)
	err = nil
	_, err = os.Stat(filepath.Join(cpPath, "provider.db"))
	providerNotExist := os.IsNotExist(err)

	if KeyStoreNoExist && providerNotExist {
		return false
	}
	return true
}

func checkDomain(domain string, expectedIP string) bool {
	ips, err := net.LookupIP(domain)
	if err != nil {
		return false
	}

	matched := false
	for _, ip := range ips {
		if ip.String() == expectedIP {
			matched = true
			break
		}
	}
	return matched
}
