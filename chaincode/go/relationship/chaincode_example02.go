package main

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	pb "github.com/hyperledger/fabric/protos/peer"
)

var logger = shim.NewLogger("OwnershipChaincode")

const (
	commonChannelName   = "common"
	commonChaincodeName = "reference"
)

// OwnershipChaincode example simple Chaincode implementation
type OwnershipChaincode struct {
}

func (t *OwnershipChaincode) Init(stub shim.ChaincodeStubInterface) pb.Response {
	return shim.Success(nil)
}

func (t *OwnershipChaincode) Invoke(stub shim.ChaincodeStubInterface) pb.Response {
	logger.Info("OwnershipChaincode.Invoke is running")
	logger.Debug("OwnershipChaincode.Invoke")

	function, args := stub.GetFunctionAndParameters()
	logger.Debug("Function: " + function + ", arguments: " + strings.Join(args, ","))

	if function == "sendRequest" {
		return t.sendRequest(stub, args)
	} else if function == "editRequest" {
		return t.editRequest(stub, args)
	} else if function == "transferAccepted" {
		return t.transferAccepted(stub, args)
	} else if function == "transferRejected" {
		return t.transferRejected(stub, args)
	} else if function == "query" {
		return t.query(stub, args)
	} else if function == "history" {
		return t.history(stub, args)
	} else if function == "sendRequestBulk" {
		return t.sendRequestBulk(stub, args)
	}

	message := "invalid invoke function name. " +
		"Expected one of {sendRequest, transferAccepted, transferRejected, query, history}, but got " + function

	logger.Error(message)
	return pb.Response{Status: 400, Message: message}
}

func (t *OwnershipChaincode) sendRequest(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	logger.Info("OwnershipChaincode.sendRequest is running")
	logger.Debug("OwnershipChaincode.sendRequest")

	const expectedArgumentsNumber = basicArgumentsNumber + 1

	if len(args) < expectedArgumentsNumber {
		message := fmt.Sprintf("insufficient number of arguments: expected %d, got %d",
			expectedArgumentsNumber, len(args))
		logger.Error(message)
		return shim.Error(message)
	}

	request := TransferDetails{}
	if err := request.FillFromArguments(args); err != nil {
		message := fmt.Sprintf("cannot read transfer details from arguments: %s", err.Error())
		logger.Error(message)
		return shim.Error(message)
	}

	if bytes, err := json.Marshal(request); err == nil {
		logger.Debug("Request: " + string(bytes))
	}

	if GetCreatorOrganization(stub) != request.Key.RequestSender {
		message := fmt.Sprintf(
			"no privileges to send request from the side of organization %s (caller is from organization %s)",
			request.Key.RequestSender, GetCreatorOrganization(stub))
		logger.Error(message)
		return pb.Response{Status: 403, Message: message}
	}

	logger.Debug("RequestSender: " + request.Key.RequestSender)

	if request.ExistsIn(stub) {
		if err := request.LoadFrom(stub); err != nil {
			message := fmt.Sprintf("cannot load existing request: %s", err.Error())
			logger.Error(message)
			return pb.Response{Status: 404, Message: message}
		}

		if request.Value.Status == statusInitiated {
			message := "ownership transfer is already initiated"
			logger.Error(message)
			return shim.Error(message)
		}
	}

	if err := checkProductExistenceAndOwnership(stub, request.Key.ProductKey, request.Key.RequestReceiver); err != nil {
		message := err.Error()
		logger.Error(message)
		return shim.Error(message)
	}

	request.Value.Status = statusInitiated
	request.Value.Message = args[basicArgumentsNumber]
	request.Value.Timestamp = time.Now().UTC().Unix()

	if err := request.UpdateOrInsertIn(stub); err != nil {
		message := fmt.Sprintf("persistence error: %s", err.Error())
		logger.Error(message)
		return pb.Response{Status: 500, Message: message}
	}

	logger.Info("OwnershipChaincode.sendRequest exited without errors")
	logger.Debug("Success: OwnershipChaincode.sendRequest")
	return shim.Success(nil)
}

func (t *OwnershipChaincode) sendRequestBulk(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	logger.Info("OwnershipChaincode.sendRequestBulk is running")
	logger.Debug("OwnershipChaincode.sendRequestBulk")

	const expectedArgumentsNumber = 5

	if len(args) < expectedArgumentsNumber {
		message := fmt.Sprintf("insufficient number of arguments: expected %d, got %d",
			expectedArgumentsNumber, len(args))
		logger.Error(message)
		return shim.Error(message)
	}

	// Required Quantity
	label := strings.ToLower(args[0])
	requiredQuantity, _ := strconv.Atoi(args[4])

	// Get Product Belongs to required Owner
	// queryString := fmt.Sprintf("{\"selector\":{\"owner\":\"%s\",\"label\":\"%s\"}}", args[2], label)
	// queryResults, err := getQueryResultForQueryString(stubReference, queryString)
	// if err != nil {
	// 	return shim.Error(err.Error())
	// }
	// availableProducts := []Product{}
	// err2 := json.Unmarshal(queryResults, &availableProducts)
	// if err2 != nil {
	// 	return shim.Error(fmt.Sprintf("product doesn't exist"))
	// }
	availableProducts, _ := getProductsFromLabel(stub, label, args[1], args[2])
	availableProductsLength := len(availableProducts)

	// If requiredQuantity < available products
	logger.Debug("-------------------- availableProducts -------------------------------------")
	logger.Debug(availableProducts)
	logger.Debug("-------------------- availableProductsLength -------------------------------------")
	logger.Debug(availableProductsLength)
	logger.Debug("-------------------- requiredQuantity -------------------------------------")
	logger.Debug(requiredQuantity)
	if requiredQuantity > availableProductsLength {
		message := "Not Enough Quantity Available"
		logger.Error(message)
		return shim.Error(message)
	}

	// Iterate
	for i := 0; i < requiredQuantity; i++ {

		finalProduct := availableProducts[i]
		var finalArgs []string
		finalArgs = append(finalArgs, finalProduct)
		finalArgs = append(finalArgs, args[1])
		finalArgs = append(finalArgs, args[2])
		finalArgs = append(finalArgs, args[3])

		request := TransferDetails{}
		if err := request.FillFromArguments(finalArgs); err != nil {
			message := fmt.Sprintf("cannot read transfer details from arguments: %s", err.Error())
			logger.Error(message)
			return shim.Error(message)
		}

		if bytes, err := json.Marshal(request); err == nil {
			logger.Debug("Request: " + string(bytes))
		}

		if GetCreatorOrganization(stub) != request.Key.RequestSender {
			message := fmt.Sprintf(
				"no privileges to send request from the side of organization %s (caller is from organization %s)",
				request.Key.RequestSender, GetCreatorOrganization(stub))
			logger.Error(message)
			return pb.Response{Status: 403, Message: message}
		}

		logger.Debug("RequestSender: " + request.Key.RequestSender)

		if request.ExistsIn(stub) {
			if err := request.LoadFrom(stub); err != nil {
				message := fmt.Sprintf("cannot load existing request: %s", err.Error())
				logger.Error(message)
				return pb.Response{Status: 404, Message: message}
			}

			if request.Value.Status == statusInitiated {
				message := "ownership transfer is already initiated"
				logger.Error(message)
				return shim.Error(message)
			}
		}

		if err := checkProductExistenceAndOwnership(stub, request.Key.ProductKey, request.Key.RequestReceiver); err != nil {
			message := err.Error()
			logger.Error(message)
			return shim.Error(message)
		}

		request.Value.Status = statusInitiated
		request.Value.Message = finalArgs[basicArgumentsNumber]
		request.Value.Timestamp = time.Now().UTC().Unix()

		if err := request.UpdateOrInsertIn(stub); err != nil {
			message := fmt.Sprintf("persistence error: %s", err.Error())
			logger.Error(message)
			return pb.Response{Status: 500, Message: message}
		}

		logger.Info("OwnershipChaincode.sendRequestBulk exited without errors")
		logger.Debug("Success: OwnershipChaincode.sendRequestBulk")

	}

	return shim.Success(nil)
}

func (t *OwnershipChaincode) editRequest(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	logger.Info("OwnershipChaincode.editRequest is running")
	logger.Debug("OwnershipChaincode.editRequest")

	const expectedArgumentsNumber = basicArgumentsNumber + 1

	if len(args) < expectedArgumentsNumber {
		message := fmt.Sprintf("insufficient number of arguments: expected %d, got %d",
			expectedArgumentsNumber, len(args))
		logger.Error(message)
		return shim.Error(message)
	}

	request := TransferDetails{}
	if err := request.FillFromArguments(args); err != nil {
		message := fmt.Sprintf("cannot read transfer details from arguments: %s", err.Error())
		logger.Error(message)
		return shim.Error(message)
	}

	if bytes, err := json.Marshal(request); err == nil {
		logger.Debug("Request: " + string(bytes))
	}

	if GetCreatorOrganization(stub) != request.Key.RequestSender {
		message := fmt.Sprintf(
			"no privileges to edit request from the side of organization %s (caller is from organization %s)",
			request.Key.RequestSender, GetCreatorOrganization(stub))
		logger.Error(message)
		return pb.Response{Status: 403, Message: message}
	}

	logger.Debug("RequestSender: " + request.Key.RequestSender)

	if !request.ExistsIn(stub) {
		message := "ownership transfer wasn't initiated"
		logger.Error(message)
		return shim.Error(message)
	}

	if err := request.LoadFrom(stub); err != nil {
		message := fmt.Sprintf("cannot load existing transfer details: %s", err.Error())
		logger.Error(message)
		return pb.Response{Status: 404, Message: message}
	}

	if request.Value.Status != statusInitiated {
		message := "ownership transfer wasn't initiated"
		logger.Error(message)
		return shim.Error(message)
	}

	request.Value.Message = args[basicArgumentsNumber]
	request.Value.Timestamp = time.Now().UTC().Unix()

	if err := request.UpdateOrInsertIn(stub); err != nil {
		message := fmt.Sprintf("persistence error: %s", err.Error())
		logger.Error(message)
		return pb.Response{Status: 500, Message: message}
	}

	logger.Info("OwnershipChaincode.editRequest exited without errors")
	logger.Debug("Success: OwnershipChaincode.editRequest")
	return shim.Success(nil)
}

func (t *OwnershipChaincode) transferAccepted(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	logger.Info("OwnershipChaincode.transferAccepted is running")
	logger.Debug("OwnershipChaincode.transferAccepted")

	if len(args) < basicArgumentsNumber {
		message := fmt.Sprintf("insufficient number of arguments: expected %d, got %d",
			basicArgumentsNumber, len(args))
		logger.Error(message)
		return shim.Error(message)
	}

	details := TransferDetails{}
	if err := details.FillFromArguments(args); err != nil {
		message := fmt.Sprintf("cannot read transfer details from arguments: %s", err.Error())
		logger.Error(message)
		return shim.Error(message)
	}

	if bytes, err := json.Marshal(details); err == nil {
		logger.Debug("Details: " + string(bytes))
	}

	if GetCreatorOrganization(stub) != details.Key.RequestReceiver {
		message := fmt.Sprintf(
			"no privileges to accept transfer from the side of organization %s (caller is from organization %s)",
			details.Key.RequestReceiver, GetCreatorOrganization(stub))
		logger.Error(message)
		return pb.Response{Status: 403, Message: message}
	}

	if !details.ExistsIn(stub) {
		message := "ownership transfer wasn't initiated"
		logger.Error(message)
		return shim.Error(message)
	}

	if err := details.LoadFrom(stub); err != nil {
		message := fmt.Sprintf("cannot load existing transfer details: %s", err.Error())
		logger.Error(message)
		return pb.Response{Status: 404, Message: message}
	}

	if details.Value.Status != statusInitiated {
		message := "ownership transfer wasn't initiated"
		logger.Error(message)
		return shim.Error(message)
	}

	if err := checkProductExistenceAndOwnership(stub, details.Key.ProductKey, details.Key.RequestReceiver); err != nil {
		// TODO: think about request deletion
		message := err.Error()
		logger.Error(message)
		return shim.Error(message)
	}

	details.Value.Status = statusAccepted
	details.Value.Timestamp = time.Now().UTC().Unix()

	if err := details.UpdateOrInsertIn(stub); err != nil {
		message := fmt.Sprintf("persistence error: %s", err.Error())
		logger.Error(message)
		return pb.Response{Status: 500, Message: message}
	}

	if err := details.EmitState(stub); err != nil {
		message := fmt.Sprintf("unable to emit outgoing event: %s", err.Error())
		logger.Error(message)
		return shim.Error(err.Error())
	}

	logger.Info("OwnershipChaincode.transferAccepted exited without errors")
	logger.Debug("Success: OwnershipChaincode.transferAccepted")
	return shim.Success(nil)
}

func (t *OwnershipChaincode) transferRejected(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	logger.Info("OwnershipChaincode.transferRejected is running")
	logger.Debug("OwnershipChaincode.transferRejected")

	if len(args) < basicArgumentsNumber {
		message := fmt.Sprintf("insufficient number of arguments: expected %d, got %d",
			basicArgumentsNumber, len(args))
		logger.Error(message)
		return shim.Error(message)
	}

	details := TransferDetails{}
	if err := details.FillFromArguments(args); err != nil {
		message := fmt.Sprintf("cannot read transfer details from arguments: %s", err.Error())
		logger.Error(message)
		return shim.Error(message)
	}

	if bytes, err := json.Marshal(details); err == nil {
		logger.Debug("Details: " + string(bytes))
	}

	creatorIsReceiver := GetCreatorOrganization(stub) == details.Key.RequestReceiver
	creatorIsSender := GetCreatorOrganization(stub) == details.Key.RequestSender

	if !creatorIsReceiver && !creatorIsSender {
		message := fmt.Sprintf(
			"no privileges to reject transfer from the side of organization %s", GetCreatorOrganization(stub))
		logger.Error(message)
		return pb.Response{Status: 403, Message: message}
	}

	if !details.ExistsIn(stub) {
		message := "ownership transfer wasn't initiated"
		logger.Error(message)
		return shim.Error(message)
	}

	if err := details.LoadFrom(stub); err != nil {
		message := fmt.Sprintf("cannot load existing transfer details: %s", err.Error())
		logger.Error(message)
		return pb.Response{Status: 404, Message: message}
	}

	if details.Value.Status != statusInitiated {
		message := "ownership transfer wasn't initiated"
		logger.Error(message)
		return shim.Error(message)
	}

	if creatorIsReceiver {
		logger.Debug("Rejected by receiver")
		details.Value.Status = statusRejected
	} else if creatorIsSender {
		logger.Debug("Rejected by sender")
		details.Value.Status = statusCancelled
	}
	details.Value.Timestamp = time.Now().UTC().Unix()

	if err := details.UpdateOrInsertIn(stub); err != nil {
		message := fmt.Sprintf("persistence error: %s", err.Error())
		logger.Error(message)
		return pb.Response{Status: 500, Message: message}
	}

	logger.Info("OwnershipChaincode.transferRejected exited without errors")
	logger.Debug("Success: OwnershipChaincode.transferRejected")
	return shim.Success(nil)
}

func (t *OwnershipChaincode) query(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	logger.Info("OwnershipChaincode.query is running")
	logger.Debug("OwnershipChaincode.query")

	it, err := stub.GetStateByPartialCompositeKey(transferIndex, []string{})
	if err != nil {
		message := fmt.Sprintf("unable to get state by partial composite key %s: %s", transferIndex, err.Error())
		logger.Error(message)
		return shim.Error(message)
	}
	defer it.Close()

	entries := []TransferDetails{}
	for it.HasNext() {
		response, err := it.Next()
		if err != nil {
			message := fmt.Sprintf("unable to get an element next to a query iterator: %s", err.Error())
			logger.Error(message)
			return shim.Error(message)
		}

		logger.Debug(fmt.Sprintf("Response: {%s, %s}", response.Key, string(response.Value)))

		entry := TransferDetails{}

		if err := entry.FillFromLedgerValue(response.Value); err != nil {
			message := fmt.Sprintf("cannot fill transfer details value from response value: %s", err.Error())
			logger.Error(message)
			return shim.Error(message)
		}

		_, compositeKeyParts, err := stub.SplitCompositeKey(response.Key)
		if err != nil {
			message := fmt.Sprintf("cannot split response key into composite key parts slice: %s", err.Error())
			logger.Error(message)
			return shim.Error(message)
		}

		if err := entry.FillFromCompositeKeyParts(compositeKeyParts); err != nil {
			message := fmt.Sprintf("cannot fill transfer details key from composite key parts: %s", err.Error())
			logger.Error(message)
			return shim.Error(message)
		}

		if bytes, err := json.Marshal(entry); err == nil {
			logger.Debug("Entry: " + string(bytes))
		}

		entries = append(entries, entry)
	}

	result, err := json.Marshal(entries)
	if err != nil {
		return shim.Error(err.Error())
	}
	logger.Debug("Result: " + string(result))

	logger.Info("OwnershipChaincode.query exited without errors")
	logger.Debug("Success: OwnershipChaincode.query")
	return shim.Success(result)
}

func (t *OwnershipChaincode) history(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	logger.Info("OwnershipChaincode.history is running")
	logger.Debug("OwnershipChaincode.history")

	const expectedArgumentsNumber = 1

	if len(args) < expectedArgumentsNumber {
		message := fmt.Sprintf("insufficient number of arguments: expected %d, got %d",
			expectedArgumentsNumber, len(args))
		logger.Error(message)
		return shim.Error(message)
	}

	queryIterator, err := stub.GetStateByPartialCompositeKey(transferIndex, []string{args[0]})
	if err != nil {
		var message string
		if compositeKey, e := stub.CreateCompositeKey(transferIndex, args[:1]); e == nil {
			message = fmt.Sprintf("unable to get state by partial composite key %s: %s",
				compositeKey, err.Error())
		} else {
			message = fmt.Sprintf("unable to get state by partial composite key: %s", err.Error())
		}
		logger.Error(message)
		return shim.Error(message)
	}
	defer queryIterator.Close()

	entries := []TransferDetails{}
	for queryIterator.HasNext() {
		queryResponse, err := queryIterator.Next()
		if err != nil {
			message := fmt.Sprintf("unable to get an element next to a query iterator: %s", err.Error())
			logger.Error(message)
			return shim.Error(message)
		}

		logger.Debug("Query response key: " + queryResponse.Key)

		historyIterator, err := stub.GetHistoryForKey(queryResponse.Key)
		if err != nil {
			message := fmt.Sprintf("unable to get history for key %s: %s", queryResponse.Key, err.Error())
			logger.Error(message)
			return shim.Error(message)
		}

		for historyIterator.HasNext() {
			historyResponse, err := historyIterator.Next()
			if err != nil {
				message := fmt.Sprintf("unable to get an element next to a history iterator: %s", err.Error())
				logger.Error(message)
				return shim.Error(message)
			}

			entry := TransferDetails{}

			if err := entry.FillFromLedgerValue(historyResponse.Value); err != nil {
				message := fmt.Sprintf("cannot fill transfer details value from response value: %s", err.Error())
				logger.Error(message)
				return shim.Error(message)
			}

			_, compositeKeyParts, err := stub.SplitCompositeKey(queryResponse.Key)
			if err != nil {
				message := fmt.Sprintf("cannot split response key into composite key parts slice: %s",
					err.Error())
				logger.Error(message)
				return shim.Error(message)
			}

			if err := entry.FillFromCompositeKeyParts(compositeKeyParts); err != nil {
				message := fmt.Sprintf("cannot fill transfer details key from composite key parts: %s",
					err.Error())
				logger.Error(message)
				return shim.Error(message)
			}

			if bytes, err := json.Marshal(entry); err == nil {
				logger.Debug("Entry: " + string(bytes))
			}

			entries = append(entries, entry)
		}
		historyIterator.Close()
	}

	result, err := json.Marshal(entries)
	if err != nil {
		return shim.Error(err.Error())
	}
	logger.Debug("Result: " + string(result))

	logger.Info("OwnershipChaincode.history exited without errors")
	logger.Debug("Success: OwnershipChaincode.history")
	return shim.Success(result)
}

func checkProductExistenceAndOwnership(stub shim.ChaincodeStubInterface, productKey, requiredOwner string) error {
	type simplifiedProduct struct {
		Value struct {
			Owner string `json:"owner"`
		} `json:"value"`
	}

	const queryFunctionName = "readProduct"

	response := stub.InvokeChaincode(commonChaincodeName,
		[][]byte{[]byte(queryFunctionName), []byte(productKey)}, commonChannelName)
	if response.Status >= 400 {
		return errors.New(
			fmt.Sprintf("unable to read product %s from common channel: %s", productKey, response.Message))
	} else {
		var p simplifiedProduct
		if err := json.Unmarshal(response.Payload, &p); err != nil {
			return errors.New(
				fmt.Sprintf("unable to unmarshal response on product %s from common channel", productKey))
		}

		if p.Value.Owner != requiredOwner {
			return errors.New(
				fmt.Sprintf("product %s doesn't belong to organization %s", productKey, requiredOwner))
		}
	}

	return nil
}

func getProductsFromLabel(stub shim.ChaincodeStubInterface, label, requestSender, requiredOwner string) ([]string, error) {

	logger.Debug("0 #########################################################################################################################################################################")
	const queryFunctionName = "getProductsByLabel"
	var finalProducts []string

	response := stub.InvokeChaincode(commonChaincodeName,
		[][]byte{[]byte(queryFunctionName), []byte(label)}, commonChannelName)

	logger.Debug("0.1 #########################################################################################################################################################################")

	if response.Status >= 400 {
		return finalProducts, errors.New(
			fmt.Sprintf("unable to read product %s from common channel: %s", label, response.Message))
	} else {
		logger.Debug("1 #########################################################################################################################################################################")
		logger.Debug(response.Payload)
		logger.Debug("1.1 #########################################################################################################################################################################")

		products := []Product{}
		if err := json.Unmarshal(response.Payload, &products); err != nil {
			return finalProducts, errors.New(
				fmt.Sprintf("unable to unmarshal response on product %s from common channel", label))
		}

		for i := 0; i < len(products); i++ {
			Status, Sender := queryDetailsbyProduct(stub, products[i].Key.Name)

			if (products[i].Value.Owner == requiredOwner && Status != statusAccepted) && Sender != requestSender {
				finalProducts = append(finalProducts, products[i].Key.Name)
			}
		}
		logger.Debug("2.1 #########################################################################################################################################################################")
		logger.Debug(products)
		logger.Debug(finalProducts)
		logger.Debug("2.2 #########################################################################################################################################################################")

	}

	return finalProducts, nil
}

func getOrganization(certificate []byte) string {
	data := certificate[strings.Index(string(certificate), "-----") : strings.LastIndex(string(certificate), "-----")+5]
	block, _ := pem.Decode([]byte(data))
	cert, _ := x509.ParseCertificate(block.Bytes)
	organization := cert.Issuer.Organization[0]
	return strings.Split(organization, ".")[0]
}

func GetCreatorOrganization(stub shim.ChaincodeStubInterface) string {
	certificate, _ := stub.GetCreator()
	return getOrganization(certificate)
}

func main() {
	err := shim.Start(new(OwnershipChaincode))
	if err != nil {
		logger.Error(err.Error())
	}
}

func getQueryResultForQueryString(stub shim.ChaincodeStubInterface, queryString string) ([]byte, error) {

	fmt.Printf("- getQueryResultForQueryString queryString:\n%s\n", queryString)

	resultsIterator, err := stub.GetQueryResult(queryString)
	if err != nil {
		return nil, err
	}
	defer resultsIterator.Close()

	entries := []Product{}

	for resultsIterator.HasNext() {
		response, err := resultsIterator.Next()
		if err != nil {
			return nil, err
		}

		entry := Product{}

		if err := json.Unmarshal(response.Value, &entry.Value); err != nil {
			return nil, err
		}

		_, compositeKeyParts, err := stub.SplitCompositeKey(response.Key)
		if err != nil {
			return nil, err
		}

		if err := entry.FillFromCompositeKeyParts(compositeKeyParts); err != nil {
			return nil, err
		}

		entries = append(entries, entry)
	}

	result, err := json.Marshal(entries)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func queryDetailsbyProduct(stub shim.ChaincodeStubInterface, productName string) (string, string) {

	type details struct {
		Status string `json:"status"`
	}

	type TransferDetail struct {
		Status string `json:"status"`
		Sender string `json:"sender"`
	}
	var entries TransferDetail

	it, err := stub.GetStateByPartialCompositeKey(transferIndex, []string{})
	if err != nil {
		return "", ""
	}
	defer it.Close()

	for it.HasNext() {
		response, err := it.Next()
		if err != nil {
			// message := fmt.Sprintf("unable to get an element next to a query iterator: %s", err.Error())
			// logger.Error(message)
			return "unable to get an element next to a query iterator", productName
		}

		logger.Debug(fmt.Sprintf("Response: {%s, %s}", response.Key, string(response.Value)))

		entry := TransferDetails{}

		var p details
		if err := json.Unmarshal(response.Value, &p); err != nil {
			// message := fmt.Sprintf("unable to unmarshal response on product %s from common channel", err.Error())
			// logger.Error(message)
			return "unable to unmarshal response on product %s from common channel", productName
		}

		if err := entry.FillFromLedgerValue(response.Value); err != nil {
			// message := fmt.Sprintf("cannot fill transfer details value from response value: %s", err.Error())
			// logger.Error(message)
			return "cannot fill transfer details value from response value", productName
		}

		_, compositeKeyParts, err := stub.SplitCompositeKey(response.Key)
		if err != nil {
			// message := fmt.Sprintf("cannot split response key into composite key parts slice: %s", err.Error())
			// logger.Error(message)
			return "cannot split response key into composite key parts slice:", productName
		}

		productID := compositeKeyParts[0]

		if err := entry.FillFromCompositeKeyParts(compositeKeyParts); err != nil {
			// message := fmt.Sprintf("cannot fill transfer details key from composite key parts: %s", err.Error())
			// logger.Error(message)
			return "cannot fill transfer details key from composite key parts", productName
		}

		if productID == productName {
			if bytes, err := json.Marshal(entry); err == nil {
				logger.Debug("Entry: " + string(bytes))
			}

			entries.Status = p.Status
			entries.Sender = compositeKeyParts[1]
		}
	}

	result, err := json.Marshal(entries)
	if err != nil {
		return "Unable to marshal data", productName
	}

	logger.Debug("Result: " + string(result))

	logger.Info("OwnershipChaincode.query exited without errors")
	logger.Debug("Success: OwnershipChaincode.query")
	return entries.Status, entries.Sender
}
