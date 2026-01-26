package outbound

import "context"

type DriverLocation struct {
	DriverID  string
	Latitude  float64
	Longitude float64
}

type LocationRepository interface {
	GetNearestDrivers(ctx context.Context, lat, lng float64, radius float64) ([]DriverLocation, error)
	UpdateLocation(ctx context.Context, driverID string, lat, lng float64) error
}
