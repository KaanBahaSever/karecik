// Package migrations, .sql dosyalarini derlenmis ikili dosyanin icine gomer.
// Boylece "go run ./cmd/api" komutu hangi klasorden calistirilirsa calistirilsin
// migration dosyalari her zaman bulunur.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
