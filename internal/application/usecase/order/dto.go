package order

// Input

type CreateOrderInput struct {
	ID    string  `json:"id"`
	Price float64 `json:"price"`
	Tax   float64 `json:"tax"`
}

// Output

type CreateOrderOutput struct {
	ID         string  `json:"id"`
	FinalPrice float64 `json:"final-price"`
}
