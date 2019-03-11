package natter

type client struct {
	config ClientConfig
}

type ClientConfig struct {
	ClientId   string
	ServerAddr string
}

func NewClient(config ClientConfig) *client {
	return &client{config}
}

func (client *client) Forward() {

}