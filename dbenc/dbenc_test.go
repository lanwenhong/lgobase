package dbenc

import (
	"context"
	"testing"

	"github.com/lanwenhong/lgobase/dbenc"
)

func TestLoadFile(t *testing.T) {
	file := "./db.conf"
	ctx := context.Background()
	dbenc.DbConfNew(ctx, file)
}
