package order

// Input

type CreateInput struct {
	ID    string  `json:"id"`
	Price float64 `json:"price"`
	Tax   float64 `json:"tax"`
}

type DispatchInput struct {
	OrderID  string
	DriverID string
}

// Output

type CreateOutput struct {
	ID         string  `json:"id"`
	FinalPrice float64 `json:"final_price"`
}
