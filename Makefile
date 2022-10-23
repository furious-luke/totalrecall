all: totalrecall worker

totalrecall:
	go build -ldflags="-s -w"

worker:
	$(MAKE) -C worker

.PHONY: all worker
