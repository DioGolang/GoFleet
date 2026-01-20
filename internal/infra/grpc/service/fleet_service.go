package service

import (
	"context"
	"fmt"
	"github.com/DioGolang/GoFleet/internal/infra/grpc/pb"
	"github.com/redis/go-redis/v9"
	"sync"
)

// Driver simulado
type Driver struct {
	ID   string
	Name string
	Lat  float64
	Lng  float64
}

type FleetService struct {
	pb.UnimplementedFleetServiceServer
	RedisClient *redis.Client
	mu          sync.RWMutex
}

func NewFleetService(rds *redis.Client) *FleetService {
	return &FleetService{
		RedisClient: rds,
	}
}

func (s *FleetService) SearchDriver(ctx context.Context, req *pb.SearchDriverRequest) (*pb.SearchDriverResponse, error) {
	// 1. Simulação: O pedido tem uma lat/lng de origem (no mundo real viria no request)
	orderLat, orderLng := -23.5505, -46.6333

	cmd := s.RedisClient.GeoSearchLocation(ctx, "drivers_locations",
		&redis.GeoSearchLocationQuery{
			GeoSearchQuery: redis.GeoSearchQuery{
				Latitude:  orderLat,
				Longitude: orderLng,
				Radius:    5,
				BoxUnit:   "km",
				Sort:      "ASC",
				Count:     1,
			},
			WithCoord: true,
		},
	)
	locations, err := cmd.Result()
	if err != nil {
		return nil, err
	}
	if len(locations) == 0 {
		return nil, fmt.Errorf("nno drivers found within 5km radius")
	}
	driver := locations[0]

	return &pb.SearchDriverResponse{
		DriverId: driver.Name,
		Name:     driver.Name,
		Lat:      driver.Latitude,
		Lng:      driver.Longitude,
	}, nil
}

func (s *FleetService) UpdateDriverPosition(ctx context.Context, driverID string, lat, lng float64) error {
	return s.RedisClient.GeoAdd(ctx, "drivers_locations", &redis.GeoLocation{
		Name:      driverID,
		Longitude: lng,
		Latitude:  lat,
	}).Err()
}
