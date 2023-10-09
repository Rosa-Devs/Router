



bin:
	go build -o ./build/main ./src/main/main.go
	@chmod +x ./build/main
	cd ./build && ./main


models:
	protoc --go_out=. ./proto/*