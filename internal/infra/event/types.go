package event

import "context"

type MessageHandler func(ctx context.Context, msg []byte, headers map[string]interface{}) error
