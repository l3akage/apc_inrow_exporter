package main

import (
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"github.com/soniah/gosnmp"
)

const prefix = "apc_inrow_"

var (
	upDesc            *prometheus.Desc
	airflowDesc       *prometheus.Desc
	rackInetTemp      *prometheus.Desc
	supplyAirTemp     *prometheus.Desc
	returnAirTemp     *prometheus.Desc
	fanSpeed          *prometheus.Desc
	enteringFluidTemp *prometheus.Desc
	leavingFluidTemp  *prometheus.Desc
)

func init() {
	l := []string{"target"}
	upDesc = prometheus.NewDesc(prefix+"up", "Scrape of target was successful", l, nil)
	l = append(l, "name", "location")
	airflowDesc = prometheus.NewDesc(prefix+"airflow", "Air flow in liters per second.", l, nil)
	rackInetTemp = prometheus.NewDesc(prefix+"rack_inlet_temp", "Rack inlet temp", l, nil)
	supplyAirTemp = prometheus.NewDesc(prefix+"supply_air_temp", "Supply air temp", l, nil)
	returnAirTemp = prometheus.NewDesc(prefix+"return_air_temp", "Return air temp", l, nil)
	fanSpeed = prometheus.NewDesc(prefix+"fan_speed", "Fan speed", l, nil)
	enteringFluidTemp = prometheus.NewDesc(prefix+"entering_fluid_temp", "Entering fluid temp", l, nil)
	leavingFluidTemp = prometheus.NewDesc(prefix+"leaving_fluid_temp", "Leaving fluid temp", l, nil)
}

type apcInrowCollector struct {
}

func (c apcInrowCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- upDesc
	ch <- airflowDesc
	ch <- rackInetTemp
	ch <- supplyAirTemp
	ch <- returnAirTemp
	ch <- fanSpeed
	ch <- enteringFluidTemp
	ch <- leavingFluidTemp
}

func (c apcInrowCollector) getLabels(snmp *gosnmp.GoSNMP) (string, string, error) {
	oids := []string{"1.3.6.1.4.1.318.1.1.13.3.2.2.1.2.0", "1.3.6.1.4.1.318.1.1.13.3.2.2.1.3.0"}
	result, err := snmp.Get(oids)
	if err != nil {
		log.Infof("Get() err: %v\n", err)
		return "", "", err
	}
	if len(result.Variables) != 2 {
		return "", "", nil
	}
	return string(result.Variables[0].Value.([]byte)), string(result.Variables[1].Value.([]byte)), nil
}

func (c apcInrowCollector) collectTarget(target string, ch chan<- prometheus.Metric, wg *sync.WaitGroup) {
	defer wg.Done()
	snmp := &gosnmp.GoSNMP{
		Target:    target,
		Port:      161,
		Community: *snmpCommunity,
		Version:   gosnmp.Version2c,
		Timeout:   time.Duration(2) * time.Second,
	}
	err := snmp.Connect()
	if err != nil {
		log.Infof("Connect() err: %v\n", err)
		ch <- prometheus.MustNewConstMetric(upDesc, prometheus.GaugeValue, 0, target)
		return
	}
	defer snmp.Conn.Close()

	name, location, err2 := c.getLabels(snmp)
	if err2 != nil {
		log.Infof("getLabels() err: %v\n", err)
		ch <- prometheus.MustNewConstMetric(upDesc, prometheus.GaugeValue, 0, target)
		return
	}
	l := []string{target, name, location}

	oids := []string{"1.3.6.1.4.1.318.1.1.13.3.2.2.2.5.0", "1.3.6.1.4.1.318.1.1.13.3.2.2.2.7.0", "1.3.6.1.4.1.318.1.1.13.3.2.2.2.9.0",
		"1.3.6.1.4.1.318.1.1.13.3.2.2.2.11.0", "1.3.6.1.4.1.318.1.1.13.3.2.2.2.16.0", "1.3.6.1.4.1.318.1.1.13.3.2.2.2.24.0",
		"1.3.6.1.4.1.318.1.1.13.3.2.2.2.26.0"}
	result, err3 := snmp.Get(oids)
	if err2 != nil {
		log.Infof("Get() err: %v\n", err3)
		ch <- prometheus.MustNewConstMetric(upDesc, prometheus.GaugeValue, 0, target)
		return
	}
	for _, variable := range result.Variables {
		if variable.Value == nil {
			continue
		}
		switch variable.Name[1:] {
		case oids[0]:
			ch <- prometheus.MustNewConstMetric(airflowDesc, prometheus.GaugeValue, float64(variable.Value.(int))/100, l...)
		case oids[1]:
			ch <- prometheus.MustNewConstMetric(rackInetTemp, prometheus.GaugeValue, float64(variable.Value.(int))/10, l...)
		case oids[2]:
			ch <- prometheus.MustNewConstMetric(supplyAirTemp, prometheus.GaugeValue, float64(variable.Value.(int))/10, l...)
		case oids[3]:
			ch <- prometheus.MustNewConstMetric(returnAirTemp, prometheus.GaugeValue, float64(variable.Value.(int))/10, l...)
		case oids[4]:
			ch <- prometheus.MustNewConstMetric(fanSpeed, prometheus.GaugeValue, float64(variable.Value.(int))/10, l...)
		case oids[5]:
			ch <- prometheus.MustNewConstMetric(enteringFluidTemp, prometheus.GaugeValue, float64(variable.Value.(int))/10, l...)
		case oids[6]:
			ch <- prometheus.MustNewConstMetric(leavingFluidTemp, prometheus.GaugeValue, float64(variable.Value.(int))/10, l...)
		}
	}

	ch <- prometheus.MustNewConstMetric(upDesc, prometheus.GaugeValue, 1, target)
}

func (c apcInrowCollector) Collect(ch chan<- prometheus.Metric) {
	targets := strings.Split(*snmpTargets, ",")
	wg := &sync.WaitGroup{}

	for _, target := range targets {
		wg.Add(1)
		go c.collectTarget(target, ch, wg)
	}

	wg.Wait()
}
