package util

import (
	"context"
	"testing"
	
	"github.com/lanwenhong/lgobase/logger"
)

func TestGenID(t *testing.T) {
	ctx := context.Background()
	
	logger.Info(ctx, NewRequestID())
}
