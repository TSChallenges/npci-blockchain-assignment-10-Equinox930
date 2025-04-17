package main

import (
	"encoding/json"
	"fmt"
	"time"
	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)

type Drug struct {
	DrugID          string   `json:"drugId"`
	Name            string   `json:"name"`
	Manufacturer    string   `json:"manufacturer"`
	BatchNumber     string   `json:"batchNumber"`
	MfgDate         string   `json:"mfgDate"`
	ExpiryDate      string   `json:"expiryDate"`
	Composition     string   `json:"composition"`
	CurrentOwner    string   `json:"currentOwner"` // Cipla, Medlife, Apollo
	Status          string   `json:"status"`       // InProduction, InTransit, Delivered, Recalled
	History         []string `json:"history"`      // Format: "timestamp|event|from|to|details"
	IsRecalled      bool     `json:"isRecalled"`
	InspectionNotes []string `json:"inspectionNotes"`
}

type SmartContract struct {
	contractapi.Contract
}

func getMSPID(ctx contractapi.TransactionContextInterface) (string, error) {
	return ctx.GetClientIdentity().GetMSPID()
}

func getTimestamp() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

// ============== MANUFACTURER FUNCTIONS ==============
func (s *SmartContract) RegisterDrug(ctx contractapi.TransactionContextInterface,
	drugID string, name string, batchNumber string, mfgDate string, expiryDate string, composition string) error {

	mspID, err := getMSPID(ctx)
	if err != nil || mspID != "CiplaMSP" {
		return fmt.Errorf("only Cipla (Manufacturer) can register drugs")
	}

	existing, err := ctx.GetStub().GetState(drugID)
	if err != nil {
		return err
	}
	if existing != nil {
		return fmt.Errorf("drug with ID %s already exists", drugID)
	}

	drug := Drug{
		DrugID:       drugID,
		Name:         name,
		Manufacturer: "Cipla",
		BatchNumber:  batchNumber,
		MfgDate:      mfgDate,
		ExpiryDate:   expiryDate,
		Composition:  composition,
		CurrentOwner: "Cipla",
		Status:       "InProduction",
		IsRecalled:   false,
		History: []string{
			fmt.Sprintf("%s|Created|Cipla|-|Batch: %s", getTimestamp(), batchNumber),
		},
	}

	bytes, _ := json.Marshal(drug)
	return ctx.GetStub().PutState(drugID, bytes)
}

// ============== DISTRIBUTION FUNCTIONS ==============
func (s *SmartContract) ShipDrug(ctx contractapi.TransactionContextInterface, drugID string, to string) error {
	drugBytes, err := ctx.GetStub().GetState(drugID)
	if err != nil || drugBytes == nil {
		return fmt.Errorf("drug %s not found", drugID)
	}

	var drug Drug
	if err := json.Unmarshal(drugBytes, &drug); err != nil {
		return err
	}

	mspID, _ := getMSPID(ctx)
	if drug.CurrentOwner != mspID[:len(mspID)-3] { // e.g., "Cipla" from "CiplaMSP"
		return fmt.Errorf("only the current owner can ship this drug")
	}

	from := drug.CurrentOwner
	drug.CurrentOwner = to
	drug.Status = "InTransit"
	drug.History = append(drug.History, fmt.Sprintf("%s|Shipped|%s|%s|", getTimestamp(), from, to))

	// Emit an event (optional)
	ctx.GetStub().SetEvent("DrugShipped", []byte(drugID))

	bytes, _ := json.Marshal(drug)
	return ctx.GetStub().PutState(drugID, bytes)
}

// ============== REGULATOR FUNCTIONS ==============
func (s *SmartContract) RecallDrug(ctx contractapi.TransactionContextInterface, drugID string, reason string) error {
	mspID, err := getMSPID(ctx)
	if err != nil || mspID != "CDSCOMSP" {
		return fmt.Errorf("only CDSCO (Regulator) can recall drugs")
	}

	drugBytes, err := ctx.GetStub().GetState(drugID)
	if err != nil || drugBytes == nil {
		return fmt.Errorf("drug %s not found", drugID)
	}

	var drug Drug
	_ = json.Unmarshal(drugBytes, &drug)

	drug.IsRecalled = true
	drug.Status = "Recalled"
	drug.InspectionNotes = append(drug.InspectionNotes, fmt.Sprintf("%s: %s", getTimestamp(), reason))
	drug.History = append(drug.History, fmt.Sprintf("%s|Recalled|CDSCO|-|Reason: %s", getTimestamp(), reason))

	// Emit recall event
	ctx.GetStub().SetEvent("DrugRecalled", []byte(drugID))

	bytes, _ := json.Marshal(drug)
	return ctx.GetStub().PutState(drugID, bytes)
}

// ============== COMMON FUNCTIONS ==============
func (s *SmartContract) TrackDrug(ctx contractapi.TransactionContextInterface, drugID string) (string, error) {
	data, err := ctx.GetStub().GetState(drugID)
	if err != nil || data == nil {
		return "", fmt.Errorf("drug %s not found", drugID)
	}
	return string(data), nil
}

// ============== MAIN ==============
func main() {
	chaincode, err := contractapi.NewChaincode(&SmartContract{})
	if err != nil {
		fmt.Printf("Error creating chaincode: %s", err.Error())
		return
	}
	if err := chaincode.Start(); err != nil {
		fmt.Printf("Error starting chaincode: %s", err.Error())
	}
}
