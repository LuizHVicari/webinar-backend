package middleware_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/LuizHVicari/webinar-backend/pkg/testhelper"
)

var sharedKratos testhelper.KratosURLs

func TestMain(m *testing.M) {
	kc, err := testhelper.StartKratos()
	if err != nil {
		fmt.Printf("setup kratos: %v\n", err)
		os.Exit(1)
	}
	defer kc.Terminate()
	sharedKratos = kc.URLs

	os.Exit(m.Run())
}
