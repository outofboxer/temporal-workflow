package feesapi

import (
	"context"

	"encore.dev/config"
	"encore.dev/rlog"

	"github.com/outofboxer/temporal-workflow/fees/app"
	"github.com/outofboxer/temporal-workflow/fees/app/usecases"
	"github.com/outofboxer/temporal-workflow/fees/internal/adapters/temporal"
	feesServiceConfig "github.com/outofboxer/temporal-workflow/fees/services/feesapi/config"
)

//nolint:unused
var cfg *feesServiceConfig.Config = config.Load[*feesServiceConfig.Config]()

// This is the DOMAIN SERVICE for Fees.
// encore:service
type Service struct {
	temporalClient app.TemporalClient
	// Use cases
	Create  usecases.CreateBill
	AddItem usecases.AddLineItem
	Close   usecases.CloseBill
	Get     usecases.GetBill
	Search  usecases.SearchBill
}

// All Dependency Injection (DI) should come here! And hierarchical wiring, too.
//
//nolint:unused
func initService() (*Service, error) {
	rlog.Debug("config", "temporal.host", cfg.Temporal.Host())

	tc, err := temporal.NewClient(cfg.Temporal.Host(), cfg.Temporal.Namespace())
	if err != nil {
		return nil, err
	}

	tgw := temporal.NewGateway(tc, cfg.Temporal.Namespace())

	s := &Service{
		temporalClient: tc,
		Create:         usecases.CreateBill{T: tgw},
		AddItem:        usecases.AddLineItem{T: tgw},
		Close:          usecases.CloseBill{T: tgw},
		Get:            usecases.GetBill{T: tgw},
		Search:         usecases.SearchBill{T: tgw},
	}

	// This project is a template for me, we don't use database in this project, but I leave it here.
	// Also gets the single, shared database connection pool.
	// dbConnection := database.DB()

	// Creates its own repository, but uses the same connection.
	// repo := NewUserRepository(dbConnection)

	return s, nil
}

func (s *Service) Shutdown(_ context.Context) {
	rlog.Debug("FeesApi service Shutdown!")
	s.temporalClient.Close()
}
