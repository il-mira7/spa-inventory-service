package main

import (
	"github.com/il-mira7/spa-inventory-service/internal/app"
	"go.uber.org/fx"
)

func main() {
	fx.New(
		app.BaseModule,
	).Run()
}
