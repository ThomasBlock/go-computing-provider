package computing

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/filswan/go-mcs-sdk/mcs/api/common/logs"
	"github.com/swanchain/go-computing-provider/conf"
	"github.com/swanchain/go-computing-provider/internal/contract"
	"github.com/swanchain/go-computing-provider/internal/contract/ecp"
	"github.com/swanchain/go-computing-provider/internal/db"
	"github.com/swanchain/go-computing-provider/internal/models"
	"io"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

const (
	TransferInterval = 10000
)

type TaskPaymentService struct {
	payment *ecp.TaskPayment
	client  *ethclient.Client
}

func NewTaskPaymentService() *TaskPaymentService {
	return &TaskPaymentService{}
}

func (tps *TaskPaymentService) ScannerChainGetTaskPayment() {
	if conf.GetConfig().CONTRACT.EdgeTaskPayment == "" {
		return
	}
	chainRpc, err := conf.GetRpcByNetWorkName()
	if err != nil {
		logs.GetLogger().Errorf("failed to get chain rpc, error: %v", err)
		return
	}
	client, err := contract.GetEthClient(chainRpc)
	if err != nil {
		logs.GetLogger().Errorf("error: %v", err)
		return
	}
	defer client.Close()

	contractAddress := common.HexToAddress(conf.GetConfig().CONTRACT.EdgeTaskPayment)
	taskPayment, err := ecp.NewTaskPayment(contractAddress, client)
	if err != nil {
		logs.GetLogger().Errorf("faile to create task payment client, error: %v", err)
		return
	}

	tps.payment = taskPayment
	tps.client = client

	cpAccountAddress, err := contract.GetCpAccountAddress()
	if err != nil {
		logs.GetLogger().Errorf("failed to get cp account contract address, error: %v", err)
		return
	}

	if err := tps.scanAndProcessEvents(cpAccountAddress); err != nil {
		logs.GetLogger().Errorf("%v", err)
	}
}

func (tps *TaskPaymentService) scanAndProcessEvents(cpAccountAddress string) error {
	lastProcessedBlock := loadLastProcessedBlock(models.ScannerTaskPaymentId)
	header, err := tps.client.HeaderByNumber(context.Background(), nil)
	if err != nil {
		time.Sleep(10 * time.Second)
		if strings.Contains(err.Error(), "Too Many Requests") {
			return fmt.Errorf("failed to get chain header, error: Too Many Requests")
		}
		return fmt.Errorf("failed to get chain header, error: %v", err)
	}
	currentBlock := header.Number.Uint64()

	filterOpts := &bind.FilterOpts{
		Start:   uint64(lastProcessedBlock + 1),
		End:     &currentBlock,
		Context: context.Background(),
	}

	iter, err := tps.payment.FilterTransferToCPBeneficiary(filterOpts, []common.Address{common.HexToAddress(cpAccountAddress)})
	if err != nil {
		return fmt.Errorf("failed to get transfer events, error: %v", err)
	}

	for iter.Next() {
		event := iter.Event
		logs.GetLogger().Infof("handle event: TaskUUID=%s, Account=%s, CPAccount=%s, Beneficiary=%s, TransferAmount=%s",
			event.TaskUUID, event.Account.Hex(), event.CpAccount.Hex(), event.Beneficiary.Hex(), event.TransferAmount.String())

		lastProcessedBlock = int64(event.Raw.BlockNumber)
		handleEdgeTask(event.TaskUUID, cpAccountAddress, lastProcessedBlock, event.TransferAmount)
	}
	if iter.Error() != nil {
		return fmt.Errorf("failed to iterator events, error: %v", iter.Error())
	}
	saveLastProcessedBlock(lastProcessedBlock, models.ScannerTaskPaymentId)
	return nil
}

func handleEdgeTask(taskUuid, cpAccountAddress string, blockNumber int64, amount *big.Int) {
	ecpJob, err := NewEcpJobService().GetEcpJobByUuid(taskUuid)
	if err != nil {
		logs.GetLogger().Errorf("failed to get edge task, error: %v", err)
		return
	}

	NewEcpJobService().UpdateEcpJobEntityRewardAndBlock(taskUuid, blockNumber, ecpJob.Reward+balanceToFloat(amount))
	status, err := getTaskStatus(taskUuid, cpAccountAddress)
	if err != nil {
		logs.GetLogger().Errorf("%v", err)
		return
	}
	if status {
		if err = NewDockerService().RemoveContainerByName(ecpJob.ContainerName); err != nil {
			logs.GetLogger().Errorf("failed to remove container, job_uuid: %s, error: %v", ecpJob.Uuid, err)
			return
		}
		logs.GetLogger().Infof("scanner_deleted, task_uuid: %s", taskUuid)
		NewEcpJobService().DeleteContainerByUuid(ecpJob.Uuid)
	}
}

func checkAgain(cpAccountAddress string, blockNumber int64) {
	ecpJobs, err := NewEcpJobService().GetEcpJobs("")
	if err != nil {
		logs.GetLogger().Errorf("failed to get edge task, error: %v", err)
		return
	}

	for _, job := range ecpJobs {
		status, err := getTaskStatus(job.Uuid, cpAccountAddress)
		if err != nil {
			logs.GetLogger().Errorf("%v", err)
			continue
		}
		if status || blockNumber-job.LastBlockNumber >= 5000 {
			if err = NewDockerService().RemoveContainerByName(job.ContainerName); err != nil {
				logs.GetLogger().Errorf("failed to remove container, job_uuid: %s, error: %v", job.Uuid, err)
				return
			}
			NewEcpJobService().DeleteContainerByUuid(job.Uuid)
		}
	}
}

func loadLastProcessedBlock(taskTypeOnChain int) int64 {
	var scan models.ScanChainEntity
	err := db.DB.Model(models.ScanChainEntity{}).Where(&models.ScanChainEntity{Id: int64(taskTypeOnChain)}).Limit(1).Find(&scan).Error
	if err != nil {
		logs.GetLogger().Errorf("failed to get scan chain, error: %v", err)
		return conf.GetConfig().CONTRACT.EdgeTaskPaymentCreated
	}

	if taskTypeOnChain == models.ScannerTaskPaymentId {
		if scan.BlockNumber == 0 {
			return conf.GetConfig().CONTRACT.EdgeTaskPaymentCreated
		}
	} else if taskTypeOnChain == models.ScannerFcpTaskManagerId {
		if scan.BlockNumber == 0 {
			return conf.GetConfig().CONTRACT.JobManagerCreated
		}
	}
	return scan.BlockNumber
}

func saveLastProcessedBlock(block int64, taskTypeOnChain int) {
	db.DB.Save(&models.ScanChainEntity{
		Id:          int64(taskTypeOnChain),
		BlockNumber: block,
		UpdateTime:  time.Now().Format("2006-01-02 15:04:05"),
	})
}

func getTaskStatus(taskUuid, cpAccount string) (bool, error) {
	req, err := http.NewRequest("GET", conf.GetConfig().UBI.EdgeUrl+fmt.Sprintf("/cps/%s/%s", cpAccount, taskUuid), nil)
	if err != nil {
		logs.GetLogger().Errorf("failed to create request, error: %v", err)
		return false, err
	}

	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("failed to read response: %v", err)
	}
	logs.GetLogger().Infof("task_status: http_status: %d, response: %s", resp.StatusCode, string(body))

	if resp.StatusCode == http.StatusBadRequest {
		return true, nil
	}

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("failed to parse resp body: %v", string(body))
	}

	if resp.StatusCode != http.StatusBadRequest {
		return true, nil
	}

	var ts TaskStatus
	err = json.Unmarshal(body, &ts)
	if err != nil {
		return false, fmt.Errorf("failed to response convert to json, error: %v", err)
	}
	return ts.Data.Ended, nil
}

type TaskStatus struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Ended  bool   `json:"ended"`
		Status string `json:"status"`
	} `json:"data"`
}

func balanceToFloat(balance *big.Int) float64 {
	var ethValue float64
	if balance.String() == "0" {
		ethValue = 0
	} else {
		fbalance := new(big.Float)
		fbalance.SetString(balance.String())
		etherQuotient := new(big.Float).Quo(fbalance, new(big.Float).SetInt(big.NewInt(1e18)))
		ethValue, _ = etherQuotient.Float64()
	}
	return ethValue
}
