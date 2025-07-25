package consumer

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"math"
	"oracle/model"

	"github.com/segmentio/kafka-go"
)

func fetchExpectedIrradiance(lat, lon float64, timestamp string) float64 {
	return 900.0 // NASA API 연동 가능
}

func HandleMessage(msg []byte) {
	var data model.SolarData
	if err := json.Unmarshal(msg, &data); err != nil {
		log.Printf("❌ JSON decode error: %v\n", err)
		return
	}

	expected := fetchExpectedIrradiance(data.Latitude, data.Longitude, data.Timestamp)
	diff := math.Abs(expected - data.Irradiance)

	log.Printf("🌞 Device=%s, Reported=%.1f, Expected=%.1f, Δ=%.1f",
		data.DeviceID, data.Irradiance, expected, diff)

	if diff <= 150 {
		log.Println("✅ Valid irradiance")
	} else {
		log.Println("⚠️ Suspicious data")
	}
}

type MappingRequest struct {
	DeviceID string `json:"device_id"` // inverter_id 의미
}

type MappingResponse struct {
	InverterID string `json:"inverter_id"`
	Address    string `json:"address"`
}

// DB에서 address 조회
func LookupAddressFromDB(db *sql.DB, inverterID string) string {
	var address string
	err := db.QueryRow("SELECT address FROM userData WHERE inverter_id = $1", inverterID).Scan(&address)
	if err != nil {
		log.Printf("[Mapping] DB query error for inverter_id=%s: %v", inverterID, err)
		return ""
	}
	return address
}

func HandleMappingRequest(msg []byte, db *sql.DB, writer *kafka.Writer) {
	var req MappingRequest
	if err := json.Unmarshal(msg, &req); err != nil {
		log.Printf("[Mapping] JSON decode error: %v\n", err)
		return
	}

	address := LookupAddressFromDB(db, req.DeviceID)
	if address == "" {
		log.Printf("[Mapping] No address found for inverter_id=%s", req.DeviceID)
		return
	}

	resp := MappingResponse{
		InverterID: req.DeviceID,
		Address:    address,
	}

	respBytes, _ := json.Marshal(resp)

	err := writer.WriteMessages(context.Background(), kafka.Message{
		Key:   []byte(req.DeviceID),
		Value: respBytes,
	})
	if err != nil {
		log.Printf("❌ Kafka publish error: %v\n", err)
		return
	}

	log.Printf("[Mapping] inverter_id=%s → address=%s", req.DeviceID, address)
}
