all: clean
	mkdir -p build
	cd cmd && go build -o ../build/natter

clean:
	rm -rf build


