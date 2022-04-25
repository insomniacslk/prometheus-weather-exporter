package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	forecast "github.com/mlbright/forecast/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"googlemaps.github.io/maps"
)

var (
	flagPath          = flag.String("p", "/metrics", "HTTP path where to expose metrics to")
	flagListen        = flag.String("l", ":9102", "Address to listen to")
	flagSleepInterval = flag.Duration("i", time.Hour, "Interval between weather checks")
	flagConfigFile    = flag.String("c", "config.json", "Configuration file")
)

// Config is the configuration file type.
type Config struct {
	Locations        []string `json:"locations"`
	Metrics          []string `json:"metrics"`
	GoogleMapsAPIKey string   `json:"google_maps_api_key"`
	DarkskyAPIKey    string   `json:"darksky_api_key"`
}

// LoadConfig loads the configuration file into a Config type.
func LoadConfig(filepath string) (*Config, error) {
	data, err := ioutil.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON config: %w", err)
	}
	return &config, nil
}

// Location is used to identify a location by name, latitude, and longitude.
type Location struct {
	Name     string
	Lat, Lng float64
}

// LatString returns a latitude string
func (l *Location) LatString() string {
	return fmt.Sprintf("%f", l.Lat)
}

// LngString returns a longitude string
func (l *Location) LngString() string {
	return fmt.Sprintf("%f", l.Lng)
}

func getLocation(apikey, locName string) (*Location, error) {
	client, err := maps.NewClient(maps.WithAPIKey(apikey))
	if err != nil {
		return nil, err
	}
	r := maps.GeocodingRequest{
		Address: locName,
	}
	resp, err := client.Geocode(context.Background(), &r)
	if err != nil {
		return nil, err
	}
	if len(resp) == 0 {
		return nil, fmt.Errorf("no location found for '%s'", locName)
	}
	loc := Location{
		Name: resp[0].AddressComponents[0].LongName,
		Lat:  resp[0].Geometry.Location.Lat,
		Lng:  resp[0].Geometry.Location.Lng,
	}
	return &loc, nil
}

func getWeather(mapsAPIKey, darkskyAPIKey, locName string) (*forecast.Forecast, error) {
	// TODO cache location
	loc, err := getLocation(mapsAPIKey, locName)
	if err != nil {
		return nil, fmt.Errorf("GMaps search failed: %w", err)
	}
	fc, err := forecast.Get(darkskyAPIKey, loc.LatString(), loc.LngString(), "now", forecast.SI, forecast.English)
	if err != nil {
		return nil, fmt.Errorf("forecast request failed: %w", err)
	}
	if fc.Flags.Units != string(forecast.SI) {
		return nil, fmt.Errorf("units are not SI: got %v", fc.Flags.Units)
	}
	return fc, nil
}

// getValueByFieldName returns a float64 value based on the supported
// fields in the forecast datapoint.
func getValueByFieldName(field string, dp *forecast.DataPoint) (float64, error) {
	switch field {
	case "temperature":
		return dp.Temperature, nil
	case "apparent_temperature":
		return dp.ApparentTemperature, nil
	case "wind_speed":
		return dp.WindSpeed, nil
	case "cloud_cover":
		return dp.CloudCover, nil
	case "humidity":
		return dp.Humidity, nil
	case "precip_intensity":
		return dp.PrecipIntensity, nil
	default:
		return 0, fmt.Errorf("unsupported field '%s'", field)
	}
}

func main() {
	flag.Parse()
	config, err := LoadConfig(*flagConfigFile)
	if err != nil {
		log.Fatalf("Failed to load configuration file '%s': %v", *flagConfigFile, err)
	}
	fmt.Printf("Locations (%d): %s\n", len(config.Locations), config.Locations)
	fmt.Printf("Metrics (%d): %s", len(config.Metrics), config.Metrics)

	if len(config.Locations) == 0 {
		log.Fatalf("Must specify at least one location")
	}
	if len(config.Metrics) == 0 {
		log.Fatalf("Must specify at least one metric")
	}

	gauges := map[string]*prometheus.GaugeVec{}
	for _, key := range config.Metrics {
		gauges[key] = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: fmt.Sprintf("weather_%s", key),
				Help: fmt.Sprintf("Weather forecast - %s", strings.Replace(key, "_", " ", -1)),
			},
			[]string{"location", "latitude", "longitude"},
		)
		if err := prometheus.Register(gauges[key]); err != nil {
			log.Fatalf("Failed to register weather %s gauge: %v", key, err)
		}
	}

	go func() {
		for {
			log.Printf("Fetching weather...")
			for _, loc := range config.Locations {
				fmt.Printf("Getting weather for %s\n", loc)
				fc, err := getWeather(config.GoogleMapsAPIKey, config.DarkskyAPIKey, loc)
				if err != nil {
					log.Printf("Failed to get weather for '%s': %v", loc, err)
				} else {
					// update values
					for key, gauge := range gauges {
						val, err := getValueByFieldName(key, &fc.Currently)
						if err != nil {
							log.Printf("Warning: skipping '%s': %v", key, err)
							continue
						}
						gauge.WithLabelValues(loc, fmt.Sprintf("%f", fc.Latitude), fmt.Sprintf("%f", fc.Longitude)).Set(val)
					}
				}
			}
			log.Printf("Sleeping %s...", *flagSleepInterval)
			time.Sleep(*flagSleepInterval)
		}
	}()

	http.Handle(*flagPath, promhttp.Handler())
	log.Printf("Starting server on %s", *flagListen)
	log.Fatal(http.ListenAndServe(*flagListen, nil))
}
