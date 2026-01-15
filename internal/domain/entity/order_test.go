package entity

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNewOrder(t *testing.T) {
	//Arrange
	id := "123"
	price := 10.0
	tax := 2.0

	//Act
	order, err := NewOrder(id, price, tax)

	//Assert
	assert.Nil(t, err)
	assert.NotNil(t, order)
	assert.Equal(t, 12.0, order.FinalPrice())
}

func TestNewOrder_ValidationErrors(t *testing.T) {
	tests := []struct {
		name        string
		id          string
		price       float64
		tax         float64
		expectedErr error
	}{
		{"Should return error when ID is empty", "", 10.0, 2.0, ErrIDIsRequired},
		{"Should return error when Price is 0", "123", 0.0, 2.0, ErrPriceIsRequired},
		{"Should return error when Tax is negative", "123", 10.0, -1.0, ErrTaxMustBePos},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			order, err := NewOrder(tt.id, tt.price, tt.tax)

			assert.Error(t, err)
			assert.ErrorIs(t, err, tt.expectedErr)
			assert.Nil(t, order)
		})
	}
}
