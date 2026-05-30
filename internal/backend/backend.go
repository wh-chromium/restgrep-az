package backend

import (
	"context"
	"github.com/wh-chromium/restgrep-az/internal/models"
)

type Backend interface {
	Name() string
	Search(ctx context.Context, query string, opts models.SearchOptions) ([]models.IntermediateResult, error)
}
