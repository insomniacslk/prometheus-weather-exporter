# prometheus-weather-exporter

This is a weather exporter for Prometheus. It uses the Google Maps API to get
the coordinates of a location given by name, and the Darksky API to get the
weather forecast.

Note that Darksky is not accepting any more API sign-ups, so this will work only
if you have an existing Darksky account.

## Metrics

The exported metrics are named like `weather_<metric>`, where `<metric>` is what
you define in the configuration file as explained below.

## Configuration file

Create a configuration file similar to the following:

```
{
    "metrics": ["temperature", "apparent_temperature", "wind_speed", "humidity", "cloud_cover"],
    "locations": ["Dublin", "Napoli", "Bagsværd", "Pianoro", "Zurich", "Trieste", "San Francisco", "Seattle", "New York", "Torino"],
    "google_maps_api_key": "your-api-key",
    "darksky_api_key": "your-api-key"
}
```

More specifically:
* `metrics`: the metrics that will be exported to Prometheus. See [`getValueByFieldName`](https://github.com/insomniacslk/prometheus-weather-exporter/blob/main/main.go#L104) for supported metrics. Feel free to add more metrics from [`forecast.DataPoint`](https://github.com/mlbright/darksky/blob/master/v2/forecast.go#L28).
* `locations`: the locations you want metrics exported for. Anything that the
  Google Maps Geocoding API will understand.
* `google_maps_api_key`: self-explaining
* `darksky_api_key`: self-explaining

## Run it

```
go build
./prometheus-weather-exporter -c /path/to/your-config.json
```

## Grafana

See dashboard at
[dashboard.json](https://github.com/insomniacslk/prometheus-weather-exporter/blob/main/dashboard.json)
