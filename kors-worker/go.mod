module github.com/haksolot/kors/kors-worker

go 1.25.8

replace github.com/haksolot/kors/shared => ../shared

require (
	github.com/google/uuid v1.6.0
	github.com/haksolot/kors/shared v0.0.0-00010101000000-000000000000
	github.com/jackc/pgx/v5 v5.8.0
	github.com/joho/godotenv v1.5.1
)

require (
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.19 // indirect
	github.com/rs/zerolog v1.34.0 // indirect
	golang.org/x/sys v0.41.0 // indirect
)
