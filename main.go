package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/tarm/serial"
)

var currVel float64 = 0
var currAngleReal float64 = 0
var currAngleMag float64 = 0
var traces chan string
var allData chan string
var pointData chan string
var pointContext chan string

type Capture struct {
	UnixTimestamp int64
	UtcTime       string
	NLatitude     float64
	ELongitude    float64
	Velocity      float64
	GPSStatus     int
	Height        float64
	AngleReal     float64
	AngleMag      float64
}

func main() {
	go recordTheData()
	pointContext = make(chan string, 32)
	for {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Enter Stake Num: ")
		text, _ := reader.ReadString('\n')
		text = strings.TrimSuffix(text, "\n")
		fmt.Println("Entered stake Num: " + text)
		pointContext <- text
	}
}

func recordTheData() {

	dt := time.Now().Format("2006-01-02-15_04_05")
	traceFile, err := os.Create("trace_" + dt + ".log")
	check(err)
	defer traceFile.Close()

	allDataFile, err := os.Create("allData_" + dt + ".log")
	check(err)
	defer allDataFile.Close()

	pointDataFile, err := os.Create("pointData_" + dt + ".log")
	check(err)
	defer pointDataFile.Close()

	traces = make(chan string, 64)
	go writeTrace(traces, traceFile)

	allData = make(chan string, 64)
	go writeData(allData, allDataFile)

	pointData = make(chan string, 64)
	go writePoint(pointDataFile)

	config := &serial.Config{
		Name:        "COM3",
		Baud:        115200,
		ReadTimeout: 1,
		Size:        8,
	}

	stream, err := serial.OpenPort(config)
	if err != nil {
		log.Fatal(err)
	}

	scanner := bufio.NewScanner(stream)
	for scanner.Scan() {
		//fmt.Println(scanner.Text())
		traces <- scanner.Text()
		allData <- scanner.Text()
		pointData <- scanner.Text()
	}

}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func addGPSDataToVec(stringItem string) (captureItem Capture, ok bool) {
	dataItem := strings.Split(stringItem, ",")
	if len(dataItem[0]) == 0 {
		ok = false
		return
	}
	if strings.ContainsAny(dataItem[0], "$") {
		if strings.ContainsAny(dataItem[0], "$GPGGA") && len(dataItem) == 15 {
			captureItem.UtcTime = dataItem[1]
			captureItem.NLatitude, _ = strconv.ParseFloat(dataItem[2], 64)
			captureItem.ELongitude, _ = strconv.ParseFloat(dataItem[4], 64)
			captureItem.GPSStatus, _ = strconv.Atoi(dataItem[6])
			captureItem.Height, _ = strconv.ParseFloat(dataItem[9], 64)

			/* utc time to unix minisecond time */
			timeNow := time.Now()
			utcH, _ := strconv.Atoi(captureItem.UtcTime[0:2])
			utcM, _ := strconv.Atoi(captureItem.UtcTime[2:4])
			utcSS, _ := strconv.ParseFloat(captureItem.UtcTime[4:], 64)
			utcSInt := int(utcSS)
			utcSS = utcSS - float64(utcSInt)
			utcSSInt := int(utcSS * 1000000000)
			//timeData := time.Date(timeNow.Year(), timeNow.Month(), timeNow.Day(), utcH, utcM, utcSInt, utcSSInt, time.UTC)
			timeData := time.Date(timeNow.Year(), 9, 27, utcH, utcM, utcSInt, utcSSInt, time.UTC)
			captureItem.UnixTimestamp = timeData.UnixNano() / (int64(time.Millisecond) / int64(time.Nanosecond))

			/* longitude and latitude */
			value := captureItem.NLatitude
			captureItem.NLatitude = float64(int(value/100.0)) + (value-float64(int(value/100.0))*100.0)/60.0
			value = captureItem.ELongitude
			captureItem.ELongitude = float64(int(value/100.0)) + (value-float64(int(value/100.0))*100.0)/60.0
			ok = true
		}

		if strings.ContainsAny(dataItem[0], "VTG") && len(dataItem) == 10 {
			currVel, _ = strconv.ParseFloat(dataItem[7], 64)
			currAngleReal, _ = strconv.ParseFloat(dataItem[1], 64)
			currAngleMag, _ = strconv.ParseFloat(dataItem[3], 64)
		}
		captureItem.Velocity = currVel
		captureItem.AngleReal = currAngleReal
		captureItem.AngleMag = currAngleMag
	}
	return
}

func writeTrace(c chan string, traceFile *os.File) {
	for s := range c {
		item, ok := addGPSDataToVec(s)
		if ok {
			var recordItem []string
			recordItem = append(recordItem, fmt.Sprintf("%.10g", item.NLatitude))
			recordItem = append(recordItem, fmt.Sprintf("%.10g", item.ELongitude))
			recordItem = append(recordItem, fmt.Sprintf("%f", item.Velocity))
			traceFile.WriteString(strings.Join(recordItem, ",") + "\n")
		}
	}
}

func writeData(c chan string, allDataFile *os.File) {
	for s := range c {
		item, ok := addGPSDataToVec(s)
		if ok {
			var recordItem []string
			recordItem = append(recordItem, fmt.Sprintf("%d", item.UnixTimestamp))
			recordItem = append(recordItem, item.UtcTime)
			recordItem = append(recordItem, fmt.Sprintf("%.10g", item.NLatitude))
			recordItem = append(recordItem, fmt.Sprintf("%.10g", item.ELongitude))
			recordItem = append(recordItem, fmt.Sprintf("%f", item.Velocity))
			recordItem = append(recordItem, fmt.Sprintf("%d", item.GPSStatus))
			recordItem = append(recordItem, fmt.Sprintf("%f", item.Height))
			recordItem = append(recordItem, fmt.Sprintf("%f", item.AngleReal))
			recordItem = append(recordItem, fmt.Sprintf("%f", item.AngleMag))
			allDataFile.WriteString(strings.Join(recordItem, ",") + "\n")
		}
	}
}

func writePoint(pointDataFile *os.File) {
	var item Capture
	for {
		select {
		case s2 := <-pointContext:
			{
				if len(s2) <= 3 {
					fmt.Printf("%+v\n", item)
				} else {
					var recordItem []string
					recordItem = append(recordItem, fmt.Sprintf("%d", item.UnixTimestamp))
					recordItem = append(recordItem, item.UtcTime)
					recordItem = append(recordItem, fmt.Sprintf("%.10g", item.NLatitude))
					recordItem = append(recordItem, fmt.Sprintf("%.10g", item.ELongitude))
					recordItem = append(recordItem, fmt.Sprintf("%f", item.Velocity))
					recordItem = append(recordItem, fmt.Sprintf("%d", item.GPSStatus))
					recordItem = append(recordItem, fmt.Sprintf("%f", item.Height))
					recordItem = append(recordItem, fmt.Sprintf("%f", item.AngleReal))
					recordItem = append(recordItem, fmt.Sprintf("%f", item.AngleMag))
					recordItem = append(recordItem, s2)
					pointDataFile.WriteString(strings.Join(recordItem, ",") + "\n")
				}
			}
		case s1 := <-pointData:
			{
				itemTemp, ok := addGPSDataToVec(s1)
				if ok {
					item = itemTemp
				}
			}
		}
	}
}
