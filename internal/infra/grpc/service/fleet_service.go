package service

import (
	"context"
	"fmt"

	"github.com/DioGolang/GoFleet/internal/application/port/outbound"
	"github.com/DioGolang/GoFleet/internal/infra/grpc/pb"
	"github.com/DioGolang/GoFleet/pkg/logger"
)

type FleetService struct {
	pb.UnimplementedFleetServiceServer
	Repo   outbound.LocationRepository
	Logger logger.Logger
}

func NewFleetService(repo outbound.LocationRepository, log logger.Logger) *FleetService {
	return &FleetService{
		Repo:   repo,
		Logger: log,
	}
}

func (s *FleetService) SearchDriver(ctx context.Context, req *pb.SearchDriverRequest) (*pb.SearchDriverResponse, error) {
	s.Logger.Debug(ctx, "Searching nearest driver", logger.String("order_id", req.OrderId))

	// Simulação: Coordenadas do pedido
	orderLat, orderLng := -23.5505, -46.6333

	drivers, err := s.Repo.GetNearestDrivers(ctx, orderLat, orderLng, 5.0)
	if err != nil {
		s.Logger.Error(ctx, "Failed to query location repository", logger.WithError(err))
		return nil, err
	}

	if len(drivers) == 0 {
		s.Logger.Warn(ctx, "No drivers found in area",
			logger.String("order_id", req.OrderId),
			logger.Float64("lat", orderLat),
			logger.Float64("lng", orderLng),
		)
		return nil, fmt.Errorf("no drivers found within 5km radius")
	}

	driver := drivers[0]

	s.Logger.Info(ctx, "Driver found for order",
		logger.String("order_id", req.OrderId),
		logger.String("driver_id", driver.DriverID),
	)

	return &pb.SearchDriverResponse{
		DriverId: driver.DriverID,
		Name:     driver.DriverID,
		Lat:      driver.Latitude,
		Lng:      driver.Longitude,
	}, nil
}

func (s *FleetService) UpdateDriverPosition(ctx context.Context, driverID string, lat, lng float64) error {
	s.Logger.Debug(ctx, "Updating driver position", logger.String("driver_id", driverID))

	err := s.Repo.UpdateLocation(ctx, driverID, lat, lng)
	if err != nil {
		s.Logger.Error(ctx, "Failed to update driver position",
			logger.String("driver_id", driverID),
			logger.WithError(err),
		)
		return err
	}
	return nil
}
