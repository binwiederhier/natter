all: clean proto cmd lib

clean:
	@echo == Cleaning ==
	rm -rf build
	@echo

proto:
	@echo == Generating protobuf code ==
	protoc --go_out=internal internal/*.proto
	@echo

cmd: proto
	@echo == Building natter CLI ==
	mkdir -p build/cmd
	go build -o build/cmd/natter cmd/natter/main.go
	@echo
	@echo "--> natter CLI built at build/cmd/natter"
	@echo

lib: proto
	@echo == Building natter library ==
	mkdir -p build/lib build/lib/_obj
	go build -o build/lib/libnatter.so -buildmode=c-shared cmd/natter/main.go
	go tool cgo -objdir build/lib/_obj -exportheader build/lib/natter.h export.go
	@echo
	@echo "--> natter library built at build/lib/libnatter.so"
	@echo

example_simple_go: proto
	@echo == Building Go example ==
	mkdir -p build/example/simple_go
	go build -o build/example/simple_go/main example/simple_go/main.go
	@echo
	@echo "--> Example built, run like this: build/example/simple_go/main"
	@echo

example_simple_go_run: example_simple_go
	build/example/simple_go/main

example_simple_c: lib
	@echo == Building C example ==
	mkdir -p build/example/simple_c
	gcc -o build/example/simple_c/main example/simple_c/main.c -L build/lib -I build/lib -l pthread -l natter
	@echo
	@echo "--> Example built, run like this: LD_LIBRARY_PATH=build/lib build/example/simple_c/main"
	@echo

example_simple_c_run: example_simple_c
	LD_LIBRARY_PATH=build/lib build/example/simple_c/main

example_simple_cpp: lib
	@echo == Building C++ example ==
	mkdir -p build/example/simple_cpp
	g++ -o build/example/simple_cpp/main example/simple_cpp/main.cpp -L build/lib -I build/lib -l pthread -l natter
	@echo
	@echo "--> Example built, run like this: LD_LIBRARY_PATH=build/lib build/example/simple_cpp/main"
	@echo

example_simple_cpp_run: example_simple_cpp
	LD_LIBRARY_PATH=build/lib build/example/simple_cpp/main
