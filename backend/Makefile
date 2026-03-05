# Load file .env
ifneq (,$(wildcard ./.env))
    include .env
    export
endif

# Run Up command
migrate-up:
	migrate -path migrations -database $(DATABASE_URL) up

# Run Down 1 step command
migrate-down:
	migrate -path migrations -database $(DATABASE_URL) down 1

# Command to create new migration file (Ex: make migrate-create name=add_users_table)
migrate-create:
	migrate create -ext sql -dir migrations -seq $(name)