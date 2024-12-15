package util

import (
	libconstants "github.com/filswan/go-swan-lib/constants"
)

type BasicResponse struct {
	Status  string      `json:"status"`
	Code    int         `json:"code"`
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message,omitempty"`
}

func CreateSuccessResponse(_data interface{}) BasicResponse {
	return BasicResponse{
		Status: libconstants.SWAN_API_STATUS_SUCCESS,
		Data:   _data,
		Code:   SuccessCode,
	}
}

func CreateErrorResponse(code int, errMsg ...string) BasicResponse {
	var msg string
	if len(errMsg) == 0 {
		msg = codeMsg[code]
	} else {
		msg = errMsg[0]
	}
	return BasicResponse{
		Status:  libconstants.SWAN_API_STATUS_FAIL,
		Code:    code,
		Message: msg,
	}
}

const (
	SuccessCode = 200
	ServerError = 500

	GetLocationError           = 3000
	GetCpAccountError          = 3001
	GeResourceError            = 3002
	JsonError                  = 4000
	BadParamError              = 4001
	SignatureError             = 4002
	SpaceParseResourceUriError = 4003
	CheckResourcesError        = 4004
	SpaceCheckWhiteListError   = 4005
	NoAvailableResourcesError  = 4006
	FoundJobEntityError        = 4007
	NotFoundJobEntityError     = 4008
	SaveJobEntityError         = 4009
	FoundWhiteListError        = 4010
	FoundBlackListError        = 4011
	SpaceCheckBlackListError   = 4012
	CheckBalanceError          = 4013
	RejectZkTaskError          = 4014
	DownloadResourceError      = 4015
	PortNoAvailableError       = 4016
	GenerateRsaError           = 4017
	SaveRsaKeyError            = 4018
	ReadRsaKeyError            = 4019
	CheckNodePortError         = 4020
	CheckResourceLimitError    = 4021
	NotAcceptNodePortError     = 4022
	RpcConnectError            = 4023
	CheckPriceError            = 4024
	BelowPriceError            = 4025
	ReadPriceError             = 4026
	ReadLogError               = 4027

	ProofParamError   = 7001
	ProofReadLogError = 7002
	ProofError        = 7003

	UbiTaskParamError    = 8001
	UbiTaskContractError = 8002
	FoundTaskEntityError = 8003
	SaveTaskEntityError  = 8004
	SubmitProofError     = 8005
)

var codeMsg = map[int]string{
	ServerError:                "Service failed",
	GetLocationError:           "An error occurred while get location of cp",
	GetCpAccountError:          "An error occurred while get cp account address",
	GeResourceError:            "An error occurred while get cp account resource",
	JsonError:                  "Invalid JSON format",
	BadParamError:              "The request parameter is not valid",
	SignatureError:             "Verify signature failed",
	SpaceParseResourceUriError: "An error occurred while parsing sourceUri",
	CheckResourcesError:        "An error occurred while check resources available",
	SpaceCheckWhiteListError:   "This cp does not accept tasks from wallet addresses outside the whitelist",
	SpaceCheckBlackListError:   "This cp does not accept tasks from wallet addresses inside the blacklist",
	NoAvailableResourcesError:  "No resources available",
	FoundJobEntityError:        "An error occurred while get job info",
	NotFoundJobEntityError:     "No found this Job",
	SaveJobEntityError:         "An error occurred while save job info",
	FoundWhiteListError:        "An error occurred while get whitelist",
	FoundBlackListError:        "An error occurred while get blacklist",
	DownloadResourceError:      "An error occurred while download space resource",
	PortNoAvailableError:       "Port number unavailable",
	GenerateRsaError:           "An error occurred while generate rsa key pair",
	SaveRsaKeyError:            "An error occurred while save rsa key pair",
	ReadRsaKeyError:            "An error occurred while read rsa key pair",
	CheckPriceError:            "Unable to verify price",
	BelowPriceError:            "Bid price below minimum requirement",
	CheckNodePortError:         "An error occurred while check cluster port availability",
	CheckResourceLimitError:    "An error occurred while check resource limit components",
	NotAcceptNodePortError:     "not accept node port type job",
	RpcConnectError:            "An error occurred while connect rpc",
	ReadPriceError:             "An error occurred while read price info",
	ReadLogError:               "failed to read logs",

	ProofReadLogError: "An error occurred while read the log of proof",
	ProofError:        "An error occurred while executing the calculation task",

	RejectZkTaskError:    "refuse to accept zk-task",
	CheckBalanceError:    "An error occurred while check balance of cp account address",
	UbiTaskContractError: "Not found this task contract on the chain",
	FoundTaskEntityError: "An error occurred while get task info",
	SaveTaskEntityError:  "An error occurred while save task info",
	SubmitProofError:     "An error occurred while submit proof",
}
