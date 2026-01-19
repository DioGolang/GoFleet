package service

import (
	"context"
	"fmt"
	"github.com/DioGolang/GoFleet/internal/infra/grpc/pb"
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

	// Estado em memória (Simulando um Redis)
	drivers map[string]*Driver

	// O Guardião da Concorrência
	mu sync.RWMutex
}

func NewFleetService() *FleetService {
	// popular com alguns motoristas fake
	s := &FleetService{
		drivers: make(map[string]*Driver),
	}

	s.drivers["driver-1"] = &Driver{ID: "driver-1", Name: "João da Silva", Lat: -23.55, Lng: -46.63}
	s.drivers["driver-2"] = &Driver{ID: "driver-2", Name: "Maria Oliveira", Lat: -23.56, Lng: -46.64}

	return s
}

func (s *FleetService) SearchDriver(ctx context.Context, req *pb.SearchDriverRequest) (*pb.SearchDriverResponse, error) {
	fmt.Printf("gRPC: Seeking driver for order %s\n", req.OrderId)

	s.mu.RLock()
	defer s.mu.RUnlock()

	// Lógica simplificada: Pega o primeiro que achar
	// (Num cenário real, faria cálculo de distância Haversine)
	for _, d := range s.drivers {
		return &pb.SearchDriverResponse{
			DriverId: d.ID,
			Name:     d.Name,
			Lat:      d.Lat,
			Lng:      d.Lng,
		}, nil
	}

	return nil, fmt.Errorf("no driver available")
}

func (s *FleetService) UpdateDriverPosition(driverID string, lat, lng float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if driver, exists := s.drivers[driverID]; exists {
		driver.Lat = lat
		driver.Lng = lng
	}
}
