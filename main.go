// udpserver.go
package main

import (
	 
	"fmt"
	"net"
	"log"
	"encoding/json"
	"time"
	"net/http"
	"sync"

	"github.com/filipkroca/teltonikaparser"
	"github.com/nsqio/go-nsq"
)

type Server struct {
	Protocol string
	IP       []byte
	Port     int
}

type ExtendedData struct {
	Imei              string `json:"imei"`
	Ignition          int    `json:"ignition"`
	Movement          int    `json:"movement"`
	GsmSignal         int    `json:"gsm_signal"`
	SleepMode         int    `json:"sleep_mode"`
	GnssStatus        int    `json:"gnss_status"`
	DigitalInput      int    `json:"digital_input"`
	BatteryLevel      int    `json:"battery_level"`
	Unplug            int    `json:"unplug"`
	GnssPdop          int    `json:"gnss_pdop"`
	GnssHdop          int    `json:"gnss_hdop"`
	ExternalVoltage   int    `json:"external_voltage"`
	GsmCellID         int    `json:"gsm_cell_id"`
	GsmAreaCode       int    `json:"gsm_area_code"`
	AxisX             int    `json:"axis_x"`
	AxisY             int    `json:"axis_y"`
	AxisZ             int    `json:"axis_z"`
	EcoScore          int    `json:"eco_score"`
	ActiveGsmOperator int    `json:"active_gsm_operator"`
	TotalOdometer     int    `json:"total_odometer"`
}

type JsonData struct {
	IMEI string `json:"imei"`
	Data []Data `json:"data"`
}

type Data struct {
	Utime 	  string `json:"utime"`
	Priority  int    `json:"priority"`
	Lat       float64 `json:"lat"`
	Lng       float64 `json:"lng"`
	Altitude  float64 `json:"altitude"`
	Angle     int `json:"angle"`
	Speed     int `json:"speed"`
	Satellite  int `json:"satellite"`
}

// Define a global HTTP client with a custom Transport
var httpClient = &http.Client{
    Transport: &http.Transport{
        MaxIdleConns:        100,  
        MaxIdleConnsPerHost: 100,  
        IdleConnTimeout:     90 * time.Second, 
    },
}

const (
	MaxWorkers = 100000 
	BufferSize = 8192 
)

var workerPool = make(chan struct{}, MaxWorkers) // Worker pool to limit concurrent goroutines

var nsqProducer *nsq.Producer

func (t *Server) New(callBack func(udpc *net.UDPConn, buf *[]byte, len int, addr *net.UDPAddr)) {

	udpc, err := net.ListenUDP(t.Protocol, &net.UDPAddr{IP: t.IP, Port: t.Port, Zone: ""})
	if err != nil {
		log.Println("[ERROR] Error when starting udp server:", err)
		return
	}
	defer udpc.Close()

	log.Printf("[INFO] Listening on %v\n", udpc.LocalAddr())

	for {
		// Make a buffer
		buf := make([]byte, BufferSize)
		n, addr, err := udpc.ReadFromUDP(buf)
		if err != nil {
			log.Println("[ERROR] Error when listening:", err)
			continue 
		}

		// Slice data
		sliced := buf[:n]

		log.Printf("[INFO] New connection from %v \n", addr)

		select {
		case workerPool <- struct{}{}:
			go func() {
				defer func() { <-workerPool }()
				callBack(udpc, &sliced, n, addr)
			}()
		default:
			log.Printf("[WARNING] Worker pool is full, waiting for a worker to become available")
			time.Sleep(time.Millisecond * 100) // Wait for a worker to become available
		}
	}
}

func main() {
	err := InitializeNSQProducer()
    if err != nil {
        log.Fatalf("[INFO] Failed to initialize NSQ producer: %v", err)
    }

	server := Server{
		Protocol: "udp",
		IP:       []byte{0, 0, 0, 0},
		Port:     8800,
	}
	// Create new server
	server.New(onUDPMessage)
	defer fmt.Println("Server closed")
	
}

// onUDPMessage is invoked when a packet arrives
func onUDPMessage(udpc *net.UDPConn, dataBs *[]byte, len int, addr *net.UDPAddr) {
	var wg sync.WaitGroup
    wg.Add(1)

	x, err := teltonikaparser.Decode(dataBs)
	if err != nil {
		log.Printf("[ERROR] Unable to decode packet: %v", err)
	}

	go func() {
		defer wg.Done()
		humanDecoder := teltonikaparser.HumanDecoder{}
		decodedValues := make(map[string]interface{})

		for _, val := range x.Data {
			for _, ioel := range val.Elements {
				decoded, err := humanDecoder.Human(&ioel, "FMBXY") 
				if err != nil {
					log.Printf("[Error] Error when converting human, %v\n", err)
					continue
				}
	
				if val, err := (*decoded).GetFinalValue(); err != nil {
					log.Printf("[Error] Unable to GetFinalValue() %v", err)
					continue
				} else if val != nil {
					decodedValues[fmt.Sprint(decoded.AvlEncodeKey.PropertyName)] = val
				}
			}

		}
		
		extendedData := ExtendedData{
			Imei:              x.IMEI,
			Ignition:          getIntValue(decodedValues, "Ignition"),
			Movement:          getIntValue(decodedValues, "Movement"),
			GsmSignal:         getIntValue(decodedValues, "GSM Signal"),
			SleepMode:         getIntValue(decodedValues, "Sleep Mode"),
			GnssStatus:        getIntValue(decodedValues, "GNSS Status"),
			DigitalInput:      getIntValue(decodedValues, "Digital Input 1"),
			BatteryLevel:      getIntValue(decodedValues, "Battery Level"),
			Unplug:            getIntValue(decodedValues, "Unplug"),
			GnssPdop:          getIntValue(decodedValues, "GNSS PDOP"),
			GnssHdop:          getIntValue(decodedValues, "GNSS HDOP"),
			ExternalVoltage:   getIntValue(decodedValues, "External Voltage"),
			GsmCellID:         getIntValue(decodedValues, "GSM Cell ID"),
			GsmAreaCode:       getIntValue(decodedValues, "GSM Area Code"),
			AxisX:             getIntValue(decodedValues, "Axis X"),
			AxisY:             getIntValue(decodedValues, "Axis Y"),
			AxisZ:             getIntValue(decodedValues, "Axis Z"),
			EcoScore:          getIntValue(decodedValues, "Eco Score"),
			ActiveGsmOperator: getIntValue(decodedValues, "Active GSM Operator"),
			TotalOdometer:     getIntValue(decodedValues, "Total Odometer"),
		}

		jsonData, err := json.MarshalIndent(extendedData, "", "    ")
			if err != nil {
				log.Printf("[ERROR] Error marshaling to JSON: %v", err)
			} 
		
			log.Printf("[INFO] Data sending to a consumer with extended_data_topic ");

			err = PublishDataToNSQ("extended_data_topic", []byte(jsonData))
			if err != nil {
				log.Printf("[ERROR] Failed to publish data to NSQ: %v", err)
			}
	}()
	

	var dataSlice []Data

	for _, data := range x.Data {
		if data.Lat != 0 || data.Lng != 0 {
				lat := float64(data.Lat) / 10000000.0
				lng := float64(data.Lng) / 10000000.0
				utime := time.Unix(int64(data.Utime), 0).Format("2006-01-02 15:04:05")

			dataSlice = append(dataSlice, Data{
				Utime: 		fmt.Sprint(utime),
				Priority:  int(data.Priority),
				Lat:       lat,
				Lng:       lng,
				Altitude:  float64(data.Altitude),
				Angle:     int(data.Angle),
				Speed:     int(data.Speed),
				Satellite: int(data.VisSat),
			})
		}}

		basicTable := JsonData{
			IMEI: x.IMEI,
			Data: dataSlice,
		}
	
		jsonString, err := json.MarshalIndent(basicTable, "", "    ")
		if err != nil {
			log.Printf("[ERROR]  Error when marshaling to JSON: %v", err );
		}
		log.Printf("[INFO] Data sending to a consumer with basic_data_topic");

		err = PublishDataToNSQ("basic_data_topic", []byte(jsonString))
		if err != nil {
			log.Printf("[ERROR] Failed to publish data to NSQ: %v", err)
		}



	wg.Wait()
	// Respond back to the client
	(*udpc).WriteToUDP([]byte("mission control "), addr)
}


func getIntValue(decodedValues map[string]interface{}, propertyName string) int {
	if val, ok := decodedValues[propertyName]; ok {
		switch v := val.(type) {
		case float64:
			return int(v)
		case uint8:
			return int(v)
		case bool:
			if v {
				return 1
			} else {
				return 0
			}
		case uint16:
			return int(v)
		case int16:
			return int(v)
		case uint32:
			return int(v)
		default:
			log.Printf("Unexpected type for property %s: %T", propertyName, v)
		}
	}
	return 0 // Default value if property not found
}

// InitializeNSQProducer initializes the NSQ producer.
func InitializeNSQProducer() error {
    var err error
    nsqProducer, err = nsq.NewProducer("127.0.0.1:4150", nsq.NewConfig())
    if err != nil {
        return err
    }
    return nil
}

// PublishDataToNSQ publishes data to the specified NSQ topic.
func PublishDataToNSQ(topic string, data []byte) error {
    return nsqProducer.Publish(topic, data)
}

