package natter

func NewDaemon() *daemon {
	return &daemon{}
}

func NewForwarder() *forwarder {
	return &forwarder{}
}

func NewServer() *server {
	return &server{}
}