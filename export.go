package natter

import "C"
import "unsafe"

//export natter_client_listen
func natter_client_listen(clientUser *C.char, brokerAddr *C.char) C.int {
	client, _ := NewClient(&Config{
		ClientId:   C.GoString(clientUser),
		BrokerAddr: C.GoString(brokerAddr),
	})

	if err := client.Listen(); err != nil {
		return 1
	}

	return 0
}

//export natter_client_forward
func natter_client_forward(clientUser *C.char, brokerAddr *C.char,
	localAddr *C.char, target *C.char, targetForwardAddr *C.char, targetCommandCount C.int, targetCommand **C.char) C.int {

	client, _ := NewClient(&Config{
		ClientId:   C.GoString(clientUser),
		BrokerAddr: C.GoString(brokerAddr),
	})

	if _, err := client.ForwardTCP(C.GoString(localAddr), C.GoString(target), C.GoString(targetForwardAddr),
		toGoStrings(targetCommandCount, targetCommand)); err != nil {
		return 1
	}

	return 0
}

//export natter_broker_listen
func natter_broker_listen(listenAddr *C.char) C.int {
	broker, err := NewBroker(&Config{BrokerAddr: C.GoString(listenAddr)})
	if err != nil {
		return 1
	}

	if err := broker.ListenAndServe(); err != nil {
		return 2
	}

	return 0 // Unreachable
}

// https://stackoverflow.com/questions/36188649/cgo-char-to-slice-string?rq=1
func toGoStrings(argc C.int, argv **C.char) []string {
	length := int(argc)

	if length > 0 {
		tmpslice := (*[1 << 30]*C.char)(unsafe.Pointer(argv))[:length:length]
		gostrings := make([]string, length)

		for i, s := range tmpslice {
			gostrings[i] = C.GoString(s)
		}

		return gostrings
	} else {
		return make([]string, 0)
	}
}

