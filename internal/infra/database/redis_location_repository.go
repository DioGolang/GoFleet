package database

import (
	"context"
	"fmt"
	"github.com/DioGolang/GoFleet/internal/application/port/outbound"
	"github.com/DioGolang/GoFleet/pkg/logger"
	"github.com/redis/go-redis/v9"
)

type RedisLocationRepository struct {
	client *redis.Client
	logger logger.Logger
}

func NewRedisLocationRepository(client *redis.Client, log logger.Logger) *RedisLocationRepository {
	return &RedisLocationRepository{client: client, logger: log}
}

func (r *RedisLocationRepository) GetNearestDrivers(ctx context.Context, lat, lng, radius float64) ([]outbound.DriverLocation, error) {
	r.logger.Debug(ctx, "Redis GeoSearch query",
		logger.Float64("lat", lat),
		logger.Float64("lng", lng),
		logger.Float64("radius", radius),
	)
	cmd := r.client.GeoSearchLocation(ctx, "drivers_locations",
		&redis.GeoSearchLocationQuery{
			GeoSearchQuery: redis.GeoSearchQuery{
				Latitude:  lat,
				Longitude: lng,
				Radius:    radius,
				BoxUnit:   "km",
				Sort:      "ASC",
				Count:     10,
			},
			WithCoord: true,
		},
	)

	results, err := cmd.Result()
	if err != nil {
		r.logger.Error(ctx, "Redis command failed", logger.WithError(err))
		return nil, fmt.Errorf("redis geo search error: %w", err)
	}

	locations := make([]outbound.DriverLocation, len(results))
	for i, res := range results {
		locations[i] = outbound.DriverLocation{
			DriverID:  res.Name,
			Latitude:  res.Latitude,
			Longitude: res.Longitude,
		}
	}

	return locations, nil
}

func (r *RedisLocationRepository) UpdateLocation(ctx context.Context, driverID string, lat, lng float64) error {
	r.logger.Debug(ctx, "Redis GeoAdd",
		logger.String("driver_id", driverID),
		logger.Float64("lat", lat),
		logger.Float64("lng", lng),
	)

	err := r.client.GeoAdd(ctx, "drivers_locations", &redis.GeoLocation{
		Name:      driverID,
		Longitude: lng,
		Latitude:  lat,
	}).Err()

	if err != nil {
		r.logger.Error(ctx, "Redis GeoAdd failed", logger.WithError(err))
		return err
	}
	return nil
}
