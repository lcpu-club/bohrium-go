package lbg

type Client struct {
}

type ClientConfigure struct {
	Username string
	Password string

	// Endpoint defaults to https://bohrium.dp.tech
	Endpoint string
}

func NewClient(conf *ClientConfigure) *Client {
	// Default values
	if conf.Endpoint == "" {
		conf.Endpoint = "https://bohrium.dp.tech"
	}
	return &Client{}
}

func (c *Client) Login() {

}
