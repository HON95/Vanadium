package db

import (
	"context"
	"fmt"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	influxdb2api "github.com/influxdata/influxdb-client-go/v2/api"

	"dev.hon.one/vanadium/common"
	"dev.hon.one/vanadium/util"
)

// InfluxDBBucket - InfluxDB bucket.
const InfluxDBBucket = "vanadium"

// InfluxDBQueryRecentTime - InfluxDB-formatted time to consider for fetching "recent" entries.
const InfluxDBQueryRecentTime = "-5m"

var client *influxdb2.Client = nil
var clientQueryAPI *influxdb2api.QueryAPI
var clientWriteAPI *influxdb2api.WriteAPI

// StartClient - Start DB client.
func StartClient(waitGroup *sync.WaitGroup, shutdown *util.ShutdownChannelDistributor) {
	// Setup shutdown signal and waitgroup
	shutdownChannel := make(chan bool, 1)
	if !shutdown.AddListener(shutdownChannel) {
		return
	}
	waitGroup.Add(1)

	newClient := influxdb2.NewClient(common.GlobalConfig.InfluxDBURL, common.GlobalConfig.InfluxDBToken)
	client = &newClient

	cleanup := func() {
		if clientQueryAPI != nil {
			clientQueryAPI = nil
		}
		if clientWriteAPI != nil {
			localWriteAPI := *clientWriteAPI
			clientWriteAPI = nil
			localWriteAPI.Flush()
		}
		localClient := *client
		client = nil
		localClient.Close()
		log.Info("DB client stopped")
		waitGroup.Done()
	}

	// Wait for DB connection (true) to come up or for shutdown signal (false)
	if !waitForDBUp(shutdownChannel) {
		cleanup()
		return
	}

	// Setup query API, async write API and error logging
	queryAPI := (*client).QueryAPI(common.GlobalConfig.InfluxDBOrg)
	clientQueryAPI = &queryAPI
	writeAPI := (*client).WriteAPI(common.GlobalConfig.InfluxDBOrg, InfluxDBBucket)
	clientWriteAPI = &writeAPI
	writAPIErrors := writeAPI.Errors()
	go func() {
		for err := range writAPIErrors {
			log.WithError(err).Error("Failed to write to database")
		}
	}()

	go func() {
		<-shutdownChannel
		cleanup()
	}()

	log.Info("DB client started: ", common.GlobalConfig.InfluxDBURL)
}

func waitForDBUp(shutdownChannel <-chan bool) bool {
	checkHealth := func() bool {
		_, err := (*client).Health(context.Background())
		if err != nil {
			log.WithError(err).Tracef("Database connection error")
			return false
		}
		return true
	}
	if checkHealth() {
		return true
	}
	log.Info("Waiting for database")
	for {
		select {
		case <-time.Tick(1 * time.Second):
			if checkHealth() {
				return true
			}
		case <-shutdownChannel:
			return false
		}

	}
}

// StoreScrapeEntry - Attempt to store a scrape entry in the DB.
func StoreScrapeEntry(entry common.ScrapeEntry) {
	log.WithFields(log.Fields{
		"source":   entry.Source,
		"time":     entry.Time,
		"duration": entry.Duration,
		"success":  entry.Success,
	}).Trace("Scrape entry")

	if clientWriteAPI == nil {
		return
	}

	point := influxdb2.NewPointWithMeasurement("scrape").
		AddTag("source", entry.Source).
		AddField("duration_seconds", float64(entry.Duration)/float64(time.Second)).
		AddField("success", entry.Success).
		SetTime(entry.Time)
	(*clientWriteAPI).WritePoint(point)
}

// StoreSourceDeviceEntry - Attempt to store a source device entry in the DB.
func StoreSourceDeviceEntry(entry common.SourceDeviceEntry) {
	log.WithFields(log.Fields{
		"source":   entry.Source,
		"vendor":   entry.Vendor,
		"software": entry.Software,
		"other":    entry.Other,
	}).Trace("Source device entry")

	if clientWriteAPI == nil {
		return
	}

	point := influxdb2.NewPointWithMeasurement("source_device").
		AddTag("source", entry.Source).
		AddField("vendor", entry.Vendor).
		AddField("software", entry.Software).
		AddField("other", entry.Other).
		SetTime(entry.Time)
	(*clientWriteAPI).WritePoint(point)
}

// StoreL2DeviceEntry - Attempt to store an L2 device entry in the DB.
func StoreL2DeviceEntry(entry common.L2DeviceEntry) {
	log.WithFields(log.Fields{
		"source":       entry.Source,
		"mac_address":  entry.MACAddress,
		"vlan_id":      entry.VLANID,
		"vlan_name":    entry.VLANName,
		"l2_interface": entry.L2Interface,
		"is_self":      entry.IsSelf,
	}).Trace("L2 device entry")

	if clientWriteAPI == nil {
		return
	}

	point := influxdb2.NewPointWithMeasurement("l2_device").
		AddTag("source", entry.Source).
		AddField("mac_address", entry.MACAddress).
		AddField("vlan_id", entry.VLANID).
		AddField("vlan_name", entry.VLANName).
		AddField("l2_interface", entry.L2Interface).
		AddField("is_self", entry.IsSelf).
		SetTime(entry.Time)
	(*clientWriteAPI).WritePoint(point)
}

// StoreL3DeviceEntry - Attempt to store an L3 device entry in the DB.
func StoreL3DeviceEntry(entry common.L3DeviceEntry) {
	log.WithFields(log.Fields{
		"source":       entry.Source,
		"ip_address":   entry.IPAddress,
		"mac_address":  entry.MACAddress,
		"l3_interface": entry.L3Interface,
		"ip_network":   entry.IPNetwork,
	}).Trace("L3 device entry")

	if clientWriteAPI == nil {
		return
	}

	point := influxdb2.NewPointWithMeasurement("l3_device").
		AddTag("source", entry.Source).
		AddField("ip_address", entry.IPAddress).
		AddField("mac_address", entry.MACAddress).
		AddField("l3_interface", entry.L3Interface).
		AddField("ip_network", entry.IPNetwork).
		SetTime(entry.Time)
	(*clientWriteAPI).WritePoint(point)
}

// FetchRecentScrapeEntries - Fetch recent scrape entries from the DB.
func FetchRecentScrapeEntries() []common.ScrapeEntry {
	if clientQueryAPI == nil {
		return nil
	}

	// TODO do processing here instead?

	// TODO demo

	measurement := "scrape"
	result, err := (*clientQueryAPI).Query(context.Background(), `from(bucket:"`+InfluxDBBucket+`")|> range(start: `+InfluxDBQueryRecentTime+`) |> filter(fn: (r) => r._measurement == "`+measurement+`")`)
	if err != nil {
		log.WithError(err).Error("Failed to query from database")
		return nil
	}

	for result.Next() {
		record := result.Record()
		fmt.Printf("measurement=%v time=%v field=%v value=%v\n", record.Measurement(), record.Time(), record.Field(), record.Value())
	}
	// check for an error
	if result.Err() != nil {
		fmt.Printf("query parsing error: %s\n", result.Err().Error())
	}

	return nil
}
