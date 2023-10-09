



bin:
	go build -o ./build/main ./src/main/main.go
	@chmod +x ./build/main
	cd ./build && ./main


db:
	go build -o ./build/db_test ./cmd/db.go
	@chmod +x ./build/db_test
	cd ./build && ./db_test


models:
	protoc --go_out=. ./proto/*