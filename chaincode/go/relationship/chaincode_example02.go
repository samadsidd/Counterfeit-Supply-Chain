package main

import (
	"bytes"
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
	} else if function == "sendRelationshipRequest" {
		return t.sendRelationshipRequest(stub, args)
	} else if function == "queryRelationshipRequestsByReciever" {
		return t.queryRelationshipRequestsByReciever(stub, args)
	} else if function == "acceptRelationshipRequest" {
		return t.acceptRelationshipRequest(stub, args)
	} else if function == "queryRelationshipRequestsBySenderAndAccepted" {
		return t.queryRelationshipRequestsBySenderAndAccepted(stub, args)
	} else if function == "sendProductRequestToManufacturer" {
		return t.sendProductRequestToManufacturer(stub, args)
	} else if function == "queryProductRequestsByReciever" {
		return t.queryProductRequestsByReciever(stub, args)
	} else if function == "acceptProductRequestforManufacturer" {
		return t.acceptProductRequestforManufacturer(stub, args)
	} else if function == "sendRequestBulkManufacturer" {
		return t.sendRequestBulkManufacturer(stub, args)
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

	// if GetCreatorOrganization(stub) != request.Key.RequestSender {
	// 	message := fmt.Sprintf(
	// 		"no privileges to send request from the side of organization %s (caller is from organization %s)",
	// 		request.Key.RequestSender, GetCreatorOrganization(stub))
	// 	logger.Error(message)
	// 	return pb.Response{Status: 403, Message: message}
	// }

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
	request.Value.OwnerShipProof = args[4]

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
	availableProducts, _ := getProductsFromName(stub, label, args[1], args[2])
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

		// if GetCreatorOrganization(stub) != request.Key.RequestSender {
		// 	message := fmt.Sprintf(
		// 		"no privileges to send request from the side of organization %s (caller is from organization %s)",
		// 		request.Key.RequestSender, GetCreatorOrganization(stub))
		// 	logger.Error(message)
		// 	return pb.Response{Status: 403, Message: message}
		// }

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
		request.Value.OwnerShipProof = args[5]
		request.Key.ProductKey = availableProducts[i]
		request.Key.RequestReceiver = args[2]
		request.Key.RequestSender = args[1]

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

func (t *OwnershipChaincode) sendRequestBulkManufacturer(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	logger.Info("OwnershipChaincode.sendRequestBulk is running")
	logger.Debug("OwnershipChaincode.sendRequestBulk")

	const expectedArgumentsNumber = 5

	if len(args) < expectedArgumentsNumber {
		message := fmt.Sprintf("insufficient number of arguments: expected %d, got %d",
			expectedArgumentsNumber, len(args))
		logger.Error(message)
		return shim.Error(message)
	}

	type Product struct {
		Key   ProductKey   `json:"key"`
		Value ProductValue `json:"value"`
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
	availableProducts, _ := getProductsFromNameManufacturer(stub, label, args[1], args[2])
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

	var productRequest ProductRequestToManufacturerDetails

	// Iterate
	for i := 0; i < requiredQuantity; i++ {

		key := args[1] + "-" + args[2] + "-" + availableProducts[i]

		logger.Debug("prodgad==============================")

		logger.Debug(key)

		requestAsByte, err := stub.GetState(key)
		if err != nil {
			return shim.Error("Failed to get request: " + err.Error())
		}

		if requestAsByte != nil {
			err2 := json.Unmarshal(requestAsByte, &productRequest)
			if err2 != nil {
				return shim.Error(fmt.Sprintf("unable to unmarshal request"))
			}

			logger.Debug("RequestSender: " + productRequest.RequestSender)

			if productRequest.Status == statusInitiated {
				message := "ownership transfer is already initiated"
				logger.Error(message)
				return shim.Error(message)
			}

			if err := checkProductExistenceAndOwnership(stub, productRequest.ProductKey, productRequest.RequestReceiver); err != nil {
				message := err.Error()
				logger.Error(message)
				return shim.Error(message)
			}
		}

		productRequest.Status = statusInitiated
		productRequest.Message = args[3]
		productRequest.Timestamp = time.Now().UTC().Unix()
		productRequest.OwnerShipProof = args[5]
		productRequest.ObjectType = "ProductRequests"
		productRequest.ProductKey = availableProducts[i]
		productRequest.RequestSender = args[1]
		productRequest.RequestReceiver = args[2]

		result, err := json.Marshal(productRequest)
		if err != nil {
			return shim.Error(err.Error())
		}

		err = stub.PutState(key, result)
		if err != nil {
			return shim.Error(err.Error())
		}

		logger.Info("OwnershipChaincode.sendRequestBulk exited without errors")
		logger.Debug("Success: OwnershipChaincode.sendRequestBulk")

	}

	return shim.Success(nil)
}

func (t *OwnershipChaincode) acceptProductRequestforManufacturer(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	if len(args) < 1 {
		return shim.Error("Incorrect number of arguments. Expecting 1")
	}

	var ProductRequestToManufacturerDetails ProductRequestToManufacturerDetails

	key := args[0]

	requestAsByte, err := stub.GetState(key)
	if err != nil {
		return shim.Error("Failed to get request: " + err.Error())
	}

	err2 := json.Unmarshal(requestAsByte, &ProductRequestToManufacturerDetails)
	if err2 != nil {
		return shim.Error(fmt.Sprintf("unable to unmarshal request"))
	}
	ProductRequestToManufacturerDetails.Status = "Accepted"
	ProductRequestToManufacturerDetails.Timestamp = time.Now().UTC().Unix()

	result, err := json.Marshal(ProductRequestToManufacturerDetails)
	if err != nil {
		return shim.Error(err.Error())
	}

	err = stub.PutState(key, result)
	if err != nil {
		return shim.Error(err.Error())
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

	// if GetCreatorOrganization(stub) != details.Key.RequestReceiver {
	// 	message := fmt.Sprintf(
	// 		"no privileges to accept transfer from the side of organization %s (caller is from organization %s)",
	// 		details.Key.RequestReceiver, GetCreatorOrganization(stub))
	// 	logger.Error(message)
	// 	return pb.Response{Status: 403, Message: message}
	// }

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

func getProductsFromName(stub shim.ChaincodeStubInterface, label, requestSender, requiredOwner string) ([]string, error) {

	logger.Debug("0 #########################################################################################################################################################################")
	const queryFunctionName = "queryProductsByName"
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
			logger.Debug("============status and sender===============")
			logger.Debug(Status, Sender)

			if (products[i].Value.Owner == requiredOwner && Status != statusAccepted) && Sender != requestSender {
				finalProducts = append(finalProducts, products[i].Key.Name)
			} else if Status == "" && Sender == "" {
				logger.Debug("=============yes====================")
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

func getProductsFromNameManufacturer(stub shim.ChaincodeStubInterface, label, requestSender, requiredOwner string) ([]string, error) {

	logger.Debug("0 #########################################################################################################################################################################")
	const queryFunctionName = "queryProductsByName"
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
			Status, Sender := queryDetailsbyProductManufacturer(stub, products[i].Key.Name, requestSender, requiredOwner)
			logger.Debug("============status and sender===============")
			logger.Debug(Status, Sender)

			if (products[i].Value.Owner == requiredOwner && Status != statusAccepted) && Sender != requestSender {
				finalProducts = append(finalProducts, products[i].Key.Name)
			} else if Status == "" && Sender == "" {
				logger.Debug("=============yes====================")
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
	logger.Debug("cert =================================")
	logger.Debug(cert)
	organization := cert.Issuer.Organization[0]
	return strings.Split(organization, ".")[0]
}

func GetCreatorOrganization(stub shim.ChaincodeStubInterface) string {
	certificate, _ := stub.GetCreator()

	return getOrganization(certificate)
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

func (t *OwnershipChaincode) sendProductRequestToManufacturer(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	logger.Info("OwnershipChaincode.sendProductRequestToManufacturer is running")
	logger.Debug("OwnershipChaincode.sendProductRequestToManufacturer")

	if len(args) < 3 {
		message := fmt.Sprintf("insufficient number of arguments: expected %d, got %d",
			3, len(args))
		logger.Error(message)
		return shim.Error(message)
	}

	objectType := "ProductRequests"
	sender := args[1]
	receiver := args[2]
	description := strings.ToLower(args[3])
	productKey := args[0]
	Timestamp := time.Now().UTC().Unix()
	key := args[1] + "-" + args[2] + "-" + args[0]
	Status := statusInitiated
	OwnerShipProof := args[4]

	requestAsByte, err := stub.GetState(key)
	if err != nil {
		return shim.Error("Failed to get request: " + err.Error())
	} else if requestAsByte != nil {
		fmt.Println("This request already exists: " + args[1])
		return shim.Error("This request already exists: " + args[1])
	}

	logger.Debug(key)

	ProductDetails := &ProductRequestToManufacturerDetails{objectType, sender, receiver, description, productKey, Timestamp, Status, OwnerShipProof}
	userAsBytes, err := json.Marshal(ProductDetails)
	if err != nil {
		return shim.Error(err.Error())
	}

	logger.Debug("Success:" + string(userAsBytes))

	err = stub.PutState(key, userAsBytes)
	if err != nil {
		return shim.Error(err.Error())
	}
	return shim.Success(nil)
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

func queryDetailsbyProductManufacturer(stub shim.ChaincodeStubInterface, productName, sender, receiver string) (string, string) {

	var entry ProductRequestToManufacturerDetails

	key := sender + "-" + receiver + "-" + productName

	requestAsByte, err := stub.GetState(key)
	if err != nil {
		return "failed to get state", ""
	}

	err2 := json.Unmarshal(requestAsByte, &entry)
	if err2 != nil {
		return "unable to unmarshal request", ""
	}

	logger.Debug("==============dataas==========")
	logger.Debug(entry)

	// for it.HasNext() {
	// 	response, err := it.Next()
	// 	if err != nil {
	// 		// message := fmt.Sprintf("unable to get an element next to a query iterator: %s", err.Error())
	// 		// logger.Error(message)
	// 		return "unable to get an element next to a query iterator", productName
	// 	}

	// 	logger.Debug(fmt.Sprintf("Response: {%s, %s}", response.OwnerShipProof, response.Status))

	// 	entry := TransferDetails{}

	// 	var p details
	// 	if err := json.Unmarshal(response.Value, &p); err != nil {
	// 		// message := fmt.Sprintf("unable to unmarshal response on product %s from common channel", err.Error())
	// 		// logger.Error(message)
	// 		return "unable to unmarshal response on product %s from common channel", productName
	// 	}

	// 	if err := entry.FillFromLedgerValue(response.Value); err != nil {
	// 		// message := fmt.Sprintf("cannot fill transfer details value from response value: %s", err.Error())
	// 		// logger.Error(message)
	// 		return "cannot fill transfer details value from response value", productName
	// 	}

	// 	_, compositeKeyParts, err := stub.SplitCompositeKey(response.Key)
	// 	if err != nil {
	// 		// message := fmt.Sprintf("cannot split response key into composite key parts slice: %s", err.Error())
	// 		// logger.Error(message)
	// 		return "cannot split response key into composite key parts slice:", productName
	// 	}

	// 	productID := compositeKeyParts[0]

	// 	if err := entry.FillFromCompositeKeyParts(compositeKeyParts); err != nil {
	// 		// message := fmt.Sprintf("cannot fill transfer details key from composite key parts: %s", err.Error())
	// 		// logger.Error(message)
	// 		return "cannot fill transfer details key from composite key parts", productName
	// 	}

	// 	if productID == productName {
	// 		if bytes, err := json.Marshal(entry); err == nil {
	// 			logger.Debug("Entry: " + string(bytes))
	// 		}

	// 		entries.Status = p.Status
	// 		entries.Sender = compositeKeyParts[1]
	// 	}
	// }

	// result, err := json.Marshal(entries)
	// if err != nil {
	// 	return "Unable to marshal data", productName
	// }

	// logger.Debug("Result: " + string(result))

	logger.Info("OwnershipChaincode.query exited without errors")
	logger.Debug("Success: OwnershipChaincode.query")
	return entry.Status, entry.RequestSender
}

func (t *OwnershipChaincode) sendRelationshipRequest(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	logger.Info("OwnershipChaincode.sendRelationshipRequest is running")
	logger.Debug("OwnershipChaincode.sendRelationshipRequest")

	if len(args) < 3 {
		message := fmt.Sprintf("insufficient number of arguments: expected %d, got %d",
			3, len(args))
		logger.Error(message)
		return shim.Error(message)
	}

	objectType := "relationshipRequests"
	sender := args[1]
	receiver := args[0]
	description := strings.ToLower(args[2])
	status := "Requested"
	key := args[0] + "-" + args[1]

	requestAsByte, err := stub.GetState(key)
	if err != nil {
		return shim.Error("Failed to get request: " + err.Error())
	} else if requestAsByte != nil {
		fmt.Println("This request already exists: " + args[1])
		return shim.Error("This request already exists: " + args[1])
	}

	logger.Debug(key)

	relationshipDetails := &RelationshipRequestDetails{objectType, sender, receiver, description, status}
	userAsBytes, err := json.Marshal(relationshipDetails)
	if err != nil {
		return shim.Error(err.Error())
	}

	logger.Debug("Success:" + string(userAsBytes))

	err = stub.PutState(key, userAsBytes)
	if err != nil {
		return shim.Error(err.Error())
	}
	return shim.Success(nil)
}

func (t *OwnershipChaincode) acceptRelationshipRequest(stub shim.ChaincodeStubInterface, args []string) pb.Response {

	if len(args) < 1 {
		return shim.Error("Incorrect number of arguments. Expecting 1")
	}

	var relationshipRequestDetails RelationshipRequestDetails

	key := args[0]

	requestAsByte, err := stub.GetState(key)
	if err != nil {
		return shim.Error("Failed to get request: " + err.Error())
	}

	err2 := json.Unmarshal(requestAsByte, &relationshipRequestDetails)
	if err2 != nil {
		return shim.Error(fmt.Sprintf("unable to unmarshal request"))
	}
	relationshipRequestDetails.Status = "Accepted"

	result, err := json.Marshal(relationshipRequestDetails)
	if err != nil {
		return shim.Error(err.Error())
	}

	err = stub.PutState(key, result)
	if err != nil {
		return shim.Error(err.Error())
	}

	return shim.Success(nil)
}

func (t *OwnershipChaincode) queryRelationshipRequestsByReciever(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	if len(args) < 1 {
		return shim.Error("Incorrect number of arguments. Expecting 1")
	}

	receiver := strings.ToLower(args[0])

	queryString := fmt.Sprintf("{\"selector\":{\"docType\":\"relationshipRequests\",\"requestReceiver\":\"%s\"}}", receiver)

	queryResults, err := getQueryResultForQueryStringForRelationshipRequests(stub, queryString)
	if err != nil {
		return shim.Error(err.Error())
	}
	return shim.Success(queryResults)
}

func (t *OwnershipChaincode) queryRelationshipRequestsBySenderAndAccepted(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	if len(args) < 1 {
		return shim.Error("Incorrect number of arguments. Expecting 1")
	}

	sender := strings.ToLower(args[0])

	queryString := fmt.Sprintf("{\"selector\":{\"docType\":\"relationshipRequests\",\"requestSender\":\"%s\",\"status\":\"%s\"}}", sender, "Accepted")

	queryResults, err := getQueryResultForQueryStringForRelationshipRequests(stub, queryString)
	if err != nil {
		return shim.Error(err.Error())
	}

	return shim.Success(queryResults)

}

func (t *OwnershipChaincode) queryProductRequestsByReciever(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	if len(args) < 1 {
		return shim.Error("Incorrect number of arguments. Expecting 1")
	}

	receiver := strings.ToLower(args[0])

	queryString := fmt.Sprintf("{\"selector\":{\"docType\":\"ProductRequests\",\"requestReceiver\":\"%s\"}}", receiver)

	queryResults, err := getQueryResultForQueryStringForRelationshipRequests(stub, queryString)
	if err != nil {
		return shim.Error(err.Error())
	}
	return shim.Success(queryResults)
}

func getQueryResultForQueryStringForRelationshipRequests(stub shim.ChaincodeStubInterface, queryString string) ([]byte, error) {

	fmt.Printf("- getQueryResultForQueryString queryString:\n%s\n", queryString)

	resultsIterator, err := stub.GetQueryResult(queryString)
	if err != nil {
		return nil, err
	}
	defer resultsIterator.Close()

	buffer, err := constructQueryResponseFromIterator(resultsIterator)
	if err != nil {
		return nil, err
	}

	fmt.Printf("- getQueryResultForQueryString queryResult:\n%s\n", buffer.String())

	return buffer.Bytes(), nil
}

// ===========================================================================================
// constructQueryResponseFromIterator constructs a JSON array containing query results from
// a given result iterator
// ===========================================================================================
func constructQueryResponseFromIterator(resultsIterator shim.StateQueryIteratorInterface) (*bytes.Buffer, error) {
	// buffer is a JSON array containing QueryResults
	var buffer bytes.Buffer
	buffer.WriteString("[")

	bArrayMemberAlreadyWritten := false
	for resultsIterator.HasNext() {
		queryResponse, err := resultsIterator.Next()
		if err != nil {
			return nil, err
		}
		// Add a comma before array members, suppress it for the first array member
		if bArrayMemberAlreadyWritten == true {
			buffer.WriteString(",")
		}
		buffer.WriteString("{\"Key\":")
		buffer.WriteString("\"")
		buffer.WriteString(queryResponse.Key)
		buffer.WriteString("\"")

		buffer.WriteString(", \"Record\":")
		// Record is a JSON object, so we write as-is
		buffer.WriteString(string(queryResponse.Value))
		buffer.WriteString("}")
		bArrayMemberAlreadyWritten = true
	}
	buffer.WriteString("]")

	return &buffer, nil
}

func main() {
	err := shim.Start(new(OwnershipChaincode))
	if err != nil {
		logger.Error(err.Error())
	}
}
