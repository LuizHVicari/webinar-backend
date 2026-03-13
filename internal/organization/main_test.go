package organization_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/LuizHVicari/webinar-backend/pkg/keto"
	"github.com/LuizHVicari/webinar-backend/pkg/testhelper"
)

var (
	sharedPool *pgxpool.Pool
	sharedKeto *keto.Client
)

func TestMain(m *testing.M) {
	pg, err := testhelper.StartPostgres()
	if err != nil {
		fmt.Printf("setup postgres: %v\n", err)
		os.Exit(1)
	}
	defer pg.Terminate()
	sharedPool = pg.Pool

	kc, err := testhelper.StartKeto()
	if err != nil {
		fmt.Printf("setup keto: %v\n", err)
		os.Exit(1)
	}
	defer kc.Terminate()
	sharedKeto = kc.Client

	os.Exit(m.Run())
}
