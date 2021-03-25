package main

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Digger struct {
	c            *Client
	l            *Licenser
	t            *Treasurer
	pointsToFind chan Point
	measure      *Measure
}

func (d *Digger) dig(point Point, depth int, license int) ([]Treasure, error) {
	request := struct {
		X         int `json:"posX"`
		Y         int `json:"posY"`
		Depth     int `json:"depth"`
		LicenseID int `json:"licenseID"`
	}{
		X: point.x, Y: point.y, Depth: depth, LicenseID: license,
	}

	response := make([]Treasure, 0)

	code, err := d.c.doRequest("dig", &request, &response, false)
	if err != nil && code != 404 {
		if strings.Contains(err.Error(), "no such license") {
			fmt.Println("no license, status code "+strconv.Itoa(code), license)
			return response, nil
		}
		return nil, err
	}

	return response, nil
}

func (d *Digger) run() {
	fmt.Println("Tochek:", len(d.pointsToFind))

	var wg sync.WaitGroup

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for point := range d.pointsToFind {
				d.measure.Add("points_queue", 1)

				for depth := 1; depth <= MAX_DEPTH; depth++ {
					license := d.l.GetLicense(d)
					treasures, err := d.dig(point, depth, license)
					if err != nil {
						d.l.ReturnLicense(license)
						return
					}
					d.measure.Add("depth_"+strconv.Itoa(depth)+"_sum", int64(len(treasures)))
					for _, treasure := range treasures {
						d.t.Cash(treasure)
					}
					point.amount -= len(treasures)
					if point.amount <= 0 {
						break
					}
				}

				d.measure.Add("points_queue", -1)
			}
		}()
	}

	time.Sleep(3 * time.Minute)
	d.Done()
	wg.Wait()
}

func (d *Digger) Find(point Point) {
	d.pointsToFind <- point
}

func (d *Digger) Done() {
	close(d.pointsToFind)
}

func NewDigger(client *Client, licenser *Licenser, treasurer *Treasurer) *Digger {
	//client.SetRPSLimit("dig", 499)
	measure := []string{"points_queue", "wait_license_count", "wait_license_time"}
	for i := 1; i <= 10; i++ {
		measure = append(measure, "depth_"+strconv.Itoa(i)+"_sum")
	}
	digger := Digger{
		c:            client,
		l:            licenser,
		t:            treasurer,
		pointsToFind: make(chan Point, 1000000),
		measure:      NewMeasure(measure),
	}
	return &digger
}
